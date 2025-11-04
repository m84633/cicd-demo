package logic

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"math/big"
	"partivo_tickets/internal/constants"
	"partivo_tickets/internal/dao/repository"
	"strconv"

	"partivo_tickets/internal/dto"
	"partivo_tickets/internal/helper"
	"partivo_tickets/internal/models"
	"partivo_tickets/pkg/pagination"
	"partivo_tickets/pkg/relation"
	"partivo_tickets/pkg/snowflake"
	"time"

	"github.com/google/wire"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// OrderLogic defines the interface for order-related business logic.
type OrderLogic interface {
	AddOrder(ctx context.Context, d *dto.AddOrderRequest) (*dto.AddOrderResponse, error)
	GetOrder(ctx context.Context, id primitive.ObjectID) (*models.Order, error)
	GetOrdersByUser(ctx context.Context, uid primitive.ObjectID, token pagination.PageToken) ([]*dto.OrderWithItems, pagination.PageToken, error)
	CancelOrder(ctx context.Context, d *dto.CancelOrderRequest) error
	RequestOrderItemRefund(ctx context.Context, d *dto.RequestOrderItemRefundRequest) error
	IsEventHasOpenOrders(ctx context.Context, eid primitive.ObjectID) (bool, error)
	CreateRefundBill(ctx context.Context, d *dto.CreateRefundBillRequest) (primitive.ObjectID, error)
	GetOrdersByEvent(ctx context.Context, eid primitive.ObjectID, pageReq *pagination.PageRequest) (*pagination.PageResult, error)
	GetOrdersBySession(ctx context.Context, sid primitive.ObjectID, pageReq *pagination.PageRequest) (*pagination.PageResult, error)
	UpdateOrderItemStatus(ctx context.Context, d *dto.UpdateOrderItemStatusRequest) error
	GetOrderDetails(ctx context.Context, orderID primitive.ObjectID) (*dto.OrderDetails, error)
	RejectOrderItemRefund(ctx context.Context, d *dto.RejectOrderItemRefundRequest) error
	ExportEventOrdersByMonth(ctx context.Context, eventID primitive.ObjectID, year int, month int) (string, []byte, error)
	FindRefundableItemForBill(ctx context.Context, bill *models.Bill) (*models.OrderItem, error)
	HandlePaymentSuccess(ctx context.Context, orderID primitive.ObjectID, ticketEventPublisher *TicketEventPublisher) error
	MarkOrderItemsAsFulfilled(ctx context.Context, orderID primitive.ObjectID) error
}

var _ OrderLogic = (*orderLogic)(nil)

type GenerateTicketsTopic string
type PaymentEventTopic string

type orderLogic struct {
	orderRepo             repository.OrdersRepository
	billRepo              repository.BillRepository
	ticketRepo            repository.TicketsRepository
	auditLogRepo          repository.AuditLogRepository
	paymentEventPublisher *PaymentEventPublisher
	// ticketEventPublisher is removed from the struct fields
	relationClient *relation.Client
	idGenerator    *snowflake.Generator
	logger         *zap.Logger
}

func NewOrderLogic(orderRepo repository.OrdersRepository, billRepo repository.BillRepository, ticketRepo repository.TicketsRepository, auditLogRepo repository.AuditLogRepository, paymentEventPublisher *PaymentEventPublisher, relationClient *relation.Client, idGenerator *snowflake.Generator, logger *zap.Logger) *orderLogic {
	return &orderLogic{
		orderRepo:             orderRepo,
		billRepo:              billRepo,
		ticketRepo:            ticketRepo,
		auditLogRepo:          auditLogRepo,
		paymentEventPublisher: paymentEventPublisher,
		relationClient:        relationClient,
		idGenerator:           idGenerator,
		logger:                logger.Named("OrderLogic"),
	}
}

func (l *orderLogic) AddOrder(ctx context.Context, d *dto.AddOrderRequest) (*dto.AddOrderResponse, error) {
	now := time.Now()

	// validate and get order item
	builtItems, serverSubtotalDecimal, serverTotalDecimal, eventID, sessionID, err := l.validateAndBuildOrderItems(ctx, d.GetDetails(), now, d.GetUser())
	if err != nil {
		return nil, err
	}

	if eventID != d.GetEvent() {
		return nil, fmt.Errorf("event mismatch between request and ticket stock. request: %s, stock: %s", d.GetEvent().Hex(), eventID.Hex())
	}

	// check 跟使用者確認的資料有沒有誤
	clientSubtotalDecimal, err := primitive.ParseDecimal128(d.GetSubtotal())
	if err != nil {
		return nil, fmt.Errorf("invalid client subtotal format: %w", err)
	}
	clientTotalDecimal, err := primitive.ParseDecimal128(d.GetTotal())
	if err != nil {
		return nil, fmt.Errorf("invalid client total format: %w", err)
	}

	cmpSubtotal, err := helper.CompareDecimal128(clientSubtotalDecimal, serverSubtotalDecimal)
	if err != nil {
		return nil, fmt.Errorf("failed to compare subtotals: %w", err)
	}
	if cmpSubtotal != 0 {
		return nil, fmt.Errorf("subtotal mismatch. client: %s, server: %s", d.GetSubtotal(), serverSubtotalDecimal.String())
	}

	cmpTotal, err := helper.CompareDecimal128(clientTotalDecimal, serverTotalDecimal)
	if err != nil {
		return nil, fmt.Errorf("failed to compare totals: %w", err)
	}
	if cmpTotal != 0 {
		return nil, fmt.Errorf("total mismatch. client: %s, server: %s", d.GetTotal(), serverTotalDecimal.String())
	}

	// Generate serial number
	serialID, err := l.idGenerator.GetID()
	if err != nil {
		l.logger.Error("failed to generate snowflake id", zap.Error(err))
		return nil, fmt.Errorf("failed to generate serial number: %w", err)
	}

	// fill model
	orderModel := &models.Order{
		ID:                 primitive.NewObjectID(),
		Event:              eventID,
		Session:            sessionID,
		Serial:             serialID,
		Subtotal:           serverSubtotalDecimal,
		Total:              serverTotalDecimal,
		OriginalTotal:      serverTotalDecimal,
		User:               d.GetUser(),
		Status:             constants.OrderStatusUnpaid.String(),
		Note:               "",
		CreatedAt:          now,
		UpdatedAt:          now,
		UpdatedBy:          nil,
		Paid:               func() primitive.Decimal128 { var i primitive.Decimal128; return i }(),
		Returned:           func() primitive.Decimal128 { var i primitive.Decimal128; return i }(),
		ItemsCount:         0,
		ReturnedItemsCount: 0,
		MerchantID:         d.GetMerchantID(),
	}

	// --- 核心業務操作 ---
	// 1. Create Order
	if _, err := l.orderRepo.CreateOrder(ctx, orderModel); err != nil {
		return nil, fmt.Errorf("failed to create order: %w", err)
	}

	// 2. Create OrderItems
	if err := l.orderRepo.CreateOrderItems(ctx, orderModel.ID, builtItems); err != nil {
		return nil, fmt.Errorf("failed to create order items: %w", err)
	}

	// 3. Reserve Stock
	if err := l.ticketRepo.ReserveTickets(ctx, builtItems, d.GetUser()); err != nil {
		return nil, fmt.Errorf("failed to reserve tickets: %w", err)
	}

	// --- 建立並儲存稽核紀錄 ---
	// 呼叫新的輔助函式來準備 auditLog 物件
	auditLog := buildCreateOrderAuditLog(orderModel, builtItems)

	if err := l.auditLogRepo.Create(ctx, auditLog); err != nil {
		l.logger.Error("addOrder: Failed to create audit log", zap.Error(err))
		return nil, err
	}

	// 4. 創建全額帳單
	bill := &models.Bill{
		ID:       primitive.NewObjectID(),
		Order:    orderModel.ID,
		Customer: orderModel.User,
		Amount:   orderModel.Total,
		Type:     constants.BillTypePayment,
		Status:   constants.BillStatusPending.String(),
		PaymentMethod: &models.PaymentMethodInfo{
			ID:   d.GetPaymentMethod().ID(),
			Name: d.GetPaymentMethod().Name(),
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Payment:   nil,
		Note:      "",
	}
	_, err = l.billRepo.CreateBill(ctx, bill)
	if err != nil {
		return nil, fmt.Errorf("failed to create bill: %w", err)
	}

	// 5. Publish payment creation event via OutboxPublisher
	if err := l.paymentEventPublisher.PublishPaymentEvent(ctx, constants.PaymentActionCreate, bill, d.GetMerchantID().Hex()); err != nil {
		l.logger.Error("_addOrder: Failed to publish payment event", zap.Error(err), zap.Stringer("orderID", orderModel.ID))
		return nil, err // Return error to rollback transaction
	}

	//relation
	if l.relationClient != nil {
		if err := l.relationClient.AddUserResourceRole(ctx, d.GetMerchantID().Hex(), constants.ResourceOrder, orderModel.ID.Hex(), relation.RoleOwner); err != nil {
			l.logger.Error("addOrder: Failed to add merchant resource role", zap.Error(err))
		}

		if err := l.relationClient.AddUserResourceRole(ctx, d.GetMerchantID().Hex(), constants.ResourceBill, bill.ID.Hex(), relation.RoleOwner); err != nil {
			l.logger.Error("addOrder: Failed to add bill resource role", zap.Error(err))
		}
	}

	response := &dto.AddOrderResponse{
		OrderID: orderModel.ID,
		BillID:  bill.ID,
	}
	return response, nil
}

func (l *orderLogic) validateAndBuildOrderItems(ctx context.Context, orderItemsDTO []*dto.OrderItem, now time.Time, user *models.User) (builtItems []*models.OrderItem, subtotal primitive.Decimal128, total primitive.Decimal128, eventID, sessionID primitive.ObjectID, err error) {
	if len(orderItemsDTO) == 0 {
		return nil, primitive.Decimal128{}, primitive.Decimal128{}, primitive.NilObjectID, primitive.NilObjectID, errors.New("order cannot be empty")
	}

	ticketIDs := make([]primitive.ObjectID, len(orderItemsDTO))
	itemsByTicketID := make(map[primitive.ObjectID]*dto.OrderItem, len(orderItemsDTO))
	for i, item := range orderItemsDTO {
		ticketIDs[i] = item.TicketStock()
		itemsByTicketID[item.TicketStock()] = item
	}

	//取得購買的票券
	tickets, err := l.ticketRepo.GetValidTicketsByIDs(ctx, ticketIDs)
	if err != nil {
		return nil, primitive.Decimal128{}, primitive.Decimal128{}, primitive.NilObjectID, primitive.NilObjectID, fmt.Errorf("failed to fetch tickets: %w", err)
	}
	if len(tickets) != len(ticketIDs) {
		return nil, primitive.Decimal128{}, primitive.Decimal128{}, primitive.NilObjectID, primitive.NilObjectID, errors.New("one or more tickets are invalid or not found")
	}

	// Check if all tickets belong to the same event and get the event ID
	eventID = tickets[0].Type.Event
	sessionID = tickets[0].Stock.Session
	for _, t := range tickets[1:] {
		if t.Type.Event != eventID {
			return nil, primitive.Decimal128{}, primitive.Decimal128{}, primitive.NilObjectID, primitive.NilObjectID, errors.New("all tickets must belong to the same event")
		}
		if t.Stock.Session != sessionID {
			return nil, primitive.Decimal128{}, primitive.Decimal128{}, primitive.NilObjectID, primitive.NilObjectID, errors.New("all tickets must belong to the same session")
		}
	}

	// Initialize big.Float for precise subtotal calculation
	subtotalBigFloat := new(big.Float).SetPrec(big.MaxPrec)

	//generate order item
	builtItems = make([]*models.OrderItem, 0)
	for _, t := range tickets {
		itemDTO, ok := itemsByTicketID[t.Stock.ID]
		if !ok {
			return nil, primitive.Decimal128{}, primitive.Decimal128{}, primitive.NilObjectID, primitive.NilObjectID, fmt.Errorf("logic error: fetched ticket %s not in original request", t.Type.ID.Hex())
		}

		// check stock
		if t.Stock.Quantity < itemDTO.Quantity() {
			return nil, primitive.Decimal128{}, primitive.Decimal128{}, primitive.NilObjectID, primitive.NilObjectID, fmt.Errorf("not enough stock for ticket %s. available: %d, requested: %d", t.Type.Name, t.Stock.Quantity, itemDTO.Quantity())
		}

		//確認這單有沒有超過買超過一單限制或少於一單限制票數
		if t.Type.MaxQuantityPerOrder < itemDTO.Quantity() || t.Type.MinQuantityPerOrder > itemDTO.Quantity() {
			return nil, primitive.Decimal128{}, primitive.Decimal128{}, primitive.NilObjectID, primitive.NilObjectID, fmt.Errorf("break the ticket config rule,ticket_id:%s, max_quantity_per_order:%d, min_quantity_per_order:%d, user_quantity:%d", t.Stock.ID.Hex(), t.Type.MaxQuantityPerOrder, t.Type.MinQuantityPerOrder, itemDTO.Quantity())
		}
		//TODO:確認有沒有超過場次容量

		// Convert ticket price (Decimal128) to big.Float
		priceBigFloat, _, err := new(big.Float).SetPrec(big.MaxPrec).Parse(t.Type.Price.String(), 10)
		if err != nil {
			return nil, primitive.Decimal128{}, primitive.Decimal128{}, primitive.NilObjectID, primitive.NilObjectID, fmt.Errorf("failed to parse ticket price '%s' to big.Float: %w", t.Type.Price.String(), err)
		}

		// Convert item quantity (uint32) to big.Float
		quantityBigFloat := new(big.Float).SetPrec(big.MaxPrec).SetUint64(uint64(itemDTO.Quantity()))

		// Calculate line price using big.Float
		linePriceBigFloat := new(big.Float).SetPrec(big.MaxPrec).Mul(priceBigFloat, quantityBigFloat)

		// Add to subtotalBigFloat
		subtotalBigFloat.Add(subtotalBigFloat, linePriceBigFloat)

		// Unroll the quantity into individual items
		for i := 0; i < int(itemDTO.Quantity()); i++ {
			builtItems = append(builtItems, &models.OrderItem{
				ID:          primitive.NewObjectID(),
				TicketStock: t.Stock.ID,
				Session:     t.Stock.Session,
				Name:        t.Type.Name,
				Price:       t.Type.Price, // Use the original Decimal128 price
				Quantity:    1,            // Quantity is always 1 for each unrolled item
				Status:      constants.OrderItemStatusPendingPayment.String(),
				CreatedAt:   now,
				UpdatedAt:   now,
				UpdatedBy:   user,
			})
		}
	}

	subtotal, err = primitive.ParseDecimal128(subtotalBigFloat.String())
	if err != nil {
		return nil, primitive.Decimal128{}, primitive.Decimal128{}, primitive.NilObjectID, primitive.NilObjectID, fmt.Errorf("failed to convert calculated subtotal to Decimal128: %w", err)
	}

	// TODO:目前total跟subtotal會一致，直到啟用coupon或是可以設定折扣之類的
	total = subtotal

	return builtItems, subtotal, total, eventID, sessionID, nil
}

func (l *orderLogic) IsEventHasOpenOrders(ctx context.Context, eid primitive.ObjectID) (bool, error) {
	return l.orderRepo.IsEventHasOpenOrders(ctx, eid)
}

func (l *orderLogic) GetOrdersByEvent(ctx context.Context, eid primitive.ObjectID, pageReq *pagination.PageRequest) (*pagination.PageResult, error) {
	// 1. Prepare repository parameters from the page request
	params := &repository.GetOrdersByEventParams{
		EventID: eid,
		Limit:   pageReq.GetLimit(),
		Offset:  pageReq.GetOffset(),
	}

	// 2. Call the repository to get data and total count
	orders, total, err := l.orderRepo.GetOrdersByEvent(ctx, params)
	if err != nil {
		return nil, err
	}

	// 3. Create and return the page result
	return pagination.NewPageResult(orders, total, pageReq), nil
}

func (l *orderLogic) GetOrdersBySession(ctx context.Context, sid primitive.ObjectID, pageReq *pagination.PageRequest) (*pagination.PageResult, error) {
	// 1. Prepare repository parameters from the page request
	params := &repository.GetOrdersBySessionParams{
		SessionID: sid,
		Limit:     pageReq.GetLimit(),
		Offset:    pageReq.GetOffset(),
	}

	// 2. Call the repository to get data and total count
	orders, total, err := l.orderRepo.GetOrdersBySession(ctx, params)
	if err != nil {
		return nil, err
	}

	// 3. Create and return the page result
	return pagination.NewPageResult(orders, total, pageReq), nil
}

func (l *orderLogic) GetOrderDetails(ctx context.Context, orderID primitive.ObjectID) (*dto.OrderDetails, error) {
	var (
		order *models.Order
		items []*models.OrderItem
		bills []*models.Bill
	)

	g, gCtx := errgroup.WithContext(ctx)

	// Goroutine to fetch the main order
	g.Go(func() error {
		var err error
		order, err = l.orderRepo.GetOrderByID(gCtx, orderID)
		if err != nil {
			return fmt.Errorf("failed to get order by id: %w", err)
		}
		return nil
	})

	// Goroutine to fetch order items
	g.Go(func() error {
		var err error
		items, err = l.orderRepo.GetOrderItemsByOrderID(gCtx, orderID)
		if err != nil {
			return fmt.Errorf("failed to get order items by order id: %w", err)
		}
		return nil
	})

	// Goroutine to fetch bills
	g.Go(func() error {
		var err error
		bills, err = l.billRepo.GetBillsByOrderID(gCtx, orderID)
		if err != nil {
			return fmt.Errorf("failed to get bills by order id: %w", err)
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return &dto.OrderDetails{
		Order: order,
		Items: items,
		Bills: bills,
	}, nil
}

func (l *orderLogic) UpdateOrderItemStatus(ctx context.Context, d *dto.UpdateOrderItemStatusRequest) error {
	return l._updateOrderItemStatus(ctx, d)
}

func (l *orderLogic) RequestOrderItemRefund(ctx context.Context, d *dto.RequestOrderItemRefundRequest) error {
	// Here we are translating the specific "refund request" into a general "status update".
	updateStatusRequest := dto.NewUpdateOrderItemStatusRequest(
		d.GetOrderItemID(),
		d.GetOrderID(),
		constants.OrderItemStatusReturnRequested.String(),
		d.GetReason(),
		d.GetOperator(),
	)

	return l._updateOrderItemStatus(ctx, updateStatusRequest)
}

func (l *orderLogic) RejectOrderItemRefund(ctx context.Context, d *dto.RejectOrderItemRefundRequest) error {
	// The target status after rejection is its original state, which is Fulfilled.
	// The downstream _updateOrderItemStatus will validate the current status and order ID match.
	updateStatusRequest := dto.NewUpdateOrderItemStatusRequest(
		d.GetOrderItemID(),
		d.GetOrderID(),
		constants.OrderItemStatusFulfilled.String(), // Return to original status
		d.GetReason(),
		d.GetOperator(),
	)

	// Use the generic status update logic.
	return l._updateOrderItemStatus(ctx, updateStatusRequest)
}

func (l *orderLogic) ExportEventOrdersByMonth(ctx context.Context, eventID primitive.ObjectID, year int, month int) (string, []byte, error) {
	orders, err := l.orderRepo.GetOrdersByEventAndMonth(ctx, eventID, year, month)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get orders for export: %w", err)
	}

	// Generate filename
	filename := fmt.Sprintf("orders-%d-%02d.csv", year, month)

	// Use a buffer to write CSV data
	var buffer bytes.Buffer
	w := csv.NewWriter(&buffer)

	// Write header row
	header := []string{
		"訂單編號", "訂單狀態", "建立日期", "消費者姓名", "電子信箱",
		"購買票券", "票券價格", "繳費狀態",
	}
	if err := w.Write(header); err != nil {
		return "", nil, fmt.Errorf("failed to write csv header: %w", err)
	}

	// Write data rows
	for _, order := range orders {
		for _, item := range order.Items {
			record := []string{
				strconv.FormatUint(order.Serial, 10),
				order.Status,
				order.CreatedAt.Format(time.RFC3339),
				order.User.Name,
				order.User.Email,
				item.Name,
				item.Price.String(),
				item.Status,
			}
			if err := w.Write(record); err != nil {
				return "", nil, fmt.Errorf("failed to write csv record: %w", err)
			}
		}
	}

	w.Flush()

	if err := w.Error(); err != nil {
		return "", nil, fmt.Errorf("csv writer error: %w", err)
	}

	return filename, buffer.Bytes(), nil
}

func (l *orderLogic) _updateOrderItemStatus(ctx context.Context, d *dto.UpdateOrderItemStatusRequest) error {
	// 1. Get the specific order item to check its current state and details
	orderItem, err := l.orderRepo.GetOrderItemWithOrder(ctx, d.GetOrderItemID())
	if err != nil {
		return fmt.Errorf("failed to get order item: %w", err)
	}
	if orderItem.Order != d.GetOrderID() {
		return fmt.Errorf("order and order item's order matched failed")
	}

	// 2. Validate status transition
	if !l.canUpdateOrderItemStatus(constants.ParseOrderItemStatus(orderItem.Status), constants.ParseOrderItemStatus(d.GetNewStatus())) {
		return fmt.Errorf("invalid status transition from %s to %s", orderItem.Status, d.GetNewStatus())
	}
	// 3. Update the single OrderItem's status
	var returnInfo *models.ReturnInfo
	if constants.ParseOrderItemStatus(d.GetNewStatus()) == constants.OrderItemStatusReturnRequested {
		returnInfo = &models.ReturnInfo{
			Reason:      d.GetReason(),
			RequestedAt: time.Now(),
		}
	}
	err = l.orderRepo.UpdateOrderItemStatus(ctx, &repository.UpdateOrderItemStatusParams{
		ItemID:     d.GetOrderItemID(),
		Status:     d.GetNewStatus(),
		ReturnInfo: returnInfo,
	})
	if err != nil {
		return fmt.Errorf("failed to update order item status: %w", err)
	}

	// 4. If the item was cancelled, release the ticket stock
	newStatus := constants.ParseOrderItemStatus(d.GetNewStatus())
	if newStatus == constants.OrderItemStatusCanceled {
		// ReleaseTickets expects a slice, so we create one with the single item
		itemsToRelease := []*models.OrderItem{orderItem.OrderItem}
		if err := l.ticketRepo.ReleaseTickets(ctx, itemsToRelease); err != nil {
			l.logger.Error("_updateOrderItemStatus: ReleaseTickets failed", zap.Error(err))
			return err
		}
		//total、subtotal要剪掉orderitems.price
		minusPrice, err := helper.NegateDecimal128(orderItem.Price)
		if err != nil {
			return fmt.Errorf("failed to negate orderItem price: %w", err)
		}
		err = l.orderRepo.UpdateOrder(ctx, d.GetOrderID(), repository.WithIncTotal(minusPrice), repository.WithIncSubtotal(minusPrice), repository.WithUpdatedBy(d.GetOperator()))
		if err != nil {
			l.logger.Error("_updateOrderItemStatus: Update order total and subtotal failed", zap.Error(err))
			return err
		}
	}

	if newStatus == constants.OrderItemStatusReturnApproved {
		//create bill
		_, err := l.billRepo.CreateBill(ctx, &models.Bill{
			ID:            primitive.NewObjectID(),
			Order:         orderItem.Order,
			Customer:      orderItem.OrderInfo.User,
			Amount:        orderItem.Price,
			Type:          constants.BillTypeRefund,
			Status:        constants.BillStatusPending.String(),
			PaymentMethod: nil,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
			Payment:       nil,
		})
		if err != nil {
			l.logger.Error("_updateOrderItemStatus: Create bill failed", zap.Error(err))
		}
	}

	//Recalculate the parent order's status if needed
	if err := l.recalculateOrderStatus(ctx, d.GetOrderID(), d.GetOperator(), nil); err != nil {
		return fmt.Errorf("failed to recalculate order status: %w", err)
	}

	// 6. Create a detailed audit log for this specific item change
	auditLog := buildUpdateOrderItemStatusAuditLog(d.GetOperator(), d.GetReason(), orderItem.OrderItem, d.GetNewStatus())
	if err := l.auditLogRepo.Create(ctx, auditLog); err != nil {
		l.logger.Error("_updateOrderItemStatus: Failed to create audit log", zap.Error(err))
		// Non-critical error, just log it.
	}

	return nil
}

func (l *orderLogic) canUpdateOrderItemStatus(o, n constants.OrderItemStatus) bool {
	switch n {
	case constants.OrderItemStatusCanceled:
		if o != constants.OrderItemStatusPendingPayment {
			return false
		}
	case constants.OrderItemStatusReturnRequested:
		if o != constants.OrderItemStatusFulfilled {
			return false
		}
	case constants.OrderItemStatusReturnApproved:
		if o != constants.OrderItemStatusReturnRequested {
			return false
		}
	case constants.OrderItemStatusReturned:
		if o != constants.OrderItemStatusReturnApproved {
			return false
		}
	case constants.OrderItemStatusFulfilled:
		if o != constants.OrderItemStatusReturnRequested && o != constants.OrderItemStatusPaid {
			return false
		}
	default:
		return true
	}
	return true
}

// MarkOrderItemsAsFulfilled marks all eligible items in an order as fulfilled and recalculates the order status.
// This method expects to be called within an existing transaction.
func (l *orderLogic) MarkOrderItemsAsFulfilled(ctx context.Context, orderID primitive.ObjectID) error {
	// 1. Update all relevant OrderItem statuses to "Delivered"
	// We assume items in "Paid" status are ready to be fulfilled.
	_, err := l.orderRepo.UpdateItemsStatusByOrder(
		ctx,
		orderID,
		constants.OrderItemStatusPaid.String(),
		constants.OrderItemStatusFulfilled.String(),
		models.SystemUser,
	)
	if err != nil {
		return fmt.Errorf("failed to update order items to delivered: %w", err)
	}

	// 2. Call the internal method to recalculate the main order's status.
	// The operator is the system itself as this is a system-driven process.
	if err := l.recalculateOrderStatus(ctx, orderID, models.SystemUser, nil); err != nil {
		return fmt.Errorf("failed to recalculate order status after fulfillment: %w", err)
	}

	return nil
}

// FindRefundableItemForBill finds the specific order item associated with a refund bill.
func (l *orderLogic) FindRefundableItemForBill(ctx context.Context, bill *models.Bill) (*models.OrderItem, error) {
	if bill.Type != constants.BillTypeRefund {
		return nil, fmt.Errorf("bill %s is not a refund bill", bill.ID.Hex())
	}
	if bill.OrderItemID == nil {
		return nil, fmt.Errorf("refund bill %s does not have an associated order item ID", bill.ID.Hex())
	}

	return l.orderRepo.GetOrderItemByID(ctx, *bill.OrderItemID)
}

func (l *orderLogic) recalculateOrderStatus(ctx context.Context, orderID primitive.ObjectID, operator *models.User, ticketEventPublisher *TicketEventPublisher) error {
	//重新計算應付的，也就是除了canceled的order item的總額
	//去bill找對應已成功付款的bill總額看有沒有match跟order.paid，如果發現吻合則order.status更新為paid，等待發票，底下的項目不是delivered狀態的都要變成delivered並且發放票券，發票完成之後就轉為completed
	//如果全部orderItem都是returned則更新帳單為returned，若不是全部但是有returned的order item，則訂單為partially_returned
	//若bill總額跟order所需付的total沒有match，並且bill總額大於0，則order status為partially paid
	//反正我們一定是全款項付完之後才能申請returned

	// 1. get orderWithItems with items
	orderWithItems, err := l.orderRepo.GetOrderWithItemsByID(ctx, orderID)
	if err != nil {
		return fmt.Errorf("failed to get orderWithItems by id: %w", err)
	}

	// Store the original status for audit logging
	oldStatus := orderWithItems.Order.Status

	// 2. switch orderWithItems status
	//碰到的status一步步補充就好
	switch orderWithItems.Order.Status {
	case constants.OrderStatusUnpaid.String():
		//canceled：當所有order item都變canceled狀態的時候
		allCanceledFlag := true
		for _, i := range orderWithItems.Items {
			if i.Status != constants.OrderItemStatusCanceled.String() {
				allCanceledFlag = false
				break
			}
		}
		if allCanceledFlag {
			//update orderWithItems status to cancel
			newStatus := constants.OrderStatusCanceled.String()
			err = l.orderRepo.UpdateOrder(ctx, orderID, repository.WithStatus(newStatus), repository.WithUpdatedBy(operator))
			if err != nil {
				return fmt.Errorf("failed to update order status to canceled: %w", err)
			}
			// Create audit log for status change
			if oldStatus != newStatus {
				auditLog := buildOrderStatusChangeAuditLog(operator, orderID, oldStatus, newStatus, "All order items were canceled")
				if auditErr := l.auditLogRepo.Create(ctx, auditLog); auditErr != nil {
					l.logger.Error("recalculateOrderStatus: Failed to create audit log for canceled status", zap.Error(auditErr))
				}
			}

			return nil
		}

		//paid：當paid等於total時
		cmpTotalAndPaid, err := helper.CompareDecimal128(orderWithItems.Order.Total, orderWithItems.Order.Paid)
		if err != nil {
			return fmt.Errorf("failed to compare total and paid: %w", err)
		}
		if cmpTotalAndPaid == 0 {
			//err = l.orderRepo.UpdateOrder(ctx, orderID, repository.WithStatus(constants.OrderStatusPaid.String()))
			err = l.updateOrderToPaidWithAudit(ctx, orderID, orderWithItems.Items, operator, oldStatus, ticketEventPublisher)

			if err != nil {
				return fmt.Errorf("failed to update order status to paid: %w", err)
			}
			return nil
		}
		//partially_paid：當自身的paid已經不為0的時候
		paid, err := helper.Decimal128ToInt64(orderWithItems.Order.Paid)
		if err != nil {
			return fmt.Errorf("failed to convert paid to int64: %w", err)
		}

		if paid > 0 {
			newStatus := constants.OrderStatusPartiallyPaid.String()
			err = l.orderRepo.UpdateOrder(ctx, orderID, repository.WithStatus(newStatus), repository.WithUpdatedBy(operator))
			if err != nil {
				return fmt.Errorf("failed to update order status to partially paid: %w", err)
			}
			// Create audit log for status change
			if oldStatus != newStatus {
				auditLog := buildOrderStatusChangeAuditLog(operator, orderID, oldStatus, newStatus, "Partial payment received")
				if auditErr := l.auditLogRepo.Create(ctx, auditLog); auditErr != nil {
					l.logger.Error("recalculateOrderStatus: Failed to create audit log for partially paid status", zap.Error(auditErr))
				}
			}
			return nil
		}
	case constants.OrderStatusPartiallyPaid.String():
		//paid
		cmpTotalAndPaid, err := helper.CompareDecimal128(orderWithItems.Total, orderWithItems.Paid)
		if err != nil {
			return fmt.Errorf("failed to compare total and paid: %w", err)
		}
		if cmpTotalAndPaid == 0 {
			err = l.updateOrderToPaidWithAudit(ctx, orderID, orderWithItems.Items, operator, oldStatus, ticketEventPublisher)
			if err != nil {
				return fmt.Errorf("failed to update order status to paid: %w", err)
			}
			return nil

			//itemCounts := 0
			//for _, i := range orderWithItems.Items {
			//	if i.Status == constants.OrderItemStatusPendingPayment.String() {
			//		itemCounts++
			//	}
			//}
			////寫入items_count
			////更改status
			//err = l.orderRepo.UpdateOrder(ctx, orderID, repository.WithStatus(constants.OrderStatusPaid.String()), repository.WithItemsCount(itemCounts))
			//if err != nil {
			//	return fmt.Errorf("failed to update order status to paid: %w", err)
			//}
			////更改order item的所有OrderItemStatusPendingPayment為paid
			//_, err = l.orderRepo.UpdateItemsStatusByOrder(ctx, orderID, constants.OrderItemStatusPendingPayment.String(), constants.OrderItemStatusPaid.String())
			//if err != nil {
			//	return fmt.Errorf("failed to update order item status to paid: %w", err)
			//}
		}
		//pending_refund
		if cmpTotalAndPaid == -1 {
			newStatus := constants.OrderStatusPendingRefund.String()
			err = l.orderRepo.UpdateOrder(ctx, orderID, repository.WithStatus(newStatus), repository.WithUpdatedBy(operator))
			if err != nil {
				return fmt.Errorf("failed to update order status to pending_refund: %w", err)
			}
			// Create audit log for status change
			if oldStatus != newStatus {
				auditLog := buildOrderStatusChangeAuditLog(operator, orderID, oldStatus, newStatus, "Overpayment detected, pending refund")
				if auditErr := l.auditLogRepo.Create(ctx, auditLog); auditErr != nil {
					l.logger.Error("recalculateOrderStatus: Failed to create audit log for pending refund status", zap.Error(auditErr))
				}
			}
			difference, err := helper.SubDecimal128(orderWithItems.Paid, orderWithItems.Total)
			if err != nil {
				return fmt.Errorf("failed to subtract paid and total : %w", err)
			}
			_, err = l.billRepo.CreateBill(ctx, &models.Bill{
				ID:            primitive.NewObjectID(),
				Order:         orderID,
				Customer:      orderWithItems.Order.User,
				Amount:        difference,
				Type:          constants.BillTypeRefund,
				Status:        constants.BillStatusPending.String(),
				PaymentMethod: nil,
				CreatedAt:     time.Time{},
				UpdatedAt:     time.Time{},
			})
			if err != nil {
				return fmt.Errorf("failed to create refund bill: %w", err)
			}
			return nil
		}
		//canceled
		cmpReturnedAndPaid, err := helper.CompareDecimal128(orderWithItems.Returned, orderWithItems.Paid)
		if err != nil {
			return fmt.Errorf("failed to compare order returned and paid: %w", err)
		}
		if cmpReturnedAndPaid == 0 {
			newStatus := constants.OrderStatusCanceled.String()
			err = l.orderRepo.UpdateOrder(ctx, orderID, repository.WithStatus(newStatus), repository.WithUpdatedBy(operator))
			if err != nil {
				return fmt.Errorf("failed to update order status to canceled: %w", err)
			}
			// Create audit log for status change
			if oldStatus != newStatus {
				auditLog := buildOrderStatusChangeAuditLog(operator, orderID, oldStatus, newStatus, "Full refund completed, order canceled")
				if auditErr := l.auditLogRepo.Create(ctx, auditLog); auditErr != nil {
					l.logger.Error("recalculateOrderStatus: Failed to create audit log for canceled status", zap.Error(auditErr))
				}
			}
			return nil
		}
	//pending_refund：當目前order item有canceled的，且paid不為0，且paid大於目前訂單所付金額的話
	//前提只會是partially或是paid(微乎其微，雖然還未履約)
	//要通知payment執行退款，並且執行完成後要變paid狀態
	case constants.OrderStatusPendingRefund.String():
		netPaid, err := helper.SubDecimal128(orderWithItems.Paid, orderWithItems.Returned)
		if err != nil {
			return fmt.Errorf("failed to subtract paid and returned : %w", err)
		}
		//paid：(paid - refunded) == total
		f, err := helper.CompareDecimal128(netPaid, orderWithItems.Total)
		if err != nil {
			return fmt.Errorf("failed to compare paid and total : %w", err)
		}
		if f == 0 {
			err = l.updateOrderToPaidWithAudit(ctx, orderID, orderWithItems.Items, operator, oldStatus, ticketEventPublisher)
			//err = l.orderRepo.UpdateOrder(ctx, orderID, repository.WithStatus(constants.OrderStatusPaid.String()))
			if err != nil {
				return fmt.Errorf("failed to update order status to paid: %w", err)
			}
		}

	case constants.OrderStatusPaid.String():
		//completed
		noUnpaidItemFlag := true
		paidItemCount := 0
		for _, item := range orderWithItems.Items {
			itemStatus := item.Status
			if itemStatus == constants.OrderItemStatusPendingPayment.String() {
				noUnpaidItemFlag = false
			}
			if itemStatus == constants.OrderItemStatusFulfilled.String() {
				paidItemCount++
			}
		}
		if noUnpaidItemFlag {
			newStatus := constants.OrderStatusCompleted.String()
			err = l.orderRepo.UpdateOrder(ctx, orderID, repository.WithStatus(newStatus), repository.WithItemsCount(paidItemCount), repository.WithUpdatedBy(operator))
			if err != nil {
				return fmt.Errorf("failed to update order status to completed: %w", err)
			}
			// Create audit log for status change
			if oldStatus != newStatus {
				auditLog := buildOrderStatusChangeAuditLog(operator, orderID, oldStatus, newStatus, "All items processed, order completed")
				if auditErr := l.auditLogRepo.Create(ctx, auditLog); auditErr != nil {
					l.logger.Error("recalculateOrderStatus: Failed to create audit log for completed status", zap.Error(auditErr))
				}
			}
		}
	case constants.OrderStatusCompleted.String():
		//partially_returned
		//if (orderWithItems.ReturnedItemsCount > 0) && (orderWithItems.ItemsCount > orderWithItems.ReturnedItemsCount) {
		//	err = l.orderRepo.UpdateOrder(ctx, orderID, repository.WithStatus(constants.OrderStatusPartiallyReturned.String()))
		//}
		//return in process
		updateToReturnInProgressFlag := false
		for _, item := range orderWithItems.Items {
			if (item.Status == constants.OrderItemStatusReturnApproved.String()) || (item.Status == constants.OrderItemStatusReturnRequested.String()) {
				updateToReturnInProgressFlag = true
				break
			}
		}
		if updateToReturnInProgressFlag {
			newStatus := constants.OrderStatusReturnInProgress.String()
			err = l.orderRepo.UpdateOrder(ctx, orderID, repository.WithStatus(newStatus), repository.WithUpdatedBy(operator))
			if err != nil {
				return fmt.Errorf("failed to update order status to return_in_progress: %w", err)
			}
			// Create audit log for status change
			if oldStatus != newStatus {
				auditLog := buildOrderStatusChangeAuditLog(operator, orderID, oldStatus, newStatus, "Return request initiated")
				if auditErr := l.auditLogRepo.Create(ctx, auditLog); auditErr != nil {
					l.logger.Error("recalculateOrderStatus: Failed to create audit log for return in progress status", zap.Error(auditErr))
				}
			}
		}
	case constants.OrderStatusReturnInProgress.String():
		// Check if there are still items in a return-related in-progress state.
		inProgressFlag := false
		for _, item := range orderWithItems.Items {
			if item.Status == constants.OrderItemStatusReturnApproved.String() || item.Status == constants.OrderItemStatusReturnRequested.String() {
				inProgressFlag = true
				break
			}
		}

		// If no items are in progress, we can finalize the order's return status.
		if !inProgressFlag {
			if orderWithItems.ReturnedItemsCount == 0 {
				newStatus := constants.OrderStatusCompleted.String()
				err = l.orderRepo.UpdateOrder(ctx, orderID, repository.WithStatus(newStatus), repository.WithUpdatedBy(operator))
				if err != nil {
					return fmt.Errorf("failed to update order status to completed: %w", err)
				}
				// Create audit log for status change
				if oldStatus != newStatus {
					auditLog := buildOrderStatusChangeAuditLog(operator, orderID, oldStatus, newStatus, "Return process completed, back to completed")
					if auditErr := l.auditLogRepo.Create(ctx, auditLog); auditErr != nil {
						l.logger.Error("recalculateOrderStatus: Failed to create audit log for completed status", zap.Error(auditErr))
					}
				}
			} else {
				var newStatus string
				// Compare total items count with returned items count to set the correct final status.
				if orderWithItems.ItemsCount == orderWithItems.ReturnedItemsCount {
					newStatus = constants.OrderStatusReturned.String()
				} else if orderWithItems.ItemsCount > orderWithItems.ReturnedItemsCount {
					newStatus = constants.OrderStatusPartiallyReturned.String()
				} else {
					// This case should not happen, indicates a data inconsistency.
					l.logger.Error("recalculateOrderStatus: ReturnedItemsCount is greater than ItemsCount",
						zap.Stringer("orderID", orderID),
						zap.Int("itemsCount", orderWithItems.ItemsCount),
						zap.Int("returnedItemsCount", orderWithItems.ReturnedItemsCount))
					return fmt.Errorf("data inconsistency: returned items count (%d) is greater than total items count (%d)",
						orderWithItems.ReturnedItemsCount, orderWithItems.ItemsCount)
				}

				err = l.orderRepo.UpdateOrder(ctx, orderID, repository.WithStatus(newStatus), repository.WithUpdatedBy(operator))
				if err != nil {
					return fmt.Errorf("failed to update order status to %s: %w", newStatus, err)
				}
				// Create audit log for status change
				if oldStatus != newStatus {
					auditLog := buildOrderStatusChangeAuditLog(operator, orderID, oldStatus, newStatus, "All returns processed, order status finalized.")
					if auditErr := l.auditLogRepo.Create(ctx, auditLog); auditErr != nil {
						l.logger.Error("recalculateOrderStatus: Failed to create audit log for returned status", zap.Error(auditErr))
					}
				}
			}
		}
	case constants.OrderStatusPartiallyReturned.String():
		updateToReturnInProgressFlag := false
		for _, item := range orderWithItems.Items {
			if (item.Status == constants.OrderItemStatusReturnApproved.String()) || (item.Status == constants.OrderItemStatusReturnRequested.String()) {
				updateToReturnInProgressFlag = true
				break
			}
		}
		if updateToReturnInProgressFlag {
			newStatus := constants.OrderStatusReturnInProgress.String()
			err = l.orderRepo.UpdateOrder(ctx, orderID, repository.WithStatus(newStatus), repository.WithUpdatedBy(operator))
			if err != nil {
				return fmt.Errorf("failed to update order status to return_in_progress: %w", err)
			}
			// Create audit log for status change
			if oldStatus != newStatus {
				auditLog := buildOrderStatusChangeAuditLog(operator, orderID, oldStatus, newStatus, "Additional return request initiated")
				if auditErr := l.auditLogRepo.Create(ctx, auditLog); auditErr != nil {
					l.logger.Error("recalculateOrderStatus: Failed to create audit log for return in progress status", zap.Error(auditErr))
				}
			}
		}

		if orderWithItems.ReturnedItemsCount == orderWithItems.ItemsCount {
			newStatus := constants.OrderStatusReturned.String()
			err = l.orderRepo.UpdateOrder(ctx, orderID, repository.WithStatus(newStatus), repository.WithUpdatedBy(operator))
			if err != nil {
				return fmt.Errorf("failed to update order status to returned: %w", err)
			}
			// Create audit log for status change
			if oldStatus != newStatus {
				auditLog := buildOrderStatusChangeAuditLog(operator, orderID, oldStatus, newStatus, "All items returned")
				if auditErr := l.auditLogRepo.Create(ctx, auditLog); auditErr != nil {
					l.logger.Error("recalculateOrderStatus: Failed to create audit log for returned status", zap.Error(auditErr))
				}
			}
		}

	default:
		panic("unhandled default case")

	}

	return nil
}

func canUpdateStatus(o, new string) (f bool) {
	newStatus := constants.ParseOrderStatus(new)
	oldStatus := constants.ParseOrderStatus(o)
	switch newStatus {
	case constants.OrderStatusUnpaid:
		return
	case constants.OrderStatusPartiallyPaid:
		if oldStatus != constants.OrderStatusUnpaid {
			return
		}
	case constants.OrderStatusPaid:
		if oldStatus != constants.OrderStatusPartiallyPaid {
			return
		}
	case constants.OrderStatusPartiallyReturned:
		if oldStatus == constants.OrderStatusPaid {
			return
		}
	case constants.OrderStatusReturned:
		if oldStatus != constants.OrderStatusPaid {
			return
		}
	default:
		return
	}
	return true
}

func (l *orderLogic) GetOrder(ctx context.Context, id primitive.ObjectID) (*models.Order, error) {
	order, err := l.orderRepo.GetOrderByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return order, nil
}

func (l *orderLogic) GetOrdersByUser(ctx context.Context, uid primitive.ObjectID, token pagination.PageToken) ([]*dto.OrderWithItems, pagination.PageToken, error) {
	const pageSize = 5 // Or from config

	page, err := token.Decode()
	if err != nil {
		return nil, "", err
	}

	params := &repository.GetOrdersByUserParams{
		UserID: uid,
		Limit:  pageSize,
	}
	if page != nil {
		cursorID, err := primitive.ObjectIDFromHex(page.CursorID)
		if err != nil {
			return nil, "", pagination.ErrInvalidToken
		}
		params.CursorID = cursorID
		params.CursorCreatedAt = time.Unix(page.CursorTimestamp, 0)
	}

	// Call the repository, which now returns the data in the desired shape
	ordersWithItems, err := l.orderRepo.GetOrdersByUser(ctx, params)
	if err != nil {
		return nil, "", err
	}

	var nextPageToken pagination.PageToken
	if len(ordersWithItems) == pageSize {
		lastOrder := ordersWithItems[len(ordersWithItems)-1]
		// The cursor is based on the main order's properties
		token, err := pagination.GenerateToken(lastOrder.Order.ID, lastOrder.Order.CreatedAt)
		if err != nil {
			l.logger.Error("failed to generate next page token", zap.Error(err))
			return nil, "", err
		}
		nextPageToken = token
	}

	return ordersWithItems, nextPageToken, nil
}

func (l *orderLogic) updateOrderToPaidWithAudit(ctx context.Context, orderID primitive.ObjectID, items []*models.OrderItem, operator *models.User, oldStatus string, ticketEventPublisher *TicketEventPublisher) (err error) {
	newStatus := constants.OrderStatusPaid.String()
	//itemCounts := 0
	//for _, i := range items {
	//	if i.Status == constants.OrderItemStatusPendingPayment.String() {
	//		itemCounts++
	//	}
	//}
	err = l.orderRepo.UpdateOrder(ctx, orderID, repository.WithStatus(newStatus), repository.WithUpdatedBy(operator), repository.WithUpdatedAt(time.Now()))
	if err != nil {
		return fmt.Errorf("failed to update order status to paid: %w", err)
	}

	// Create audit log for status change
	if oldStatus != newStatus {
		auditLog := buildOrderStatusChangeAuditLog(operator, orderID, oldStatus, newStatus, "Payment completed, order fully paid")
		if auditErr := l.auditLogRepo.Create(ctx, auditLog); auditErr != nil {
			l.logger.Error("updateOrderToPaidWithAudit: Failed to create audit log for paid status", zap.Error(auditErr))
		}
	}

	//更改order item的所有OrderItemStatusPendingPayment為paid
	_, err = l.orderRepo.UpdateItemsStatusByOrder(ctx, orderID, constants.OrderItemStatusPendingPayment.String(), constants.OrderItemStatusPaid.String(), operator)
	if err != nil {
		return fmt.Errorf("failed to update order item status to paid: %w", err)
	}

	if err := ticketEventPublisher.PublishTicketGenerationTask(ctx, orderID); err != nil {
		// In this specific case, failing to create the ticket generation task
		// does not roll back the entire "update order to paid" transaction.
		// This preserves the original logic. If stricter consistency is needed,
		// this error should be returned to the caller.
		l.logger.Error("updateOrderToPaidWithAudit: failed to publish ticket generation event", zap.Error(err), zap.Stringer("orderID", orderID))
	}

	//TODO:等到之後開發票要自動化寫在這

	return nil
}

func (l *orderLogic) CreateRefundBill(ctx context.Context, d *dto.CreateRefundBillRequest) (primitive.ObjectID, error) {
	now := time.Now()
	// 1. Get the order item to be refunded
	orderItem, err := l.orderRepo.GetOrderItemByID(ctx, d.GetOrderItemID())
	if err != nil {
		return primitive.NilObjectID, fmt.Errorf("failed to get order item '%s': %w", d.GetOrderItemID().Hex(), err)
	}

	// 2. Get the parent order to access user and merchant info
	order, err := l.orderRepo.GetOrderByID(ctx, orderItem.Order)
	if err != nil {
		return primitive.NilObjectID, fmt.Errorf("failed to get parent order '%s': %w", orderItem.Order.Hex(), err)
	}

	// 3. Validate that the item is in a refundable state
	itemStatus := constants.ParseOrderItemStatus(orderItem.Status)
	if itemStatus != constants.OrderItemStatusReturnRequested {
		return primitive.NilObjectID, fmt.Errorf("order item status is '%s', cannot be refunded", orderItem.Status)
	}

	// 4. Check for existing IN-PROGRESS or COMPLETED refunds
	refundedBills, err := l.billRepo.GetRefundsByOrderItemID(ctx, orderItem.ID)
	if err != nil {
		return primitive.NilObjectID, fmt.Errorf("failed to check for existing refunds for order item '%s': %w", orderItem.ID.Hex(), err)
	}
	// Check if there's already a refund that is being processed or is completed.
	for _, bill := range refundedBills {
		status := constants.ParseBillStatus(bill.Status)
		if status == constants.BillStatusPending || status == constants.BillStatusRefunded {
			return primitive.NilObjectID, fmt.Errorf("order item '%s' has a pending or successful refund and cannot be refunded again", orderItem.ID.Hex())
		}
	}

	// 5. Validate the requested refund amount against the item's total price
	if cmp, _ := helper.CompareDecimal128(d.GetAmount(), orderItem.Price); cmp > 0 {
		return primitive.NilObjectID, fmt.Errorf("refund amount %s exceeds item price %s", d.GetAmount().String(), orderItem.Price.String())
	}

	// 6. Construct the new refund bill model
	refundBill := &models.Bill{
		ID:          primitive.NewObjectID(),
		Order:       orderItem.Order,
		OrderItemID: &orderItem.ID,
		Customer:    order.User,
		Amount:      d.GetAmount(),
		Type:        constants.BillTypeRefund,
		Status:      constants.BillStatusPending.String(),
		PaymentMethod: &models.PaymentMethodInfo{
			ID:   d.GetPaymentMethod().ID(),
			Name: d.GetPaymentMethod().Name(),
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	// 7. Save the new refund bill
	billID, err := l.billRepo.CreateBill(ctx, refundBill)
	if err != nil {
		return primitive.NilObjectID, fmt.Errorf("failed to create refund bill: %w", err)
	}

	// 8. Publish refund processing event
	if err := l.paymentEventPublisher.PublishPaymentEvent(ctx, constants.PaymentActionRefund, refundBill, order.MerchantID.Hex()); err != nil {
		l.logger.Error("_createRefundBill: Failed to publish payment refund event", zap.Error(err), zap.Stringer("billID", billID))
		return primitive.NilObjectID, err
	}

	// 9. Create audit log
	auditLog := l.buildCreateRefundBillAuditLog(d.GetOperator(), refundBill, d.GetReason())
	if err := l.auditLogRepo.Create(ctx, auditLog); err != nil {
		l.logger.Error("_createRefundBill: Failed to create audit log", zap.Error(err))
		// Do not fail transaction for audit log failure
	}

	// 10. Update OrderItem status to show it has been approved for return/refund.
	if err := l.orderRepo.UpdateOrderItemStatus(ctx, &repository.UpdateOrderItemStatusParams{
		ItemID:     orderItem.ID,
		Status:     constants.OrderItemStatusReturnApproved.String(),
		ReturnInfo: nil,
	}); err != nil {
		l.logger.Error("_createRefundBill: Failed to update order item status", zap.Error(err), zap.Stringer("orderItemID", orderItem.ID))
		return primitive.NilObjectID, fmt.Errorf("failed to update order item status: %w", err)
	}

	// 11. Update parent Order status and returned amount
	updateOptions := []repository.UpdateOption{
		repository.WithStatus(constants.OrderStatusReturnInProgress.String()),
		repository.WithUpdatedBy(d.GetOperator()),
		repository.WithUpdatedAt(now),
	}
	if err := l.orderRepo.UpdateOrder(ctx, order.ID, updateOptions...); err != nil {
		l.logger.Error("_createRefundBill: Failed to update order status and returned total", zap.Error(err), zap.Stringer("orderID", order.ID))
		return primitive.NilObjectID, fmt.Errorf("failed to update order: %w", err)
	}

	return billID, nil
}

func (l *orderLogic) buildCreateRefundBillAuditLog(operator *models.User, bill *models.Bill, reason string) *models.AuditLog {
	details := map[string]interface{}{
		"reason":          reason,
		"refunded_amount": bill.Amount.String(),
		"order_item_id":   bill.OrderItemID.Hex(),
	}
	return NewAuditLog(operator, "CREATE_REFUND_BILL", "bill", bill.ID, details, bill)
}

// HandlePaymentSuccess is called after a payment bill has been successfully processed.
// Its sole responsibility is to trigger a recalculation of the order status.
func (l *orderLogic) HandlePaymentSuccess(ctx context.Context, orderID primitive.ObjectID, ticketEventPublisher *TicketEventPublisher) error {
	// The operator is the system itself in this context.
	return l.recalculateOrderStatus(ctx, orderID, models.SystemUser, ticketEventPublisher)
}

func (l *orderLogic) CancelOrder(ctx context.Context, d *dto.CancelOrderRequest) error {
	// 1. Get the order
	order, err := l.orderRepo.GetOrderByID(ctx, d.GetOrderID())
	if err != nil {
		return fmt.Errorf("failed to get order: %w", err)
	}

	// 2. State validation and branching logic
	switch order.Status {
	case constants.OrderStatusUnpaid.String():
		// --- Logic for UNPAID orders (Voiding the order) ---
		return l.voidUnpaidOrder(ctx, order, d.GetUser(), d.GetReason())

	//TODO:之後討論完相關流程後在處理，還有如果要這樣的話我要給payment什麼資訊
	//case constants.OrderStatusPaid.String(), constants.OrderStatusPartiallyPaid.String():
	//	// --- Logic for PAID orders (Requesting a refund) ---
	//	return l.requestRefundForPaidOrder(ctx, order, d.GetUser(), d.GetReason())

	default:
		// For other statuses (Canceled, PendingRefund, etc.), cancellation is not allowed.
		return fmt.Errorf("order cannot be cancelled in status: %s", order.Status)
	}
}

// voidUnpaidOrder handles the cancellation of an order that has not been paid.
func (l *orderLogic) voidUnpaidOrder(ctx context.Context, order *models.Order, operator *models.User, reason string) error {
	// Get order items to be released
	items, err := l.orderRepo.GetOrderItemsByOrderID(ctx, order.ID)
	if err != nil {
		return fmt.Errorf("failed to get order items for void: %w", err)
	}

	// Cancel all associated pending bills and notify payment service
	bills, err := l.billRepo.GetBillsByOrderID(ctx, order.ID)
	if err != nil {
		return fmt.Errorf("failed to get bills for void: %w", err)
	}
	for _, bill := range bills {
		if bill.Status == constants.BillStatusPending.String() {
			if err := l.billRepo.UpdateBill(ctx, bill.ID, repository.WithStatus(constants.BillStatusCanceled.String()), repository.WithUpdatedBy(operator)); err != nil {
				return fmt.Errorf("failed to cancel bill %s: %w", bill.ID.Hex(), err)
			}
			if err := l.paymentEventPublisher.PublishPaymentEvent(ctx, constants.PaymentActionCancel, bill, order.MerchantID.Hex()); err != nil {
				l.logger.Error("voidUnpaidOrder: Failed to publish payment cancellation event", zap.Error(err), zap.Stringer("billID", bill.ID))
				return err
			}
		}
	}

	// Cancel all associated pending order items
	if _, err := l.orderRepo.UpdateItemsStatusByOrder(ctx, order.ID, constants.OrderItemStatusPendingPayment.String(), constants.OrderItemStatusCanceled.String(), operator); err != nil {
		return fmt.Errorf("failed to cancel order items: %w", err)
	}

	// Release ticket stock
	if len(items) > 0 {
		if err := l.ticketRepo.ReleaseTickets(ctx, items); err != nil {
			return fmt.Errorf("failed to release tickets: %w", err)
		}
	}

	// Update order status
	newStatus := constants.OrderStatusCanceled.String()
	if err := l.orderRepo.UpdateOrder(ctx, order.ID, repository.WithStatus(newStatus), repository.WithUpdatedBy(operator)); err != nil {
		return fmt.Errorf("failed to update order status: %w", err)
	}

	// Create audit log
	auditLog := buildCancelOrderAuditLog(operator, order, reason)
	if err := l.auditLogRepo.Create(ctx, auditLog); err != nil {
		l.logger.Error("voidUnpaidOrder: Failed to create audit log", zap.Error(err))
	}

	return nil
}

// requestRefundForPaidOrder handles the cancellation of a paid/partially-paid order by initiating a refund.
func (l *orderLogic) requestRefundForPaidOrder(ctx context.Context, order *models.Order, operator *models.User, reason string) error {
	paidAmount, err := helper.Decimal128ToFloat64(order.Paid)
	if err != nil {
		return fmt.Errorf("invalid paid amount format: %w", err)
	}

	if paidAmount <= 0 {
		// This case might occur for a partially paid order that was adjusted to zero.
		// Treat it as a simple void without refund.
		return l.voidUnpaidOrder(ctx, order, operator, reason)
	}

	// Create a single refund bill for the entire paid amount.
	refundBill := &models.Bill{
		ID:        primitive.NewObjectID(),
		Order:     order.ID,
		Customer:  order.User,
		Amount:    order.Paid,
		Type:      constants.BillTypeRefund,
		Status:    constants.BillStatusPending.String(),
		Note:      "Full order cancellation refund",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if _, err := l.billRepo.CreateBill(ctx, refundBill); err != nil {
		return fmt.Errorf("failed to create refund bill: %w", err)
	}

	// Publish refund request to payment service
	if err := l.paymentEventPublisher.PublishPaymentEvent(ctx, constants.PaymentActionRefund, refundBill, order.MerchantID.Hex()); err != nil {
		l.logger.Error("requestRefundForPaidOrder: Failed to publish refund event", zap.Error(err), zap.Stringer("billID", refundBill.ID))
		return err
	}

	// Update order items to a refund-related status
	// Here we mark them as ReturnApproved directly since this is a full cancellation, not a user request.
	// This assumes that paid items are in "Paid" status.
	if _, err := l.orderRepo.UpdateItemsStatusByOrder(ctx, order.ID, constants.OrderItemStatusPaid.String(), constants.OrderItemStatusReturnApproved.String(), operator); err != nil {
		return fmt.Errorf("failed to update order items to return_approved: %w", err)
	}

	// Update order status to PendingRefund
	newStatus := constants.OrderStatusPendingRefund.String()
	if err := l.orderRepo.UpdateOrder(ctx, order.ID, repository.WithStatus(newStatus), repository.WithUpdatedBy(operator)); err != nil {
		return fmt.Errorf("failed to update order status to pending_refund: %w", err)
	}

	// Create audit log for the refund request
	// We can reuse the cancel log builder, or create a new one for clarity.
	auditLog := buildCancelOrderAuditLog(operator, order, reason)
	auditLog.Action = "REQUEST_REFUND"
	if err := l.auditLogRepo.Create(ctx, auditLog); err != nil {
		l.logger.Error("requestRefundForPaidOrder: Failed to create audit log", zap.Error(err))
	}

	return nil
}

var OrderLogicProviderSet = wire.NewSet(NewOrderLogic, wire.Bind(new(OrderLogic), new(*orderLogic)))

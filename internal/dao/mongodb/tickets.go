package mongodb

import (
	"context"
	"errors"
	"fmt"
	"partivo_tickets/internal/constants"
	"partivo_tickets/internal/dao/fields"
	"partivo_tickets/internal/dao/repository"
	"partivo_tickets/internal/dto"
	"partivo_tickets/internal/models"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

func NewTicketsDAO(db *mongo.Database, logger *zap.Logger) *TicketsDAO {
	return &TicketsDAO{
		ticketTypesCollection: db.Collection(CollectionTicketTypes),
		ticketStockCollection: db.Collection(CollectionTicketStock),
		ticketsCollection:     db.Collection(CollectionTickets),
		logger:                logger.Named("TicketsDAO"),
	}
}

type TicketsDAO struct {
	ticketTypesCollection *mongo.Collection
	ticketStockCollection *mongo.Collection
	ticketsCollection     *mongo.Collection
	logger                *zap.Logger
}

func (d *TicketsDAO) CountTicketsByOrder(ctx context.Context, orderID primitive.ObjectID) (int64, error) {
	count, err := d.ticketsCollection.CountDocuments(ctx, bson.M{"order": orderID})
	if err != nil {
		d.logger.Error("CountTicketsByOrder failed", zap.Error(err), zap.Stringer("orderID", orderID))
		return 0, err
	}
	return count, nil
}

func (d *TicketsDAO) CreateManyTickets(ctx context.Context, tickets []interface{}) error {
	if len(tickets) == 0 {
		return nil
	}
	_, err := d.ticketsCollection.InsertMany(ctx, tickets)
	if err != nil {
		d.logger.Error("CreateManyTickets: InsertMany failed", zap.Error(err))
		return err
	}
	return nil
}

func (d *TicketsDAO) AddTicketType(ctx context.Context, t *models.TicketType) (nid primitive.ObjectID, err error) {
	res, err := d.ticketTypesCollection.InsertOne(ctx, t)
	if err != nil {
		d.logger.Error("AddTicketType failed", zap.Error(err), zap.Any("ticket_type", t))
		return
	}
	nid = res.InsertedID.(primitive.ObjectID)
	return
}

func (d *TicketsDAO) AddTicketStock(ctx context.Context, t *models.TicketStock) (err error) {
	_, err = d.ticketStockCollection.InsertOne(ctx, t)
	if err != nil {
		d.logger.Error("AddTicketStock failed", zap.Error(err), zap.Any("ticket", t))
	}
	return
}

func (d *TicketsDAO) GetTicketTypesWithStockByEvent(ctx context.Context, eid primitive.ObjectID, opts ...repository.GetTicketsOption) (res []dto.TicketTypeWithStock, err error) {
	topts := &repository.GetTicketsOptions{}

	for _, opt := range opts {
		opt(topts)
	}

	matchStage := bson.M{fields.FieldTicketTypeEvent: eid}

	var orConditions []bson.M

	if topts.Display != nil {
		orConditions = append(orConditions, bson.M{fields.FieldTicketTypeDisplay: *topts.Display})
	}

	if topts.IsShowing != nil {
		now := time.Now()
		if *topts.IsShowing {
			isShowingCondition := bson.M{
				fields.FieldTicketTypeVisible:     true,
				fields.FieldTicketTypeStartShowAt: bson.M{"$lte": now},
				fields.FieldTicketTypeStopShowAt:  bson.M{"$gte": now},
			}
			orConditions = append(orConditions, isShowingCondition)
		} else {
			isNotShowingCondition := bson.M{
				"$or": []bson.M{
					{fields.FieldTicketTypeVisible: false},
					{fields.FieldTicketTypeStartShowAt: bson.M{"gt": now}},
					{fields.FieldTicketTypeStopShowAt: bson.M{"lt": now}},
				},
			}
			orConditions = append(orConditions, isNotShowingCondition)
		}
	}

	if len(orConditions) > 0 {
		matchStage["$or"] = orConditions
	}

	// 使用 Aggregation Pipeline 來關聯 ticket_type 和 stock
	pipeline := mongo.Pipeline{
		// 第一階段：篩選符合條件的 ticket_type
		{{"$match", matchStage}},
		// 第二階段：從 tickets collection 查找關聯文件
		{{"$lookup", bson.M{
			"from":         CollectionTicketStock,
			"localField":   fields.FieldObjectId,        // ticket_types collection 的關聯鍵
			"foreignField": fields.FieldTicketStockType, // tickets collection 的關聯鍵
			"as":           "tickets",                   // 關聯結果存入的欄位名稱，需與 DTO struct 的欄位名對應
		}}},
		// 第三階段：排序
		{{"$sort", bson.M{fields.FieldPick: 1}}},
	}

	res = make([]dto.TicketTypeWithStock, 0)
	cursor, err := d.ticketTypesCollection.Aggregate(ctx, pipeline)
	if err != nil {
		d.logger.Error("GetTicketTypesWithStockByEvent: Aggregate failed", zap.Error(err), zap.Stringer("eid", eid), zap.Any("opts", opts))
		return
	}

	if err = cursor.All(ctx, &res); err != nil {
		d.logger.Error("GetTicketTypesWithStockByEvent: cursor.All failed", zap.Error(err), zap.Stringer("eid", eid), zap.Any("opts", opts))
	}
	return
}

func (d *TicketsDAO) DeleteTicketTypeByID(ctx context.Context, tcid primitive.ObjectID) (res *models.TicketType, err error) {
	res = &models.TicketType{}
	err = d.ticketTypesCollection.FindOneAndDelete(ctx, bson.M{fields.FieldObjectId: tcid}).Decode(res)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		d.logger.Error("DeleteTicketTypeByID failed", zap.Error(err), zap.Stringer("ticketID", tcid))
		return nil, err
	}
	return
}

func (d *TicketsDAO) UpdateTicketType(ctx context.Context, t *models.TicketType) (err error) {
	updateat := t.UpdatedAt
	updateuser := t.UpdatedBy
	update := bson.M{
		"$set": bson.M{
			fields.FieldTicketTypeName:        t.Name,
			fields.FieldTicketTypePrice:       t.Price,
			fields.FieldTicketTypeDesc:        t.Desc,
			fields.FieldTicketTypeVisible:     t.Visible,
			fields.FieldTicketTypeStartShowAt: t.StartShowAt,
			fields.FieldTicketTypeStopShowAt:  t.StopShowAt,
			fields.FieldUpdatedAt:             *updateat,
			fields.FieldUpdatedBy:             *updateuser,
			fields.FieldTicketTypeMinPerOrder: t.MinQuantityPerOrder,
			fields.FieldTicketTypeMaxPerOrder: t.MaxQuantityPerOrder,
			fields.FieldTicketTypeDueDuration: t.DueDuration,
			fields.FieldTicketTypeEnable:      t.Enable,
			fields.FieldTicketTypeSaleSetting: t.SaleSetting,
		},
	}

	res, err := d.ticketTypesCollection.UpdateOne(ctx, bson.M{fields.FieldObjectId: t.ID}, update)
	if err != nil {
		d.logger.Error("UpdateTicketType failed", zap.Error(err), zap.Any("ticket", t))
	}
	if res.MatchedCount == 0 {
		err = mongo.ErrNoDocuments
	}
	return
}
func (d *TicketsDAO) UpdateTicketTypesOrder(ctx context.Context, eid primitive.ObjectID, ids []primitive.ObjectID, oper *models.User) (err error) {
	if len(ids) == 0 {
		return nil
	}

	var writes []mongo.WriteModel
	now := time.Now()
	for i, id := range ids {
		filter := bson.M{
			fields.FieldTicketTypeEvent: eid,
			fields.FieldObjectId:        id,
		}
		update := bson.M{
			"$set": bson.M{
				fields.FieldPick:      i,
				fields.FieldUpdatedBy: *oper,
				fields.FieldUpdatedAt: now,
			},
		}
		model := mongo.NewUpdateOneModel().SetFilter(filter).SetUpdate(update)
		writes = append(writes, model)
	}

	bulkWriteOptions := options.BulkWrite().SetOrdered(false)
	_, err = d.ticketTypesCollection.BulkWrite(ctx, writes, bulkWriteOptions)
	if err != nil {
		d.logger.Error("UpdateTicketTypesOrder: BulkWrite failed", zap.Error(err), zap.Stringer("eid", eid), zap.Any("ids", ids))
	}
	return
}

func (d *TicketsDAO) GetTicketTypeByID(ctx context.Context, tid primitive.ObjectID) (*models.TicketType, error) {
	filter := bson.M{
		fields.FieldObjectId: tid,
	}
	res := models.TicketType{}
	err := d.ticketTypesCollection.FindOne(ctx, filter).Decode(&res)
	if err != nil {
		if !errors.Is(err, mongo.ErrNoDocuments) {
			d.logger.Error("GetTicketTypeByID failed", zap.Error(err), zap.Stringer("ticketID", tid))
		}
		return nil, err
	}

	return &res, nil
}

func (d *TicketsDAO) GetValidTicketsByIDs(ctx context.Context, ids []primitive.ObjectID) ([]dto.TicketStockWithType, error) {
	tempColumn := "type"

	pipeline := mongo.Pipeline{
		// Match tickets with the given IDs and sale times
		{{"$match", bson.M{
			fields.FieldObjectId:               bson.M{"$in": ids},
			fields.FieldTicketStockSaleStartAt: bson.M{"$lte": time.Now()},
			fields.FieldTicketStockSaleEndAt:   bson.M{"$gte": time.Now()},
		}}},
		// Join with ticket_types
		{{"$lookup", bson.M{
			"from":         CollectionTicketTypes,
			"localField":   fields.FieldTicketStockType,
			"foreignField": fields.FieldObjectId,
			"as":           tempColumn,
		}}},
		// Deconstruct the array from the lookup
		{{"$unwind", fmt.Sprintf("$%s", tempColumn)}},
	}

	cursor, err := d.ticketStockCollection.Aggregate(ctx, pipeline)
	if err != nil {
		d.logger.Error("GetValidTicketsByIDs: Aggregate failed", zap.Error(err), zap.Any("ids", ids))
		return nil, err
	}
	defer cursor.Close(ctx)

	var tickets []dto.TicketStockWithType
	if err = cursor.All(ctx, &tickets); err != nil {
		d.logger.Error("GetValidTicketsByIDs: cursor.All failed", zap.Error(err), zap.Any("ids", ids))
		return nil, err
	}

	return tickets, nil
}

func (d *TicketsDAO) ReleaseTickets(ctx context.Context, items []*models.OrderItem) error {
	if len(items) == 0 {
		return nil
	}

	var writes []mongo.WriteModel
	for _, item := range items {
		filter := bson.M{fields.FieldObjectId: item.TicketStock}
		update := bson.M{
			"$inc": bson.M{fields.FieldQuantity: item.Quantity},
			"$set": bson.M{fields.FieldUpdatedAt: time.Now()},
		}
		model := mongo.NewUpdateOneModel().SetFilter(filter).SetUpdate(update)
		writes = append(writes, model)
	}

	bulkWriteOptions := options.BulkWrite().SetOrdered(false)
	_, err := d.ticketStockCollection.BulkWrite(ctx, writes, bulkWriteOptions)
	if err != nil {
		d.logger.Error("ReleaseTickets: BulkWrite failed", zap.Error(err), zap.Any("items", items))
	}
	return err
}

func (d *TicketsDAO) ReserveTickets(ctx context.Context, items []*models.OrderItem, user *models.User) error {
	if len(items) == 0 {
		return nil
	}

	var writes []mongo.WriteModel
	for _, item := range items {
		filter := bson.M{
			fields.FieldObjectId: item.TicketStock,
			fields.FieldQuantity: bson.M{"$gte": item.Quantity},
		}
		update := bson.M{
			"$inc": bson.M{fields.FieldQuantity: -item.Quantity},
			"$set": bson.M{
				fields.FieldUpdatedAt: time.Now(),
				fields.FieldUpdatedBy: *user,
			},
		}
		model := mongo.NewUpdateOneModel().SetFilter(filter).SetUpdate(update)
		writes = append(writes, model)
	}

	bulkWriteOptions := options.BulkWrite().SetOrdered(true)
	res, err := d.ticketStockCollection.BulkWrite(ctx, writes, bulkWriteOptions)
	if err != nil {
		d.logger.Error("ReserveTickets: BulkWrite failed", zap.Error(err), zap.Any("items", items))
		// Check if the error is a bulk write exception to determine if it was an inventory issue
		var e mongo.BulkWriteException
		if errors.As(err, &e) {
			// 112 is the error code for WriteConflict, which can happen in transactions.
			// A more specific check might be needed if other errors are common.
			// For now, we assume any bulk write error under a transaction could be a stock issue.
			return ErrInsufficientStock
		}
		return err
	}

	// If the number of modified documents does not match the number of items, it means
	// at least one ticket did not have enough stock.
	if res.ModifiedCount != int64(len(items)) {
		d.logger.Warn("ReserveTickets: Insufficient stock detected",
			zap.Int64("requested_items", int64(len(items))),
			zap.Int64("modified_tickets", res.ModifiedCount))
		return ErrInsufficientStock
	}

	return nil
}

func (d *TicketsDAO) GetTicketStockWithTypeBySession(ctx context.Context, sessionID primitive.ObjectID) (res []dto.TicketStockWithType, err error) {
	//// 建立 Aggregation Pipeline
	now := time.Now().UTC()
	tempColumn := "type"
	startShow := fmt.Sprintf("%s.%s", tempColumn, fields.FieldTicketTypeStartShowAt)
	stopShow := fmt.Sprintf("%s.%s", tempColumn, fields.FieldTicketTypeStopShowAt)
	alwaysDisplay := fmt.Sprintf("%s.%s", tempColumn, fields.FieldTicketTypeDisplay)
	visible := fmt.Sprintf("%s.%s", tempColumn, fields.FieldTicketTypeVisible)

	pipeline := mongo.Pipeline{
		// 第一階段：篩選出符合特定 session 的所有票券
		{{"$match", bson.M{fields.FieldTicketStockSession: sessionID}}},
		// 第二階段：透過 config 欄位，從 ticket_configs collection 查找對應的票券設定
		{{"$lookup", bson.M{
			"from":         CollectionTicketTypes,       // 要關聯的 collection
			"localField":   fields.FieldTicketStockType, // tickets collection 的關聯鍵
			"foreignField": "_id",                       // ticket_configs collection 的關聯鍵
			"as":           tempColumn,                  // 關聯結果存入的欄位名稱
		}}},
		// 第三階段：因為 $lookup 回傳的是一個陣列，但我們預期只會有一筆對應的 ticket_config，
		// 所以使用 $unwind 將陣列展開，方便後續處理
		{{"$unwind", fmt.Sprintf("$%s", tempColumn)}},
		// 第四階段：篩選出在售票時間內且可見的票券
		{{"$match", bson.M{
			"$or": []bson.M{
				{alwaysDisplay: true},
				{
					visible:   true,
					startShow: bson.M{"$lte": now},
					stopShow:  bson.M{"$gte": now},
				},
			},
		}}},
		// 第五階段：排序
		{{"$sort", bson.M{
			fmt.Sprintf("%s.%s", tempColumn, fields.FieldPick): 1, // 主要排序欄位
			fields.FieldObjectId:                                 1, // 次要排序欄位，確保順序穩定
		}}},
	}

	// 執行 Aggregation 查詢
	cursor, err := d.ticketStockCollection.Aggregate(ctx, pipeline)
	if err != nil {
		d.logger.Error("GetTicketStockWithTypeBySession: Aggregate failed", zap.Error(err), zap.Stringer("sessionID", sessionID))
		return nil, err
	}
	defer cursor.Close(ctx)

	// 將查詢結果解碼到 res 中
	res = make([]dto.TicketStockWithType, 0)
	if err = cursor.All(ctx, &res); err != nil {
		d.logger.Error("GetTicketStockWithTypeBySession: cursor.All failed", zap.Error(err), zap.Stringer("sessionID", sessionID))
		return nil, err
	}

	return res, nil
}

func (d *TicketsDAO) DeleteTicketStockByType(ctx context.Context, tcid primitive.ObjectID) (err error) {
	_, err = d.ticketStockCollection.DeleteMany(ctx, bson.M{"config": tcid})
	if err != nil {
		d.logger.Error("DeleteTicketStockByType failed", zap.Error(err), zap.Stringer("tcid", tcid))
	}
	return
}

// GetTicketByOrderItemID finds a single ticket by its associated order_item.id.
func (d *TicketsDAO) GetTicketByOrderItemID(ctx context.Context, orderItemID primitive.ObjectID) (*models.Ticket, error) {
	var ticket models.Ticket
	filter := bson.M{"order_item.id": orderItemID}
	err := d.ticketsCollection.FindOne(ctx, filter).Decode(&ticket)
	if err != nil {
		if !errors.Is(err, mongo.ErrNoDocuments) {
			d.logger.Error("GetTicketByOrderItemID: FindOne failed", zap.Error(err), zap.Stringer("orderItemID", orderItemID))
		}
		return nil, err
	}
	return &ticket, nil
}

// UpdateTicketStatus updates the status of a single ticket.
func (d *TicketsDAO) UpdateTicketStatus(ctx context.Context, ticketID primitive.ObjectID, status string) error {
	filter := bson.M{fields.FieldObjectId: ticketID}
	update := bson.M{"$set": bson.M{fields.FieldStatus: status, fields.FieldUpdatedAt: time.Now()}}

	result, err := d.ticketsCollection.UpdateOne(ctx, filter, update)
	if err != nil {
		d.logger.Error("UpdateTicketStatus: UpdateOne failed", zap.Error(err), zap.Stringer("ticketID", ticketID))
		return err
	}

	if result.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}

	return nil
}

// GetTicketByID retrieves a single ticket by its ID.
func (d *TicketsDAO) GetTicketByID(ctx context.Context, ticketID primitive.ObjectID) (*models.Ticket, error) {
	var ticket models.Ticket
	filter := bson.M{fields.FieldObjectId: ticketID}
	err := d.ticketsCollection.FindOne(ctx, filter).Decode(&ticket)
	if err != nil {
		if !errors.Is(err, mongo.ErrNoDocuments) {
			d.logger.Error("GetTicketByID: FindOne failed", zap.Error(err), zap.Stringer("ticketID", ticketID))
		}
		return nil, err
	}
	return &ticket, nil
}

// MarkTicketAsUsed atomically updates a ticket's status to "Used" only if it is currently "Valid".
func (d *TicketsDAO) MarkTicketAsUsed(ctx context.Context, ticketID primitive.ObjectID, usedAt time.Time, operator *models.User) error {
	filter := bson.M{
		fields.FieldObjectId: ticketID,
		fields.FieldStatus:   constants.TicketStatusValid.String(),
	}
	update := bson.M{"$set": bson.M{
		fields.FieldStatus:            constants.TicketStatusUsed.String(),
		fields.FieldTicketUsedAt:      usedAt,
		fields.FieldUpdatedAt:         time.Now(),
		fields.FieldTicketCheckedInBy: operator,
	}}

	result, err := d.ticketsCollection.UpdateOne(ctx, filter, update)
	if err != nil {
		d.logger.Error("MarkTicketAsUsed: UpdateOne failed", zap.Error(err), zap.Stringer("ticketID", ticketID))
		return err
	}

	if result.MatchedCount == 0 {
		// This is the key check for the atomic operation. If no document matched,
		// it means the ticket was not in the 'Valid' state, so we return ErrNoDocuments
		// to signal the race condition to the logic layer.
		return mongo.ErrNoDocuments
	}

	return nil
}

// ExpireTickets updates the status of all valid tickets that have expired.
func (d *TicketsDAO) ExpireTickets(ctx context.Context, now time.Time) (int64, error) {
	filter := bson.M{
		fields.FieldStatus:           constants.TicketStatusValid.String(),
		fields.FieldTicketValidUntil: bson.M{"$lt": now},
	}
	update := bson.M{"$set": bson.M{
		fields.FieldStatus:    constants.TicketExpired.String(),
		fields.FieldUpdatedAt: now,
	}}

	result, err := d.ticketsCollection.UpdateMany(ctx, filter, update)
	if err != nil {
		d.logger.Error("ExpireTickets: UpdateMany failed", zap.Error(err))
		return 0, err
	}

	return result.ModifiedCount, nil
}

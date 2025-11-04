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

func NewOrdersDAO(db *mongo.Database, logger *zap.Logger) *OrdersDAO {
	return &OrdersDAO{
		ordersCollection:     db.Collection(CollectionOrders),
		orderItemsCollection: db.Collection(CollectionOrderItems),
		paymentsCollection:   db.Collection(CollectionPayments),
		logger:               logger.Named("OrdersDAO"),
	}
}

type OrdersDAO struct {
	ordersCollection     *mongo.Collection
	orderItemsCollection *mongo.Collection
	paymentsCollection   *mongo.Collection
	logger               *zap.Logger
}

// CreateOrder creates a new order document in the database.
func (d *OrdersDAO) CreateOrder(ctx context.Context, order *models.Order) (primitive.ObjectID, error) {
	_, err := d.ordersCollection.InsertOne(ctx, order)
	if err != nil {
		d.logger.Error("CreateOrder: InsertOne failed", zap.Error(err), zap.Any("order", order))
		return primitive.NilObjectID, err
	}
	return order.ID, nil
}

func (d *OrdersDAO) CreateOrderItems(ctx context.Context, orderID primitive.ObjectID, items []*models.OrderItem) error {
	if len(items) == 0 {
		return nil // Nothing to insert
	}

	docs := make([]interface{}, len(items))
	for i, item := range items {
		item.Order = orderID // Set the OrderID for each item
		docs[i] = item
	}

	_, err := d.orderItemsCollection.InsertMany(ctx, docs)
	if err != nil {
		d.logger.Error("CreateOrderItems: InsertMany failed", zap.Error(err), zap.Stringer("orderID", orderID))
		return err
	}
	return nil
}

//func (d *OrdersDAO) GetPaymentByID(ctx context.Context, id primitive.ObjectID) (*models.Payment, error) {
//	res := models.Payment{}
//	err := d.paymentsCollection.FindOne(ctx, bson.M{fields.FieldObjectId: id, fields.FieldPaymentEnable: true}).Decode(&res)
//	if err != nil {
//		d.logger.Error("GetPaymentByID failed", zap.Error(err), zap.Stringer("paymentID", id))
//		return nil, err
//	}
//
//	return &res, nil
//}

func (d *OrdersDAO) IsEventHasOpenOrders(ctx context.Context, eventID primitive.ObjectID) (bool, error) {
	filter := bson.M{
		fields.FieldOrdersEvent: eventID,
		fields.FieldStatus: bson.M{
			"$nin": []string{constants.OrderStatusCompleted.String(), constants.OrderStatusReturned.String(), constants.OrderStatusCanceled.String(), constants.OrderStatusPartiallyReturned.String()},
		},
	}
	res := d.ordersCollection.FindOne(ctx, filter)
	if errors.Is(res.Err(), mongo.ErrNoDocuments) {
		return false, nil
	}
	if res.Err() != nil {
		d.logger.Error("IsEventHasOpenOrders: FindOne failed", zap.Error(res.Err()), zap.Stringer("eventID", eventID))
		return false, res.Err()
	}

	return true, nil
}

func (d *OrdersDAO) GetOrdersByEvent(ctx context.Context, params *repository.GetOrdersByEventParams) ([]*dto.OrderWithItems, int64, error) {
	filter := bson.M{fields.FieldOrdersEvent: params.EventID}
	return d.getPaginatedOrdersWithItems(ctx, filter, params.Limit, params.Offset)
}

func (d *OrdersDAO) GetOrdersBySession(ctx context.Context, params *repository.GetOrdersBySessionParams) ([]*dto.OrderWithItems, int64, error) {
	filter := bson.M{fields.FieldOrdersSession: params.SessionID}
	return d.getPaginatedOrdersWithItems(ctx, filter, params.Limit, params.Offset)
}

func (d *OrdersDAO) getPaginatedOrders(ctx context.Context, filter bson.M, limit, offset int) ([]*models.Order, int64, error) {
	// First, get the total count of documents that match the filter.
	total, err := d.ordersCollection.CountDocuments(ctx, filter)
	if err != nil {
		d.logger.Error("getPaginatedOrders: CountDocuments failed", zap.Error(err), zap.Any("filter", filter))
		return nil, 0, err
	}

	if total == 0 {
		return []*models.Order{}, 0, nil
	}

	// Then, find the documents for the current page.
	opts := options.Find().
		SetSort(bson.D{{fields.FieldCreatedAt, -1}}).
		SetSkip(int64(offset)).
		SetLimit(int64(limit))

	cursor, err := d.ordersCollection.Find(ctx, filter, opts)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return []*models.Order{}, total, nil
		}
		d.logger.Error("getPaginatedOrders: Find failed", zap.Error(err), zap.Any("filter", filter))
		return nil, 0, err
	}

	var orders []*models.Order
	if err = cursor.All(ctx, &orders); err != nil {
		d.logger.Error("getPaginatedOrders: cursor.All failed", zap.Error(err), zap.Any("filter", filter))
		return nil, 0, err
	}

	return orders, total, nil
}

func (d *OrdersDAO) getOrders(ctx context.Context, filter interface{}, opts ...*options.FindOptions) ([]*models.Order, error) {
	res := make([]*models.Order, 0)
	cursor, err := d.ordersCollection.Find(ctx, filter, opts...)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		d.logger.Error("getOrders: Find failed", zap.Error(err), zap.Any("filter", filter), zap.Any("opts", opts))
		return nil, err
	}

	if err = cursor.All(ctx, &res); err != nil {
		d.logger.Error("getOrders: cursor.All failed", zap.Error(err), zap.Any("filter", filter), zap.Any("opts", opts))
		return nil, err
	}

	return res, nil
}

func (d *OrdersDAO) GetOrderByID(ctx context.Context, id primitive.ObjectID) (*models.Order, error) {
	var res models.Order
	err := d.ordersCollection.FindOne(ctx, bson.M{fields.FieldObjectId: id}).Decode(&res)
	if err != nil {
		if !errors.Is(err, mongo.ErrNoDocuments) {
			d.logger.Error("GetOrderByID: FindOne failed", zap.Error(err), zap.Stringer("orderID", id))
		}
		return nil, err // Return nil model on any error
	}
	return &res, nil // Return model and nil error on success
}

func (d *OrdersDAO) GetOrderItemByID(ctx context.Context, itemID primitive.ObjectID) (*models.OrderItem, error) {
	var item models.OrderItem
	filter := bson.M{fields.FieldObjectId: itemID}
	err := d.orderItemsCollection.FindOne(ctx, filter).Decode(&item)
	if err != nil {
		if !errors.Is(err, mongo.ErrNoDocuments) {
			d.logger.Error("GetOrderItemByID: FindOne failed", zap.Error(err), zap.Stringer("itemID", itemID))
		}
		return nil, err
	}
	return &item, nil
}

func (d *OrdersDAO) GetOrderItemsByOrderID(ctx context.Context, orderID primitive.ObjectID) ([]*models.OrderItem, error) {
	filter := bson.M{fields.FieldOrderItemOrder: orderID}
	cursor, err := d.orderItemsCollection.Find(ctx, filter)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return []*models.OrderItem{}, nil // Return empty slice if no items found
		}
		d.logger.Error("GetOrderItemsByOrderID: Find failed", zap.Error(err), zap.Stringer("orderID", orderID))
		return nil, err
	}

	var items []*models.OrderItem
	if err = cursor.All(ctx, &items); err != nil {
		d.logger.Error("GetOrderItemsByOrderID: cursor.All failed", zap.Error(err), zap.Stringer("orderID", orderID))
		return nil, err
	}

	return items, nil
}

func (d *OrdersDAO) UpdateOrderItemStatus(ctx context.Context, params *repository.UpdateOrderItemStatusParams) error {
	filter := bson.M{fields.FieldObjectId: params.ItemID}
	update := bson.M{
		"$set": bson.M{
			fields.FieldStatus:              params.Status,
			fields.FieldUpdatedAt:           time.Now(),
			fields.FieldOrderItemReturnInfo: params.ReturnInfo,
		},
	}

	result, err := d.orderItemsCollection.UpdateOne(ctx, filter, update)
	if err != nil {
		d.logger.Error("UpdateOrderItemStatus: UpdateOne failed", zap.Error(err), zap.Stringer("itemID", params.ItemID))
		return err
	}

	if result.MatchedCount == 0 {
		return mongo.ErrNoDocuments // Use a standard error to indicate not found
	}

	return nil
}

func (d *OrdersDAO) UpdateOrderStatus(ctx context.Context, params *repository.UpdateOrderStatusParams) (*models.Order, error) {
	filter := bson.M{fields.FieldObjectId: params.OrderID}
	update := bson.M{
		"$set": bson.M{
			fields.FieldStatus:    params.Status,
			fields.FieldUpdatedAt: time.Now(),
			fields.FieldUpdatedBy: params.User,
		},
	}
	opts := options.FindOneAndUpdate().SetReturnDocument(options.Before)
	res := d.ordersCollection.FindOneAndUpdate(ctx, filter, update, opts)
	if res.Err() != nil {
		if !errors.Is(res.Err(), mongo.ErrNoDocuments) {
			d.logger.Error("UpdateOrderStatus failed", zap.Error(res.Err()), zap.Any("params", params))
		}
		return nil, res.Err()
	}
	var order models.Order
	if err := res.Decode(&order); err != nil {
		d.logger.Error("UpdateOrderStatus: Decode failed", zap.Error(err), zap.Any("params", params))
		return nil, err
	}
	return &order, nil
}

func (d *OrdersDAO) GetOrdersByUser(ctx context.Context, params *repository.GetOrdersByUserParams) ([]*dto.OrderWithItems, error) {
	pipeline := mongo.Pipeline{}

	// Stage 1: Match orders for the specific user
	matchUserStage := bson.D{
		{"$match", bson.D{
			{fmt.Sprintf("%s.%s", fields.FieldOrderUser, fields.FieldOrderUserUserID), params.UserID},
		}},
	}
	pipeline = append(pipeline, matchUserStage)

	// Stage 2: Handle cursor-based pagination (Robust approach)
	if !params.CursorID.IsZero() && !params.CursorCreatedAt.IsZero() {
		cursorMatchStage := bson.D{
			{"$match", bson.D{
				{"$or", []bson.D{
					{{"created_at", bson.D{{"$lt", params.CursorCreatedAt}}}},
					{{"created_at", params.CursorCreatedAt}, {"_id", bson.D{{"$lt", params.CursorID}}}},
				}},
			}},
		}
		pipeline = append(pipeline, cursorMatchStage)
	}

	// Stage 3: Sort for consistent pagination
	sortStage := bson.D{{"$sort", bson.D{{"created_at", -1}, {"_id", -1}}}}
	pipeline = append(pipeline, sortStage)

	// Stage 4: Limit the results
	limitStage := bson.D{{"$limit", params.Limit}}
	pipeline = append(pipeline, limitStage)

	// Stage 5: Lookup order items from the 'order_items' collection
	lookupStage := bson.D{
		{"$lookup", bson.D{
			{"from", CollectionOrderItems},
			{"localField", fields.FieldObjectId},
			{"foreignField", fields.FieldOrderItemOrder},
			{"as", "items"},
		}},
	}
	pipeline = append(pipeline, lookupStage)

	cursor, err := d.ordersCollection.Aggregate(ctx, pipeline)
	if err != nil {
		d.logger.Error("GetOrdersByUser: Aggregate failed", zap.Error(err))
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []*dto.OrderWithItems
	if err = cursor.All(ctx, &results); err != nil {
		d.logger.Error("GetOrdersByUser: cursor.All failed", zap.Error(err))
		return nil, err
	}

	return results, nil
}

func (d *OrdersDAO) GetOrderWithItemsByID(ctx context.Context, orderID primitive.ObjectID) (*dto.OrderWithItems, error) {
	pipeline := mongo.Pipeline{}

	// Stage 1: Match the specific order
	matchStage := bson.D{
		{"$match", bson.D{
			{fields.FieldObjectId, orderID},
		}},
	}
	pipeline = append(pipeline, matchStage)

	// Stage 2: Lookup order items
	lookupStage := bson.D{
		{"$lookup", bson.D{
			{"from", CollectionOrderItems},
			{"localField", fields.FieldObjectId},
			{"foreignField", fields.FieldOrderItemOrder},
			{"as", "items"},
		}},
	}
	pipeline = append(pipeline, lookupStage)

	// Stage 3: Limit to one result (since we're matching by id)
	limitStage := bson.D{{"$limit", 1}}
	pipeline = append(pipeline, limitStage)

	cursor, err := d.ordersCollection.Aggregate(ctx, pipeline)
	if err != nil {
		d.logger.Error("GetOrderWithItemsByID: Aggregate failed", zap.Error(err), zap.Stringer("orderID", orderID))
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []*dto.OrderWithItems
	if err = cursor.All(ctx, &results); err != nil {
		d.logger.Error("GetOrderWithItemsByID: cursor.All failed", zap.Error(err), zap.Stringer("orderID", orderID))
		return nil, err
	}

	if len(results) == 0 {
		return nil, mongo.ErrNoDocuments
	}

	return results[0], nil
}

// getOrderItemsMapByOrderIDs fetches order items for a given list of order IDs and returns them as a map
// where the key is the order id and the value is a slice of its order items.
func (d *OrdersDAO) getOrderItemsMapByOrderIDs(ctx context.Context, orderIDs []primitive.ObjectID) (map[primitive.ObjectID][]*models.OrderItem, error) {
	if len(orderIDs) == 0 {
		return map[primitive.ObjectID][]*models.OrderItem{}, nil
	}

	itemFilter := bson.M{"order": bson.M{"$in": orderIDs}}
	cursor, err := d.orderItemsCollection.Find(ctx, itemFilter)
	if err != nil {
		// No need to log here as the caller will handle it.
		return nil, err
	}

	var items []*models.OrderItem
	if err = cursor.All(ctx, &items); err != nil {
		return nil, err
	}

	// Group items by OrderID.
	itemsByOrderID := make(map[primitive.ObjectID][]*models.OrderItem, len(orderIDs))
	for _, item := range items {
		itemsByOrderID[item.Order] = append(itemsByOrderID[item.Order], item)
	}

	return itemsByOrderID, nil
}

func (d *OrdersDAO) getPaginatedOrdersWithItems(ctx context.Context, filter bson.M, limit, offset int) ([]*dto.OrderWithItems, int64, error) {
	// Step 1: Get the total count of documents that match the filter, respecting the concern about $facet.
	total, err := d.ordersCollection.CountDocuments(ctx, filter)
	if err != nil {
		d.logger.Error("getPaginatedOrdersWithItems: CountDocuments failed", zap.Error(err), zap.Any("filter", filter))
		return nil, 0, err
	}

	if total == 0 {
		return []*dto.OrderWithItems{}, 0, nil
	}

	// Step 2: Build the aggregation pipeline to fetch the paginated data with items.
	pipeline := mongo.Pipeline{}

	// Stage 1: Match the initial documents
	matchStage := bson.D{{"$match", filter}}
	pipeline = append(pipeline, matchStage)

	// Stage 2: Sort the documents
	sortStage := bson.D{{"$sort", bson.D{{fields.FieldCreatedAt, -1}}}}
	pipeline = append(pipeline, sortStage)

	// Stage 3: Skip documents for pagination
	if offset > 0 {
		skipStage := bson.D{{"$skip", int64(offset)}}
		pipeline = append(pipeline, skipStage)
	}

	// Stage 4: Limit the number of results
	limitStage := bson.D{{"$limit", int64(limit)}}
	pipeline = append(pipeline, limitStage)

	// Stage 5: Lookup order items
	lookupStage := bson.D{
		{"$lookup", bson.D{
			{"from", CollectionOrderItems},
			{"localField", fields.FieldObjectId},
			{"foreignField", fields.FieldOrderItemOrder},
			{"as", "items"},
		}},
	}
	pipeline = append(pipeline, lookupStage)

	// Step 3: Execute the aggregation pipeline
	cursor, err := d.ordersCollection.Aggregate(ctx, pipeline)
	if err != nil {
		d.logger.Error("getPaginatedOrdersWithItems: Aggregate failed", zap.Error(err), zap.Any("filter", filter))
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	// Step 4: Decode the results directly into the DTO
	var results []*dto.OrderWithItems
	if err = cursor.All(ctx, &results); err != nil {
		d.logger.Error("getPaginatedOrdersWithItems: cursor.All failed", zap.Error(err), zap.Any("filter", filter))
		return nil, 0, err
	}

	return results, total, nil
}

func (d *OrdersDAO) UpdateOrder(ctx context.Context, orderID primitive.ObjectID, opts ...repository.UpdateOption) error {
	// Create a new UpdateOptions struct and apply all the functional updateOpts.
	updateOpts := repository.NewUpdateOptions()
	for _, opt := range opts {
		opt(updateOpts)
	}

	// Build the final MongoDB update document from the updateOpts.
	updateDoc := bson.M{}
	if len(updateOpts.SetFields) > 0 {
		updateDoc["$set"] = updateOpts.SetFields
	}
	if len(updateOpts.IncFields) > 0 {
		updateDoc["$inc"] = updateOpts.IncFields
	}

	// If no fields are being updated, we can return early.
	if len(updateDoc) == 0 {
		return nil
	}

	// Execute the update operation.
	filter := bson.M{fields.FieldObjectId: orderID}
	result, err := d.ordersCollection.UpdateOne(ctx, filter, updateDoc)
	if err != nil {
		d.logger.Error("UpdateOrder: UpdateOne failed", zap.Error(err), zap.Stringer("orderID", orderID))
		return err
	}

	if result.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}

	return nil
}

func (d *OrdersDAO) UpdateItemsStatusByOrder(ctx context.Context, orderID primitive.ObjectID, currentStatus string, newStatus string, operator *models.User) (int64, error) {
	filter := bson.M{
		fields.FieldOrderItemOrder: orderID,
		fields.FieldStatus:         currentStatus,
	}

	update := bson.M{
		"$set": bson.M{
			fields.FieldStatus:    newStatus,
			fields.FieldUpdatedAt: time.Now(),
			fields.FieldUpdatedBy: operator,
		},
	}

	result, err := d.orderItemsCollection.UpdateMany(ctx, filter, update)
	if err != nil {
		d.logger.Error("UpdateItemsStatusByOrder: UpdateMany failed",
			zap.Error(err),
			zap.Stringer("orderID", orderID),
			zap.String("currentStatus", currentStatus),
			zap.String("newStatus", newStatus),
		)
		return 0, err
	}

	return result.ModifiedCount, nil
}

func (d *OrdersDAO) GetOrderItemWithOrder(ctx context.Context, orderItemID primitive.ObjectID) (*dto.OrderItemWithOrder, error) {
	pipeline := mongo.Pipeline{
		// Stage 1: Match the specific order item
		{{"$match", bson.D{{fields.FieldObjectId, orderItemID}}}},

		// Stage 2: Lookup the parent order from the 'orders' collection
		{{"$lookup", bson.D{
			{"from", CollectionOrders},
			{"localField", fields.FieldOrderItemOrder},
			{"foreignField", fields.FieldObjectId},
			{"as", "order_info"},
		}}},

		// Stage 3: Deconstruct the order_info array. As we expect only one order, this is safe.
		// Use $unwind to make order_info a single embedded document instead of an array.
		{{"$unwind", "$order_info"}},
	}

	cursor, err := d.orderItemsCollection.Aggregate(ctx, pipeline)
	if err != nil {
		d.logger.Error("GetOrderItemWithOrder: Aggregate failed", zap.Error(err), zap.Stringer("orderItemID", orderItemID))
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []*dto.OrderItemWithOrder
	if err = cursor.All(ctx, &results); err != nil {
		d.logger.Error("GetOrderItemWithOrder: cursor.All failed", zap.Error(err), zap.Stringer("orderItemID", orderItemID))
		return nil, err
	}

	if len(results) == 0 {
		return nil, mongo.ErrNoDocuments
	}

	return results[0], nil
}

func (d *OrdersDAO) GetOrdersByEventAndMonth(ctx context.Context, eventID primitive.ObjectID, year int, month int) ([]*dto.OrderWithItems, error) {
	// 1. Define the time range for the given month
	startTime := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	endTime := startTime.AddDate(0, 1, 0)

	// 2. Build the aggregation pipeline
	pipeline := mongo.Pipeline{
		// Stage 1: Match orders by event and date range
		{{"$match", bson.D{
			{fields.FieldOrdersEvent, eventID},
			{fields.FieldCreatedAt, bson.D{
				{"$gte", startTime},
				{"$lt", endTime},
			}},
		}}},
		// Stage 2: Sort by creation time
		{{"$sort", bson.D{{fields.FieldCreatedAt, 1}}}},
		// Stage 3: Lookup order items
		{{"$lookup", bson.D{
			{"from", CollectionOrderItems},
			{"localField", fields.FieldObjectId},
			{"foreignField", fields.FieldOrderItemOrder},
			{"as", "items"},
		}}},
	}

	// 3. Execute the aggregation
	cursor, err := d.ordersCollection.Aggregate(ctx, pipeline)
	if err != nil {
		d.logger.Error("GetOrdersByEventAndMonth: Aggregate failed", zap.Error(err), zap.Stringer("eventID", eventID))
		return nil, err
	}
	defer cursor.Close(ctx)

	// 4. Decode the results
	var results []*dto.OrderWithItems
	if err = cursor.All(ctx, &results); err != nil {
		d.logger.Error("GetOrdersByEventAndMonth: cursor.All failed", zap.Error(err))
		return nil, err
	}

	return results, nil
}

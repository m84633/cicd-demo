package mongodb

import (
	"context"
	"errors"
	"partivo_tickets/internal/constants"
	"partivo_tickets/internal/dao/fields"
	"partivo_tickets/internal/dao/repository"
	"partivo_tickets/internal/models"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

func NewBillDAO(db *mongo.Database, logger *zap.Logger) *BillDAO {
	return &BillDAO{
		billsCollection: db.Collection(CollectionBills),
		logger:          logger.Named("BillDAO"),
	}
}

type BillDAO struct {
	billsCollection *mongo.Collection
	logger          *zap.Logger
}

func (d *BillDAO) CreateBill(ctx context.Context, bill *models.Bill) (primitive.ObjectID, error) {
	res, err := d.billsCollection.InsertOne(ctx, bill)
	if err != nil {
		d.logger.Error("CreateBill: InsertOne failed", zap.Error(err), zap.Any("bill", bill))
		return primitive.NilObjectID, err
	}
	return res.InsertedID.(primitive.ObjectID), nil
}

func (d *BillDAO) GetBillsByOrderID(ctx context.Context, orderID primitive.ObjectID) ([]*models.Bill, error) {
	var bills []*models.Bill
	findOptions := options.Find().SetSort(bson.D{{fields.FieldCreatedAt, 1}}) // 1 for ascending
	cursor, err := d.billsCollection.Find(ctx, bson.M{"order": orderID}, findOptions)
	if err != nil {
		d.logger.Error("GetBillsByOrderID: Find failed", zap.Error(err), zap.Stringer("orderID", orderID))
		return nil, err
	}
	if err := cursor.All(ctx, &bills); err != nil {
		d.logger.Error("GetBillsByOrderID: cursor.All failed", zap.Error(err), zap.Stringer("orderID", orderID))
		return nil, err
	}
	return bills, nil
}

// GetBillByID retrieves a single bill by its ID.
func (d *BillDAO) GetBillByID(ctx context.Context, id primitive.ObjectID) (*models.Bill, error) {
	var bill models.Bill
	err := d.billsCollection.FindOne(ctx, bson.M{"_id": id}).Decode(&bill)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		d.logger.Error("GetBillByID: FindOne failed", zap.Error(err), zap.Stringer("id", id))
		return nil, err
	}
	return &bill, nil
}

// UpdateBill updates a single bill using functional options.
func (d *BillDAO) UpdateBill(ctx context.Context, id primitive.ObjectID, opts ...repository.UpdateOption) error {
	// 1. Apply all the options to a new UpdateOptions struct.
	updateData := repository.NewUpdateOptions()
	for _, opt := range opts {
		opt(updateData)
	}

	// 2. Construct the MongoDB update document from the options.
	update := bson.M{}
	if len(updateData.SetFields) > 0 {
		// Always set the updated_at field when other fields are being set.
		updateData.SetFields[fields.FieldUpdatedAt] = time.Now()
		update["$set"] = updateData.SetFields
	}
	if len(updateData.IncFields) > 0 {
		update["$inc"] = updateData.IncFields
	}

	// 3. Check if there is anything to update.
	if len(update) == 0 {
		return nil // Nothing to do.
	}

	// 4. Execute the update operation.
	res, err := d.billsCollection.UpdateOne(ctx, bson.M{fields.FieldObjectId: id}, update)
	if err != nil {
		d.logger.Error("UpdateBill: UpdateOne failed", zap.Error(err), zap.Stringer("id", id))
		return err
	}
	if res.MatchedCount == 0 {
		return ErrNotFound
	}
	return nil
}

// GetRefundsByOrderItemID retrieves all refund bills associated with a specific order item.
func (d *BillDAO) GetRefundsByOrderItemID(ctx context.Context, orderItemID primitive.ObjectID) ([]*models.Bill, error) {
	var bills []*models.Bill
	filter := bson.M{
		"order_item_id": &orderItemID,
		"type":          constants.BillTypeRefund,
	}
	cursor, err := d.billsCollection.Find(ctx, filter)
	if err != nil {
		d.logger.Error("GetRefundsByOrderItemID: Find failed", zap.Error(err), zap.Stringer("orderItemID", orderItemID))
		return nil, err
	}
	if err := cursor.All(ctx, &bills); err != nil {
		d.logger.Error("GetRefundsByOrderItemID: cursor.All failed", zap.Error(err), zap.Stringer("orderItemID", orderItemID))
		return nil, err
	}
	return bills, nil
}

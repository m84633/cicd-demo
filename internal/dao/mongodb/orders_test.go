package mongodb

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	tcMongo "github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"

	"partivo_tickets/internal/dao/fields"
	"partivo_tickets/internal/dao/repository"
	"partivo_tickets/internal/models"
)

func TestOrdersDAO_CreateOrder(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	t.Run("insert succeeds returns order id", func(t *testing.T) {
		dao := setupOrdersDAOIntegration(t)

		testCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		order := buildOrder(primitive.NewObjectID())

		insertedID, err := dao.CreateOrder(testCtx, order)
		require.NoError(t, err)
		require.Equal(t, order.ID, insertedID)

		stored, err := dao.GetOrderByID(testCtx, insertedID)
		require.NoError(t, err)
		require.NotNil(t, stored)
		require.Equal(t, order.ID, stored.ID)
	})

	t.Run("duplicate id returns error", func(t *testing.T) {
		dao := setupOrdersDAOIntegration(t)

		testCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		orderID := primitive.NewObjectID()
		firstOrder := buildOrder(orderID)

		insertedID, err := dao.CreateOrder(testCtx, firstOrder)
		require.NoError(t, err)
		require.Equal(t, orderID, insertedID)

		duplicateOrder := buildOrder(orderID)
		_, err = dao.CreateOrder(testCtx, duplicateOrder)
		require.Error(t, err)
		var writeException mongo.WriteException
		require.True(t, errors.As(err, &writeException))
		require.True(t, mongo.IsDuplicateKeyError(err))
	})
}

func buildOrder(id primitive.ObjectID) *models.Order {
	now := time.Now().UTC()
	return &models.Order{
		ID:                 id,
		Event:              primitive.NewObjectID(),
		Session:            primitive.NewObjectID(),
		Serial:             1,
		Subtotal:           primitive.NewDecimal128(0, 0),
		Total:              primitive.NewDecimal128(0, 0),
		OriginalTotal:      primitive.NewDecimal128(0, 0),
		Status:             "created",
		CreatedAt:          now,
		UpdatedAt:          now,
		Paid:               primitive.NewDecimal128(0, 0),
		Returned:           primitive.NewDecimal128(0, 0),
		ItemsCount:         0,
		ReturnedItemsCount: 0,
		MerchantID:         primitive.NewObjectID(),
	}
}

func TestOrdersDAO_CreateOrderItems(t *testing.T) {
	t.Run("empty items returns nil", func(t *testing.T) {
		dao := &OrdersDAO{}
		err := dao.CreateOrderItems(context.Background(), primitive.NewObjectID(), nil)
		require.NoError(t, err)
	})

	t.Run("insert items stores order id", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping integration test in short mode")
		}

		dao := setupOrdersDAOIntegration(t)

		orderID := primitive.NewObjectID()
		items := []*models.OrderItem{
			{
				ID:          primitive.NewObjectID(),
				TicketStock: primitive.NewObjectID(),
				Session:     primitive.NewObjectID(),
				Name:        "item-1",
				Price:       primitive.NewDecimal128(0, 0),
				Quantity:    1,
				Status:      "created",
				CreatedAt:   time.Now().UTC(),
				UpdatedAt:   time.Now().UTC(),
			},
			{
				ID:          primitive.NewObjectID(),
				TicketStock: primitive.NewObjectID(),
				Session:     primitive.NewObjectID(),
				Name:        "item-2",
				Price:       primitive.NewDecimal128(0, 0),
				Quantity:    2,
				Status:      "created",
				CreatedAt:   time.Now().UTC(),
				UpdatedAt:   time.Now().UTC(),
			},
		}

		err := dao.CreateOrderItems(context.Background(), orderID, items)
		require.NoError(t, err)

		cursor, err := dao.orderItemsCollection.Find(context.Background(), bson.M{fields.FieldOrderItemOrder: orderID})
		require.NoError(t, err)
		var stored []*models.OrderItem
		require.NoError(t, cursor.All(context.Background(), &stored))
		require.Len(t, stored, len(items))
		for _, item := range stored {
			require.Equal(t, orderID, item.Order)
		}
	})

	t.Run("insert error is returned", func(t *testing.T) {
		mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
		mt.Run("InsertMany failure", func(mt *mtest.T) {
			dao := &OrdersDAO{
				ordersCollection:     mt.Coll,
				orderItemsCollection: mt.Coll,
				paymentsCollection:   mt.Coll,
				logger:               zap.NewNop(),
			}

			mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
				Code:    11000,
				Message: "duplicate key",
				Name:    "DuplicateKey",
			}))

			err := dao.CreateOrderItems(context.Background(), primitive.NewObjectID(), []*models.OrderItem{
				{ID: primitive.NewObjectID()},
			})
			require.Error(mt, err)
		})
	})
}

func TestOrdersDAO_IsEventHasOpenOrders(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("returns false when no matching orders", func(mt *mtest.T) {
		dao := &OrdersDAO{
			ordersCollection:     mt.Coll,
			orderItemsCollection: mt.Coll,
			paymentsCollection:   mt.Coll,
			logger:               zap.NewNop(),
		}

		ns := fmt.Sprintf("%s.%s", mt.Coll.Database().Name(), mt.Coll.Name())
		mt.AddMockResponses(mtest.CreateCursorResponse(0, ns, mtest.FirstBatch))

		hasOpen, err := dao.IsEventHasOpenOrders(context.Background(), primitive.NewObjectID())
		require.NoError(mt, err)
		require.False(mt, hasOpen)
	})

	mt.Run("returns true when open order exists", func(mt *mtest.T) {
		dao := &OrdersDAO{
			ordersCollection:     mt.Coll,
			orderItemsCollection: mt.Coll,
			paymentsCollection:   mt.Coll,
			logger:               zap.NewNop(),
		}

		ns := fmt.Sprintf("%s.%s", mt.Coll.Database().Name(), mt.Coll.Name())
		doc := bson.D{{"_id", primitive.NewObjectID()}, {fields.FieldStatus, "created"}}
		mt.AddMockResponses(mtest.CreateCursorResponse(0, ns, mtest.FirstBatch, doc))

		hasOpen, err := dao.IsEventHasOpenOrders(context.Background(), primitive.NewObjectID())
		require.NoError(mt, err)
		require.True(mt, hasOpen)
	})

	mt.Run("propagates find errors", func(mt *mtest.T) {
		dao := &OrdersDAO{
			ordersCollection:     mt.Coll,
			orderItemsCollection: mt.Coll,
			paymentsCollection:   mt.Coll,
			logger:               zap.NewNop(),
		}

		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    123,
			Message: "failure",
			Name:    "CommandFailed",
		}))

		hasOpen, err := dao.IsEventHasOpenOrders(context.Background(), primitive.NewObjectID())
		require.Error(mt, err)
		require.False(mt, hasOpen)
	})
}

func TestOrdersDAO_GetOrdersByEvent(t *testing.T) {
	t.Run("returns empty slice when no orders match", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping integration test in short mode")
		}

		dao := setupOrdersDAOIntegration(t)

		params := &repository.GetOrdersByEventParams{
			EventID: primitive.NewObjectID(),
			Limit:   5,
			Offset:  0,
		}

		orders, total, err := dao.GetOrdersByEvent(context.Background(), params)
		require.NoError(t, err)
		require.Zero(t, total)
		require.Empty(t, orders)
	})

	t.Run("returns paginated orders with items sorted by creation time desc", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping integration test in short mode")
		}

		dao := setupOrdersDAOIntegration(t)

		ctx := context.Background()
		eventID := primitive.NewObjectID()
		otherEventID := primitive.NewObjectID()
		now := time.Now().UTC()

		recentOrder := buildOrder(primitive.NewObjectID())
		recentOrder.Event = eventID
		recentOrder.CreatedAt = now
		recentOrder.UpdatedAt = now

		olderOrder := buildOrder(primitive.NewObjectID())
		olderOrder.Event = eventID
		olderOrder.CreatedAt = now.Add(-time.Hour)
		olderOrder.UpdatedAt = olderOrder.CreatedAt

		otherEventOrder := buildOrder(primitive.NewObjectID())
		otherEventOrder.Event = otherEventID
		otherEventOrder.CreatedAt = now.Add(-2 * time.Hour)
		otherEventOrder.UpdatedAt = otherEventOrder.CreatedAt

		_, err := dao.ordersCollection.InsertMany(ctx, []interface{}{recentOrder, olderOrder, otherEventOrder})
		require.NoError(t, err)

		itemRecent := &models.OrderItem{
			ID:          primitive.NewObjectID(),
			Order:       recentOrder.ID,
			TicketStock: primitive.NewObjectID(),
			Session:     primitive.NewObjectID(),
			Name:        "recent-item",
			Price:       primitive.NewDecimal128(0, 0),
			Quantity:    1,
			Status:      "created",
			CreatedAt:   now,
			UpdatedAt:   now,
		}

		itemOlder := &models.OrderItem{
			ID:          primitive.NewObjectID(),
			Order:       olderOrder.ID,
			TicketStock: primitive.NewObjectID(),
			Session:     primitive.NewObjectID(),
			Name:        "older-item",
			Price:       primitive.NewDecimal128(0, 0),
			Quantity:    1,
			Status:      "created",
			CreatedAt:   now.Add(-30 * time.Minute),
			UpdatedAt:   now.Add(-30 * time.Minute),
		}

		itemOther := &models.OrderItem{
			ID:          primitive.NewObjectID(),
			Order:       otherEventOrder.ID,
			TicketStock: primitive.NewObjectID(),
			Session:     primitive.NewObjectID(),
			Name:        "other-event-item",
			Price:       primitive.NewDecimal128(0, 0),
			Quantity:    1,
			Status:      "created",
			CreatedAt:   now,
			UpdatedAt:   now,
		}

		_, err = dao.orderItemsCollection.InsertMany(ctx, []interface{}{itemRecent, itemOlder, itemOther})
		require.NoError(t, err)

		params := &repository.GetOrdersByEventParams{
			EventID: eventID,
			Limit:   2,
			Offset:  0,
		}

		orders, total, err := dao.GetOrdersByEvent(ctx, params)
		require.NoError(t, err)
		require.Equal(t, int64(2), total)
		require.Len(t, orders, 2)
		require.Equal(t, recentOrder.ID, orders[0].Order.ID)
		require.Len(t, orders[0].Items, 1)
		require.Equal(t, recentOrder.ID, orders[0].Items[0].Order)
		require.Equal(t, olderOrder.ID, orders[1].Order.ID)
		require.Len(t, orders[1].Items, 1)
		require.Equal(t, olderOrder.ID, orders[1].Items[0].Order)
	})

	t.Run("propagates aggregate errors", func(t *testing.T) {
		mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

		mt.Run("aggregate failure", func(mt *mtest.T) {
			dao := &OrdersDAO{
				ordersCollection:     mt.Coll,
				orderItemsCollection: mt.Coll,
				paymentsCollection:   mt.Coll,
				logger:               zap.NewNop(),
			}
			mt.AddMockResponses(
				mtest.CreateSuccessResponse(
					bson.E{Key: "n", Value: int32(1)},
				),
				mtest.CreateCommandErrorResponse(mtest.CommandError{
					Code:    134,
					Message: "aggregate failed",
					Name:    "CommandFailed",
				}),
			)

			params := &repository.GetOrdersByEventParams{
				EventID: primitive.NewObjectID(),
				Limit:   1,
				Offset:  0,
			}

			res, total, err := dao.GetOrdersByEvent(context.Background(), params)
			require.Error(mt, err)
			require.Nil(mt, res)
			require.Zero(mt, total)
		})
	})
}

func TestOrdersDAO_GetOrdersBySession(t *testing.T) {
	t.Run("returns empty slice when no orders match session", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping integration test in short mode")
		}

		dao := setupOrdersDAOIntegration(t)

		params := &repository.GetOrdersBySessionParams{
			SessionID: primitive.NewObjectID(),
			Limit:     5,
			Offset:    0,
		}

		orders, total, err := dao.GetOrdersBySession(context.Background(), params)
		require.NoError(t, err)
		require.Zero(t, total)
		require.Empty(t, orders)
	})

	t.Run("returns paginated orders with items by session sorted desc", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping integration test in short mode")
		}

		dao := setupOrdersDAOIntegration(t)

		ctx := context.Background()
		sessionID := primitive.NewObjectID()
		otherSessionID := primitive.NewObjectID()
		now := time.Now().UTC()

		recentOrder := buildOrder(primitive.NewObjectID())
		recentOrder.Session = sessionID
		recentOrder.CreatedAt = now
		recentOrder.UpdatedAt = now

		olderOrder := buildOrder(primitive.NewObjectID())
		olderOrder.Session = sessionID
		olderOrder.CreatedAt = now.Add(-2 * time.Hour)
		olderOrder.UpdatedAt = olderOrder.CreatedAt

		otherSessionOrder := buildOrder(primitive.NewObjectID())
		otherSessionOrder.Session = otherSessionID
		otherSessionOrder.CreatedAt = now.Add(-time.Hour)
		otherSessionOrder.UpdatedAt = otherSessionOrder.CreatedAt

		_, err := dao.ordersCollection.InsertMany(ctx, []interface{}{recentOrder, olderOrder, otherSessionOrder})
		require.NoError(t, err)

		items := []interface{}{
			&models.OrderItem{
				ID:          primitive.NewObjectID(),
				Order:       recentOrder.ID,
				TicketStock: primitive.NewObjectID(),
				Session:     sessionID,
				Name:        "recent-session-item",
				Price:       primitive.NewDecimal128(0, 0),
				Quantity:    1,
				Status:      "created",
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			&models.OrderItem{
				ID:          primitive.NewObjectID(),
				Order:       olderOrder.ID,
				TicketStock: primitive.NewObjectID(),
				Session:     sessionID,
				Name:        "older-session-item",
				Price:       primitive.NewDecimal128(0, 0),
				Quantity:    2,
				Status:      "created",
				CreatedAt:   now.Add(-90 * time.Minute),
				UpdatedAt:   now.Add(-90 * time.Minute),
			},
			&models.OrderItem{
				ID:          primitive.NewObjectID(),
				Order:       otherSessionOrder.ID,
				TicketStock: primitive.NewObjectID(),
				Session:     otherSessionID,
				Name:        "other-session-item",
				Price:       primitive.NewDecimal128(0, 0),
				Quantity:    3,
				Status:      "created",
				CreatedAt:   now.Add(-30 * time.Minute),
				UpdatedAt:   now.Add(-30 * time.Minute),
			},
		}

		_, err = dao.orderItemsCollection.InsertMany(ctx, items)
		require.NoError(t, err)

		params := &repository.GetOrdersBySessionParams{
			SessionID: sessionID,
			Limit:     2,
			Offset:    0,
		}

		orders, total, err := dao.GetOrdersBySession(ctx, params)
		require.NoError(t, err)
		require.Equal(t, int64(2), total)
		require.Len(t, orders, 2)
		require.Equal(t, recentOrder.ID, orders[0].Order.ID)
		require.Len(t, orders[0].Items, 1)
		require.Equal(t, recentOrder.ID, orders[0].Items[0].Order)

		require.Equal(t, olderOrder.ID, orders[1].Order.ID)
		require.Len(t, orders[1].Items, 1)
		require.Equal(t, olderOrder.ID, orders[1].Items[0].Order)
	})

	t.Run("propagates aggregate errors", func(t *testing.T) {
		mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

		mt.Run("aggregate failure", func(mt *mtest.T) {
			dao := &OrdersDAO{
				ordersCollection:     mt.Coll,
				orderItemsCollection: mt.Coll,
				paymentsCollection:   mt.Coll,
				logger:               zap.NewNop(),
			}

			mt.AddMockResponses(
				mtest.CreateSuccessResponse(
					bson.E{Key: "n", Value: int32(1)},
				),
				mtest.CreateCommandErrorResponse(mtest.CommandError{
					Code:    134,
					Message: "aggregate failed",
					Name:    "CommandFailed",
				}),
			)

			params := &repository.GetOrdersBySessionParams{
				SessionID: primitive.NewObjectID(),
				Limit:     1,
				Offset:    0,
			}

			res, total, err := dao.GetOrdersBySession(context.Background(), params)
			require.Error(mt, err)
			require.Nil(mt, res)
			require.Zero(mt, total)
		})
	})
}

func configureDockerDesktop(t *testing.T) {
	t.Helper()

	home, err := os.UserHomeDir()
	if err != nil {
		return
	}

	socket := filepath.Join(home, ".docker", "run", "docker.sock")
	if info, err := os.Stat(socket); err == nil && !info.IsDir() {
		t.Setenv("DOCKER_HOST", "unix://"+socket)
		t.Setenv("TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE", socket)
	}
}

func setupOrdersDAOIntegration(t *testing.T) *OrdersDAO {
	t.Helper()

	configureDockerDesktop(t)

	baseCtx := context.Background()
	containerCtx, cancel := context.WithTimeout(baseCtx, 5*time.Minute)
	t.Cleanup(cancel)

	mongoContainer, err := tcMongo.Run(containerCtx, "mongo:7.0.14")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, mongoContainer.Terminate(context.Background()))
	})

	connString, err := mongoContainer.ConnectionString(containerCtx)
	require.NoError(t, err)

	client, err := mongo.Connect(containerCtx, options.Client().ApplyURI(connString))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, client.Disconnect(context.Background()))
	})

	dbName := fmt.Sprintf("ordersdao_test_%d", time.Now().UnixNano())
	db := client.Database(dbName)
	t.Cleanup(func() {
		err := db.Drop(context.Background())
		var cmdErr mongo.CommandError
		if err != nil && (!errors.As(err, &cmdErr) || cmdErr.Code != 26) {
			require.NoError(t, err)
		}
	})

	return NewOrdersDAO(db, zap.NewNop())
}

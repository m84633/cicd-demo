package models

import (
	"encoding/json"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type TicketType struct {
	ID                  primitive.ObjectID   `bson:"_id,omitempty"`
	Event               primitive.ObjectID   `bson:"event,omitempty"`
	Name                string               `bson:"name"`
	Quantity            uint32               `bson:"quantity"`
	Price               primitive.Decimal128 `bson:"price"`
	Desc                string               `bson:"desc"`
	Visible             bool                 `bson:"visible"`
	AlwaysDisplay       bool                 `bson:"always_display"`
	StartShowAt         *time.Time           `bson:"start_show_at"`
	Pick                uint32               `bson:"pick"`
	StopShowAt          *time.Time           `bson:"stop_show_at"`
	UpdatedAt           *time.Time           `bson:"updated_at"`
	CreatedAt           time.Time            `bson:"created_at"`
	CreatedBy           User                 `bson:"created_by"`
	UpdatedBy           *User                `bson:"updated_by"`
	MinQuantityPerOrder uint32               `bson:"min_quantity_per_order"`
	MaxQuantityPerOrder uint32               `bson:"max_quantity_per_order"`
	DueDuration         *time.Duration       `bson:"due_duration"`
	Enable              bool                 `bson:"enable"`
	SaleSetting         string               `bson:"sale_setting"`
	//SaleStartAt         time.Time            `bson:"sale_start_at,omitempty"`
	//SaleEndAt           time.Time            `bson:"sale_end_at,omitempty"`
}

type TicketStock struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"`
	Session     primitive.ObjectID `bson:"session"`
	Type        primitive.ObjectID `bson:"type"`
	SaleStartAt time.Time          `bson:"sale_start_at"`
	SaleEndAt   time.Time          `bson:"sale_end_at"`
	Quantity    uint32             `bson:"quantity"`
	UpdatedAt   time.Time          `bson:"updated_at"`
	UpdatedBy   User               `bson:"updated_by"`
}

type Ticket struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"`
	OrderItem   OrderItemInfo      `bson:"order_item"`
	Order       uint64             `bson:"order"`    // serial
	Customer    User               `bson:"customer"` //只要有id跟name就好
	Event       primitive.ObjectID `bson:"event"`
	Session     primitive.ObjectID `bson:"session"`
	TicketStock primitive.ObjectID `bson:"ticket_type"`
	Attributes  json.RawMessage    `bson:"attributes"`
	Status      string             `bson:"status"`
	QRCodeData  string             `bson:"qr_code_data"`
	ValidFrom   time.Time          `bson:"valid_from"`
	ValidUntil  time.Time          `bson:"valid_until"`
	UsedAt      *time.Time         `bson:"used_at"`
	CheckedInBy *User              `bson:"checked_in_by"`
	CreatedAt   time.Time          `bson:"created_at"`
	UpdatedAt   time.Time          `bson:"updated_at"`
}

type OrderItemInfo struct {
	ID    primitive.ObjectID   `bson:"id"`
	Price primitive.Decimal128 `bson:"price"`
	Name  string               `bson:"name"`
}

//ticket := &models.Ticket{
//id:          primitive.NewObjectID(),
//Event:       orderWithItems.Order.Event,
//Order:       orderID,
//OrderItem:   item.id,
//Session:     item.Session,
//TicketType:  item.TicketStock, // Assuming TicketStock id corresponds to TicketType id
//User:        orderWithItems.Order.User,
//name:        item.name,
//Price:       item.Price,
//Status:      constants.TicketStatusActive.String(),
//CreatedAt:   time.Now(),
//UpdatedAt:   time.Now(),

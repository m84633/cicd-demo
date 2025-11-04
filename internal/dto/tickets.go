package dto

import (
	"partivo_tickets/internal/models"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func NewAddTicketTypeRequest(e primitive.ObjectID, s []*TicketStockRequest, n, d string, q uint32, p string, pi uint32, v bool, sso, stso *timestamppb.Timestamp, cb *models.User, min, max uint32, due time.Duration, enable bool, salesetting string) (*AddTicketTypeRequest, error) {
	price, err := primitive.ParseDecimal128(p)
	if err != nil {
		return nil, err
	}

	var startshow *time.Time
	if sso != nil {
		sts := sso.AsTime()
		startshow = &sts
	}

	var stopshow *time.Time
	if stso != nil {
		sps := stso.AsTime()
		stopshow = &sps
	}

	return &AddTicketTypeRequest{
		event:               e,
		sessions:            s,
		name:                n,
		description:         d,
		quantity:            q,
		price:               price,
		visibility:          v,
		startShowingOn:      startshow,
		stopShowingOn:       stopshow,
		createdBy:           cb,
		pick:                pi,
		minQuantityPerOrder: min,
		maxQuantityPerOrder: max,
		dueDuration:         &due,
		enable:              enable,
		saleTimeSetting:     salesetting,
	}, nil
}

func NewAddTicketStockRequest(sid primitive.ObjectID, ssa time.Time, sea time.Time) *TicketStockRequest {
	return &TicketStockRequest{
		session:     sid,
		saleStartAt: ssa,
		saleEndAt:   sea,
	}
}

type TicketStockRequest struct {
	session     primitive.ObjectID
	saleStartAt time.Time
	saleEndAt   time.Time
}

func (s *TicketStockRequest) ID() primitive.ObjectID {
	return s.session
}

func (s *TicketStockRequest) SaleStartAt() time.Time {
	return s.saleStartAt
}

func (s *TicketStockRequest) SaleEndAt() time.Time {
	return s.saleEndAt
}

type AddTicketTypeRequest struct {
	event               primitive.ObjectID
	sessions            []*TicketStockRequest
	name                string
	description         string
	quantity            uint32
	price               primitive.Decimal128
	visibility          bool
	startShowingOn      *time.Time
	stopShowingOn       *time.Time
	createdBy           *models.User
	pick                uint32
	minQuantityPerOrder uint32
	maxQuantityPerOrder uint32
	dueDuration         *time.Duration
	enable              bool
	saleTimeSetting     string
}

//uint32 min_quantity_per_order =12 [(buf.validate.field).uint32.gt = 0];
//uint32 max_quantity_per_order = 13 [(buf.validate.field).uint32.gt = 0];
//option (buf.validate.message).cel = { id: "max_gt_min_quantity", expression: "max_quantity_per_order > min_quantity_per_order" };
//optional google.protobuf.Duration due_duration = 14;

func (r AddTicketTypeRequest) GetEvent() primitive.ObjectID {
	return r.event
}

func (r AddTicketTypeRequest) GetSessions() []*TicketStockRequest {
	return r.sessions
}

func (r AddTicketTypeRequest) GetName() string {
	return r.name
}

func (r AddTicketTypeRequest) GetDescription() string {
	return r.description
}

func (r AddTicketTypeRequest) GetQuantity() uint32 {
	return r.quantity
}

func (r AddTicketTypeRequest) GetPrice() primitive.Decimal128 {
	return r.price
}

func (r AddTicketTypeRequest) GetVisibility() bool {
	return r.visibility
}

func (r AddTicketTypeRequest) GetStartShowingOn() *time.Time {
	return r.startShowingOn
}

func (r AddTicketTypeRequest) GetStopShowingOn() *time.Time {
	return r.stopShowingOn
}

func (r AddTicketTypeRequest) GetCreatedBy() *models.User {
	return r.createdBy
}

func (r AddTicketTypeRequest) GetPick() uint32 {
	return r.pick
}

func (r AddTicketTypeRequest) GetMinQuantityPerOrder() uint32 {
	return r.minQuantityPerOrder
}

func (r AddTicketTypeRequest) GetMaxQuantityPerOrder() uint32 {
	return r.maxQuantityPerOrder
}

func (r AddTicketTypeRequest) GetDueDuration() *time.Duration {
	return r.dueDuration
}

func (r AddTicketTypeRequest) GetEnable() bool {
	return r.enable
}

func (r AddTicketTypeRequest) GetSaleTimeSetting() string {
	return r.saleTimeSetting
}
func NewUpdateTicketTypeRequest(t primitive.ObjectID, n, d string, q uint32, p string, v bool, sso, stso *timestamppb.Timestamp, uu *models.User, min, max uint32, due time.Duration, sessions []*TicketStockRequest, enable bool, saletimesetting string) (*UpdateTicketTypeRequest, error) {
	price, err := primitive.ParseDecimal128(p)
	if err != nil {
		return nil, err
	}
	var startshow *time.Time
	if sso != nil {
		sts := sso.AsTime()
		startshow = &sts
	}

	var stopshow *time.Time
	if stso != nil {
		sps := stso.AsTime()
		stopshow = &sps
	}
	return &UpdateTicketTypeRequest{
		id:          t,
		name:        n,
		description: d,
		quantity:    q,
		price:       price,
		visibility:  v,
		//saleStartAt:         ssa.AsTime(),
		//saleEndAt:           sea.AsTime(),
		startShowingOn:      startshow,
		stopShowingOn:       stopshow,
		updatedBy:           uu,
		minQuantityPerOrder: min,
		maxQuantityPerOrder: max,
		dueDuration:         &due,
		sessions:            sessions,
		saleTimeSetting:     saletimesetting,
		enable:              enable,
	}, nil
}

type UpdateTicketTypeRequest struct {
	id          primitive.ObjectID
	name        string
	description string
	quantity    uint32
	price       primitive.Decimal128
	visibility  bool
	//saleStartAt         time.Time
	//saleEndAt           time.Time
	startShowingOn      *time.Time
	stopShowingOn       *time.Time
	updatedBy           *models.User
	minQuantityPerOrder uint32
	maxQuantityPerOrder uint32
	dueDuration         *time.Duration
	sessions            []*TicketStockRequest
	saleTimeSetting     string
	enable              bool
}

func (r UpdateTicketTypeRequest) GetID() primitive.ObjectID {
	return r.id
}
func (r UpdateTicketTypeRequest) GetName() string {
	return r.name
}
func (r UpdateTicketTypeRequest) GetDescription() string {
	return r.description
}
func (r UpdateTicketTypeRequest) GetQuantity() uint32 {
	return r.quantity
}
func (r UpdateTicketTypeRequest) GetPrice() primitive.Decimal128 {
	return r.price
}
func (r UpdateTicketTypeRequest) GetVisibility() bool {
	return r.visibility
}
func (r UpdateTicketTypeRequest) GetStartShowingOn() *time.Time {
	return r.startShowingOn
}
func (r UpdateTicketTypeRequest) GetStopShowingOn() *time.Time {
	return r.stopShowingOn
}
func (r UpdateTicketTypeRequest) GetUpdatedBy() *models.User {
	return r.updatedBy
}
func (r UpdateTicketTypeRequest) GetMinQuantityPerOrder() uint32 {
	return r.minQuantityPerOrder
}
func (r UpdateTicketTypeRequest) GetMaxQuantityPerOrder() uint32 {
	return r.maxQuantityPerOrder
}
func (r UpdateTicketTypeRequest) GetDueDuration() *time.Duration {
	return r.dueDuration
}

func (r UpdateTicketTypeRequest) GetEnable() bool {
	return r.enable
}

func (r UpdateTicketTypeRequest) GetSaleTimeSetting() string {
	return r.saleTimeSetting
}

func (r UpdateTicketTypeRequest) GetSessions() []*TicketStockRequest {
	return r.sessions
}

func NewUpdateTicketTypesOrderRequest(e primitive.ObjectID, ids []primitive.ObjectID, u *models.User) *UpdateTicketTypesOrderRequest {
	return &UpdateTicketTypesOrderRequest{
		event:     e,
		ids:       ids,
		updatedBy: u,
	}
}

type UpdateTicketTypesOrderRequest struct {
	event     primitive.ObjectID
	ids       []primitive.ObjectID
	updatedBy *models.User
}

func (r UpdateTicketTypesOrderRequest) GetEvent() primitive.ObjectID {
	return r.event
}

func (r UpdateTicketTypesOrderRequest) GetIDs() []primitive.ObjectID {
	return r.ids
}

func (r UpdateTicketTypesOrderRequest) GetUpdatedBy() *models.User {
	return r.updatedBy
}

// TicketTypeWithStock is a DTO that embeds a TicketType and includes the associated tickets.
type TicketTypeWithStock struct {
	models.TicketType `bson:",inline"`
	Stocks            []models.TicketStock `bson:"stocks" json:"stocks"`
}

type TicketStockWithType struct {
	Type  models.TicketType  `bson:"type"`
	Stock models.TicketStock `bson:",inline"`
}

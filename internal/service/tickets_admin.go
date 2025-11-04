package service

import (
	"context"
	"errors"
	"fmt"
	"partivo_tickets/api"
	"partivo_tickets/api/tickets"
	"partivo_tickets/internal/db"
	"partivo_tickets/internal/dto"
	"partivo_tickets/internal/helper"
	"partivo_tickets/internal/logic"
	"partivo_tickets/internal/models"
	"partivo_tickets/pkg/jwt"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type TicketsAdminService struct {
	tickets.UnimplementedTicketsAdminServiceServer
	logic  *logic.TicketLogic
	logger *zap.Logger
	tm     db.TransactionManager
	jwt    *jwt.Manager
}

func NewTicketsAdminService(l *logic.TicketLogic, logger *zap.Logger, tm db.TransactionManager, jwt *jwt.Manager) *TicketsAdminService {
	return &TicketsAdminService{
		logic:  l,
		logger: logger.Named("TicketsAdminService"),
		tm:     tm,
		jwt:    jwt,
	}
}

func (s *TicketsAdminService) CheckInTicket(c context.Context, in *tickets.CheckInTicketRequest) (*api.Response, error) {
	//TODO:Event Owner or Editor
	uid, err := getUserId(c)
	if err != nil {
		return ResponseError(codes.Unauthenticated, err)
	}
	operator := &models.User{
		UserId: uid,
		Name:   getUserName(c),
		Email:  getUserEmail(c),
	}
	// 1. Parse and validate the QR code (JWT).
	claims, err := s.jwt.Parse(in.GetQrCode())
	if err != nil {
		s.logger.Warn("CheckInTicket: QR code parsing failed", zap.Error(err), zap.String("qr_code", in.GetQrCode()))
		// Map JWT errors to gRPC status codes
		var code codes.Code
		switch {
		case errors.Is(err, jwt.ErrTokenExpired):
			code = codes.DeadlineExceeded
		case errors.Is(err, jwt.ErrTokenNotValidYet):
			code = codes.FailedPrecondition
		case errors.Is(err, jwt.ErrTokenMalformed), errors.Is(err, jwt.ErrTokenSignatureInvalid):
			code = codes.InvalidArgument
		default:
			code = codes.Unauthenticated // General catch-all for other invalid token errors
		}
		return nil, status.Error(code, err.Error())
	}

	// 2. Extract ticket ID from claims.
	ticketIDHex, ok := claims["id"].(string)
	if !ok {
		s.logger.Error("CheckInTicket: Missing or invalid 'id' claim in JWT", zap.Any("claims", claims))
		return nil, status.Error(codes.InvalidArgument, "invalid ticket payload")
	}
	ticketID, err := primitive.ObjectIDFromHex(ticketIDHex)
	if err != nil {
		s.logger.Warn("CheckInTicket: Invalid ticket ID format in JWT claim", zap.Error(err), zap.String("ticket_id_hex", ticketIDHex))
		return nil, status.Error(codes.InvalidArgument, "invalid ticket id format")
	}

	// 3. Call the business logic.
	var ticket *models.Ticket
	var wasCheckedInNow bool
	_, err = s.tm.WithTransaction(c, func(sessCtx context.Context) (interface{}, error) {
		ticket, wasCheckedInNow, err = s.logic.CheckInTicket(sessCtx, ticketID, operator)
		if err != nil {
			return nil, err
		}
		return nil, nil
	})

	// Handle fatal errors first (e.g., ticket not found, database error)
	if err != nil && ticket == nil {
		s.logger.Error("CheckInTicket: logic.CheckInTicket failed with fatal error", zap.Error(err), zap.Stringer("ticketID", ticketID))
		if errors.Is(err, logic.ErrTicketNotFound) {
			return ResponseError(codes.NotFound, err)
		}
		return ResponseError(codes.Internal, errors.New("failed to process ticket check-in"))
	}

	// 4. Build the response from the ticket model.
	resp := &tickets.CheckInTicketResponse{
		TicketId:      ticket.ID.Hex(),
		Status:        ticket.Status,
		UsedAt:        helper.ConvertTimeToProtoTimestamp(ticket.UsedAt),
		Customer:      &api.User{UserId: ticket.Customer.UserId.Hex(), Name: ticket.Customer.Name, Email: ticket.Customer.Email},
		OrderItemName: ticket.OrderItem.Name,
		CheckedInNow:  wasCheckedInNow,
	}

	// 5. If there was a non-fatal business rule violation (e.g., not within validity period),
	// return a success response but with a specific message explaining the failure.
	if err != nil {
		return ResponseSuccessWithMsg(resp, err.Error())
	}

	// 6. Otherwise, it's a standard success response.
	// The client will use the `checked_in_now` and `status` fields to determine the outcome.
	return ResponseSuccess(resp)
}

func (s *TicketsAdminService) AddTicketType(c context.Context, in *tickets.AddTicketTypeRequest) (*api.Response, error) {
	//TODO:Event Editor
	uid, err := getUserId(c)
	if err != nil {
		return ResponseError(codes.Unauthenticated, err)
	}

	u := &models.User{
		UserId: uid,
		Name:   getUserName(c),
		Email:  getUserEmail(c),
		Avatar: getUserAvatar(c),
	}

	eid, err := primitive.ObjectIDFromHex(in.GetEvent())
	if err != nil {
		s.logger.Warn("AddTicketType: Invalid event_id format", zap.Error(err), zap.String("event_id", in.GetEvent()))
		return ResponseError(codes.InvalidArgument, err)
	}

	requestSessions := make([]*dto.TicketStockRequest, 0, len(in.GetSessions()))
	sessionSet := make(map[string]struct{})
	for _, session := range in.GetSessions() {
		sid, err := primitive.ObjectIDFromHex(session.GetSession())
		if err != nil {
			s.logger.Warn("AddTicketType: Invalid session_id format", zap.Error(err), zap.Any("session", session))
			return ResponseError(codes.InvalidArgument, err)
		}
		_, ok := sessionSet[session.GetSession()]
		if ok {
			err := fmt.Errorf("session %s can not duplicate", session.GetSession())
			s.logger.Warn("AddTicketType: Duplicate session_id", zap.Error(err))
			return ResponseError(codes.AlreadyExists, err)
		}
		sessionSet[session.GetSession()] = struct{}{}
		requestSessions = append(requestSessions, dto.NewAddTicketStockRequest(sid, session.GetSaleStartAt().AsTime(), session.GetSaleEndAt().AsTime()))
	}

	d, err := dto.NewAddTicketTypeRequest(
		eid,
		requestSessions,
		in.GetName(),
		in.GetDesc(),
		in.GetQuantity(),
		in.GetPrice(),
		in.GetPick(),
		in.GetVisibility(),
		in.GetStartShowingOn(),
		in.GetStopShowingOn(),
		u,
		in.GetMinQuantityPerOrder(),
		in.GetMaxQuantityPerOrder(),
		in.GetDueDuration().AsDuration(),
		in.GetEnable(),
		in.GetSaleTimeSetting(),
	)
	if err != nil {
		s.logger.Error("AddTicketType: dto.NewAddTicketTypeRequest failed", zap.Error(err), zap.Any("request", in))
		return ResponseError(codes.InvalidArgument, err)
	}

	result, err := s.tm.WithTransaction(c, func(sessCtx context.Context) (interface{}, error) {
		return s.logic.AddTicketType(sessCtx, d)
	})
	if err != nil {
		s.logger.Error("AddTicketType: logic.AddTicketType failed", zap.Error(err), zap.Any("dto", d))
		return ResponseError(codes.Internal, err)
	}
	nid := result.(primitive.ObjectID)

	return ResponseSuccess(&api.ID{
		Id: nid.Hex(),
	})
}

func (s *TicketsAdminService) GetEventTicketTypes(c context.Context, in *tickets.GetEventTicketTypesRequest) (*api.Response, error) {
	//TODO:Event Viewer
	eid, err := primitive.ObjectIDFromHex(in.GetEvent())
	if err != nil {
		s.logger.Warn("GetEventTicketTypes: Invalid event_id format", zap.Error(err), zap.String("event_id", in.GetEvent()))
		return ResponseError(codes.InvalidArgument, err)
	}

	tl, err := s.logic.GetTicketTypesByEvent(c, eid)
	if err != nil {
		s.logger.Error("GetEventTicketTypes: logic.GetTicketTypesByEvent failed", zap.Error(err), zap.String("event_id", in.GetEvent()))
		return ResponseError(codes.Internal, err)
	}
	res := make([]*tickets.GetEventTicketTypesResponse_Ticket, 0)
	for _, v := range tl {
		var updateby *api.User
		if v.UpdatedBy != nil {
			updateby = &api.User{
				UserId: v.UpdatedBy.UserId.Hex(),
				Name:   v.UpdatedBy.Name,
				Email:  v.UpdatedBy.Email,
				Avatar: v.UpdatedBy.Avatar,
			}
		}

		sis := make([]*tickets.GetEventTicketTypesResponse_Ticket_SessionInfo, 0, len(v.Stocks))
		for _, si := range v.Stocks {
			d := &tickets.GetEventTicketTypesResponse_Ticket_SessionInfo{
				Session:  si.Session.Hex(),
				Quantity: si.Quantity,
			}
			sis = append(sis, d)
		}

		t := tickets.GetEventTicketTypesResponse_Ticket{
			Id:                  v.ID.Hex(),
			Name:                v.Name,
			Quantity:            v.Quantity,
			Price:               v.Price.String(),
			Desc:                v.Desc,
			Visibility:          v.Visible,
			StartShowingOn:      helper.ConvertTimeToProtoTimestamp(v.StartShowAt),
			StopShowingOn:       helper.ConvertTimeToProtoTimestamp(v.StopShowAt),
			CreatedAt:           helper.ConvertTimeToProtoTimestamp(v.CreatedAt),
			UpdatedAt:           helper.ConvertTimeToProtoTimestamp(v.UpdatedAt),
			CreatedBy:           &api.User{UserId: v.CreatedBy.UserId.Hex(), Name: v.CreatedBy.Name, Email: v.CreatedBy.Email, Avatar: v.CreatedBy.Avatar},
			UpdatedBy:           updateby,
			Pick:                v.Pick,
			MinQuantityPerOrder: v.MinQuantityPerOrder,
			MaxQuantityPerOrder: v.MaxQuantityPerOrder,
			SessionsInfo:        sis,
			Enable:              v.Enable,
			SaleTimeSetting:     v.SaleSetting,
		}
		res = append(res, &t)
	}
	return ResponseSuccess(&tickets.GetEventTicketTypesResponse{
		Data: res,
	})
}

func (s *TicketsAdminService) DeleteTicketType(c context.Context, in *tickets.DeleteTicketTypeRequest) (*api.Response, error) {
	//TODO:Event Editor
	tcid, err := primitive.ObjectIDFromHex(in.GetTicket())
	if err != nil {
		s.logger.Warn("DeleteTicketType: Invalid ticket_id format", zap.Error(err), zap.String("ticket_id", in.GetTicket()))
		return ResponseError(codes.InvalidArgument, err)
	}

	uid, err := getUserId(c)
	if err != nil {
		return ResponseError(codes.Unauthenticated, err)
	}
	operator := &models.User{
		UserId: uid,
		Name:   getUserName(c),
		Email:  getUserEmail(c),
	}

	result, err := s.tm.WithTransaction(c, func(sessCtx context.Context) (interface{}, error) {
		return s.logic.DeleteTicketType(sessCtx, tcid, operator)
	})
	if err != nil {
		s.logger.Error("DeleteTicketType: logic.DeleteTicketType failed", zap.Error(err), zap.String("ticket_id", in.GetTicket()))
		return ResponseError(codes.Internal, err)
	}

	s.logger.Info("DeleteTicketType: Ticket type deleted successfully", zap.Any("operator", operator), zap.Any("deletedData", result))
	return ResponseSuccess(nil)
}

func (s *TicketsAdminService) UpdateTicketType(c context.Context, in *tickets.UpdateTicketTypeRequest) (*api.Response, error) {
	//TODO:Event Editor
	uid, err := getUserId(c)
	if err != nil {
		return ResponseError(codes.Unauthenticated, err)
	}

	tid, err := primitive.ObjectIDFromHex(in.GetTicket())
	if err != nil {
		s.logger.Warn("UpdateTicketType: Invalid ticket_id format", zap.Error(err), zap.String("ticket_id", in.GetTicket()))
		return ResponseError(codes.InvalidArgument, err)
	}

	requestSessions := make([]*dto.TicketStockRequest, 0, len(in.GetSessions()))
	sessionSet := make(map[string]struct{})
	for _, session := range in.GetSessions() {
		sid, err := primitive.ObjectIDFromHex(session.GetSession())
		if err != nil {
			s.logger.Warn("UpdateTicketType: Invalid session_id format", zap.Error(err), zap.Any("session", session))
			return ResponseError(codes.InvalidArgument, err)
		}
		_, ok := sessionSet[session.GetSession()]
		if ok {
			err := fmt.Errorf("session %s can not duplicate", session.GetSession())
			s.logger.Warn("UpdateTicketType: Duplicate session_id", zap.Error(err))
			return ResponseError(codes.AlreadyExists, err)
		}
		sessionSet[session.GetSession()] = struct{}{}
		requestSessions = append(requestSessions, dto.NewAddTicketStockRequest(sid, session.GetSaleStartAt().AsTime(), session.GetSaleEndAt().AsTime()))
	}

	r, err := dto.NewUpdateTicketTypeRequest(
		tid,
		in.GetName(),
		in.GetDesc(),
		in.GetQuantity(),
		in.GetPrice(),
		in.GetVisibility(),
		in.GetStartShowingOn(),
		in.GetStopShowingOn(),
		&models.User{
			UserId: uid,
			Name:   getUserName(c),
			Email:  getUserEmail(c),
			Avatar: getUserAvatar(c),
		},
		in.GetMinQuantityPerOrder(),
		in.GetMaxQuantityPerOrder(),
		in.GetDueDuration().AsDuration(),
		requestSessions,
		in.GetEnable(),
		in.GetSaleTimeSetting(),
	)
	if err != nil {
		s.logger.Error("UpdateTicketType: dto.NewUpdateTicketTypeRequest failed", zap.Error(err), zap.Any("request", in))
		return ResponseError(codes.InvalidArgument, err)
	}

	_, err = s.tm.WithTransaction(c, func(sessCtx context.Context) (interface{}, error) {
		return nil, s.logic.UpdateTicketType(sessCtx, r)
	})
	if err != nil {
		s.logger.Error("UpdateTicketType: logic.UpdateTicketType failed", zap.Error(err), zap.Any("dto", r))
		return ResponseError(codes.Internal, err)
	}

	return ResponseSuccess(nil)
}

func (s *TicketsAdminService) UpdateTicketTypesOrder(c context.Context, in *tickets.UpdateTicketTypesOrderRequest) (*api.Response, error) {
	//TODO:Event Editor
	uid, err := getUserId(c)
	if err != nil {
		return ResponseError(codes.Unauthenticated, err)
	}

	eid, err := primitive.ObjectIDFromHex(in.GetEvent())
	if err != nil {
		s.logger.Warn("UpdateTicketTypesOrder: Invalid event_id format", zap.Error(err), zap.String("event_id", in.GetEvent()))
		return ResponseError(codes.InvalidArgument, err)
	}

	ids, err := helper.ConvertStringsToObjectID(in.GetOrder())
	if err != nil {
		s.logger.Warn("UpdateTicketTypesOrder: Failed to convert order IDs", zap.Error(err), zap.Strings("order_ids", in.GetOrder()))
		return ResponseError(codes.InvalidArgument, err)
	}

	d := dto.NewUpdateTicketTypesOrderRequest(eid, ids, &models.User{UserId: uid, Name: getUserName(c), Email: getUserEmail(c), Avatar: getUserAvatar(c)})

	_, err = s.tm.WithTransaction(c, func(sessCtx context.Context) (interface{}, error) {
		return nil, s.logic.UpdateTicketTypesOrder(sessCtx, d)
	})
	if err != nil {
		s.logger.Error("UpdateTicketTypesOrder: logic.UpdateTicketTypesOrder failed", zap.Error(err), zap.Any("dto", d))
		return ResponseError(codes.Internal, err)
	}

	return ResponseSuccess(nil)
}

package service

import (
	"context"
	"partivo_tickets/api"
	"partivo_tickets/api/tickets"
	"partivo_tickets/internal/conf"
	"partivo_tickets/internal/helper"
	"partivo_tickets/internal/logic"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
)

type TicketsService struct {
	tickets.UnimplementedTicketsServiceServer
	logic  *logic.TicketLogic
	cfg    *conf.AppConfig
	logger *zap.Logger
}

func NewTicketsService(l *logic.TicketLogic, cfg *conf.AppConfig, logger *zap.Logger) *TicketsService {
	return &TicketsService{
		logic:  l,
		cfg:    cfg,
		logger: logger.Named("TicketsService"),
	}
}

func (s *TicketsService) GetEventTickets(c context.Context, in *tickets.GetEventTicketTypesRequest) (*api.Response, error) {
	sid, err := primitive.ObjectIDFromHex(in.GetEvent())
	if err != nil {
		s.logger.Warn("GetEventTickets: Invalid event_id format", zap.Error(err), zap.String("event_id", in.GetEvent()))
		return ResponseError(codes.InvalidArgument, err)
	}

	tl, err := s.logic.GetValidTicketTypesByEvent(c, sid)
	if err != nil {
		s.logger.Error("GetEventTickets: logic.GetValidTicketTypesByEvent failed", zap.Error(err), zap.String("event_id", in.GetEvent()))
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

		//generate session info
		sis := make([]*tickets.GetEventTicketTypesResponse_Ticket_SessionInfo, 0, len(v.Stocks))
		for _, si := range v.Stocks {
			d := &tickets.GetEventTicketTypesResponse_Ticket_SessionInfo{
				Session:  si.Session.Hex(),
				Quantity: si.Quantity,
			}
			sis = append(sis, d)
		}

		t := tickets.GetEventTicketTypesResponse_Ticket{
			Id:       v.ID.Hex(),
			Name:     v.Name,
			Quantity: v.Quantity,
			Price:    v.Price.String(),
			Desc:     v.Desc,
			//SaleStartAt:         helper.ConvertTimeToProtoTimestamp(v.SaleStartAt),
			//SaleEndAt:           helper.ConvertTimeToProtoTimestamp(v.SaleEndAt),
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
		}
		res = append(res, &t)
	}
	return ResponseSuccess(&tickets.GetEventTicketTypesResponse{
		Data: res,
	})
}

func (s *TicketsService) GetSessionTickets(c context.Context, in *tickets.GetSessionTicketsRequest) (*api.Response, error) {
	sid, err := primitive.ObjectIDFromHex(in.GetSession())
	if err != nil {
		s.logger.Warn("GetSessionTickets: Invalid session_id format", zap.Error(err), zap.String("session_id", in.GetSession()))
		return ResponseError(codes.InvalidArgument, err)
	}

	tl, err := s.logic.GetTicketStockBySession(c, sid)
	if err != nil {
		s.logger.Error("GetSessionTickets: logic.GetTicketStockBySession failed", zap.Error(err), zap.String("session_id", in.GetSession()))
		return ResponseError(codes.Internal, err)
	}

	res := make([]*tickets.GetSessionTicketsResponse_Ticket, 0, len(tl))
	for _, v := range tl {
		d := &tickets.GetSessionTicketsResponse_Ticket{
			Id:            v.Stock.ID.Hex(),
			Name:          v.Type.Name,
			TotalQuantity: v.Type.Quantity,
			Price:         v.Type.Price.String(),
			Desc:          v.Type.Desc,
			//SaleStartAt:   helper.ConvertTimeToProtoTimestamp(v.Type.SaleStartAt),
			//SaleEndAt:     helper.ConvertTimeToProtoTimestamp(v.Type.SaleEndAt),
			Pick:        v.Type.Pick,
			MinPerOrder: v.Type.MinQuantityPerOrder,
			MaxPerOrder: v.Type.MaxQuantityPerOrder,
			Quantity:    v.Stock.Quantity,
		}

		res = append(res, d)
	}

	return ResponseSuccess(&tickets.GetSessionTicketsResponse{
		Data: res,
	})
}



package app

import (
	"context"
	"fmt"
	"net/http"

	billv1 "partivo_tickets/api/bills"
	orderv1 "partivo_tickets/api/orders"
	ticketv1 "partivo_tickets/api/tickets"
	"partivo_tickets/internal/limiter"
	http_middleware "partivo_tickets/internal/middleware/http"
	"partivo_tickets/internal/service"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// registerGRPCServices registers all the gRPC services.
func registerGRPCServices(s *grpc.Server, services ...interface{}) {
	for _, serviceImpl := range services {
		switch srv := serviceImpl.(type) {
		case *service.TicketsService:
			ticketv1.RegisterTicketsServiceServer(s, srv)
		case *service.TicketsAdminService:
			ticketv1.RegisterTicketsAdminServiceServer(s, srv)
		case *service.OrdersAdminService:
			orderv1.RegisterOrdersAdminServiceServer(s, srv)
		case *service.OrderService:
			orderv1.RegisterOrderServiceServer(s, srv)
		case *service.BillsAdminService:
			billv1.RegisterBillAdminServiceServer(s, srv)
		case *service.BillService:
			billv1.RegisterBillServiceServer(s, srv)
		}
	}
}

// registerGatewayHandlers registers the gRPC-Gateway handlers for all services.
func registerGatewayHandlers(ctx context.Context, gwmux *runtime.ServeMux, endpoint string) error {
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}

	if err := ticketv1.RegisterTicketsServiceHandlerFromEndpoint(ctx, gwmux, endpoint, opts); err != nil {
		return fmt.Errorf("failed to register tickets handler: %w", err)
	}

	if err := ticketv1.RegisterTicketsAdminServiceHandlerFromEndpoint(ctx, gwmux, endpoint, opts); err != nil {
		return fmt.Errorf("failed to register tickets admin handler: %w", err)
	}

	if err := orderv1.RegisterOrdersAdminServiceHandlerFromEndpoint(ctx, gwmux, endpoint, opts); err != nil {
		return fmt.Errorf("failed to register orders admin handler: %w", err)
	}

	if err := orderv1.RegisterOrderServiceHandlerFromEndpoint(ctx, gwmux, endpoint, opts); err != nil {
		return fmt.Errorf("failed to register order handler: %w", err)
	}

	if err := billv1.RegisterBillAdminServiceHandlerFromEndpoint(ctx, gwmux, endpoint, opts); err != nil {
		return fmt.Errorf("failed to register bill admin handler: %w", err)
	}

	if err := billv1.RegisterBillServiceHandlerFromEndpoint(ctx, gwmux, endpoint, opts); err != nil {
		return fmt.Errorf("failed to register bills handler: %w", err)
	}

	return nil
}

// NewHttpHandlerRegister creates the registrar function for all custom HTTP handlers.
// It takes all necessary handlers and services as dependencies and registers them on the mux.
func NewHttpHandlerRegister(
	authMiddleware http_middleware.AuthMiddleware,
	limiterManager *limiter.Manager,
	orderExportHandler *service.OrderExportHandler,
) HttpHandlerRegister {
	return func(mux *http.ServeMux) {
		// Create a rate limit middleware specifically for the export policy.
		exportRateLimiter := http_middleware.CreateRateLimitMiddleware(limiterManager, "export_orders")

		// Register all custom handlers here, wrapped with any necessary middleware.
		mux.Handle(
			"GET /api/v1/console/events/{event_id}/orders/export",
			authMiddleware(exportRateLimiter(orderExportHandler)),
		)
	}
}

package conf

import (
	"google.golang.org/grpc"
	middleware "partivo_tickets/internal/middleware/grpc"
	"time"
)

// NewUnaryInterceptors creates and returns a slice of gRPC UnaryServerInterceptor.
func NewUnaryInterceptors() []grpc.UnaryServerInterceptor {
	timeoutOverrides := map[string]time.Duration{
		//設定特定路徑的請求允許秒數
		//"/orders.Orders/AddOrder": 10 * time.Second,
	}

	return []grpc.UnaryServerInterceptor{
		middleware.AuthInterceptor(),
		middleware.NewUnaryTimeoutInterceptor(timeoutOverrides),
		middleware.UnaryPanicInterceptor,
		middleware.UnaryValidatorInterceptor,
	}
}

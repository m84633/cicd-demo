package grpc

import (
	"context"
	"log"
	"time"

	protovalidate "buf.build/go/protovalidate" // Corrected import
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/reflect/protoreflect"
)

const DefaultRequestTimeout = 5 * time.Second

// NewUnaryTimeoutInterceptor creates a gRPC unary server interceptor that sets a timeout on the context.
// It uses the DefaultRequestTimeout unless a specific timeout for the method is provided in the overrides map.
func NewUnaryTimeoutInterceptor(overrides map[string]time.Duration) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		timeout := DefaultRequestTimeout
		if t, ok := overrides[info.FullMethod]; ok {
			timeout = t
		}

		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		return handler(ctx, req)
	}
}

// UnaryValidatorInterceptor is a gRPC unary server interceptor that validates incoming requests.
func UnaryValidatorInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	// Create a new validator instance. In a real application, you might want to
	// create this once and reuse it for performance.
	validator, err := protovalidate.New()
	if err != nil {
		// This error indicates a problem with the validator itself, not the request.
		log.Printf("Failed to create protovalidate validator: %v", err)
		return nil, status.Error(codes.Internal, "internal server error")
	}

	if msg, ok := req.(protoreflect.ProtoMessage); ok {
		if err := validator.Validate(msg); err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
	}
	return handler(ctx, req)
}

// UnaryPanicInterceptor is a gRPC unary server interceptor that recovers from panics.
var UnaryPanicInterceptor grpc.UnaryServerInterceptor = func(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (resp interface{}, err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered from panic in %s: %v", info.FullMethod, r)
			err = status.Error(codes.Internal, "Internal server error")
		}
	}()
	return handler(ctx, req)
}

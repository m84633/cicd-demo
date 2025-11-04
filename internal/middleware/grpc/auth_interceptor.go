package grpc

import (
	"context"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type contextKey string

const UserIDKey contextKey = "userID"

// AuthInterceptor is a gRPC interceptor that checks for the X-User-Id header.
func AuthInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Errorf(codes.Unauthenticated, "unauthenticated: missing metadata")
		}

		values := md.Get("X-User-Id")
		if len(values) == 0 {
			return nil, status.Errorf(codes.Unauthenticated, "unauthenticated: missing user id")
		}
		userIDHex := values[0]

		userID, err := primitive.ObjectIDFromHex(userIDHex)
		if err != nil {
			return nil, status.Errorf(codes.Unauthenticated, "unauthenticated: invalid user id format")
		}

		// Add the user ID to the request context for downstream handlers.
		newCtx := context.WithValue(ctx, UserIDKey, userID)
		return handler(newCtx, req)
	}
}

// GetUserIDFromContext retrieves the user ID from the context.
// It returns the user ID and a boolean indicating if the user ID was found.
func GetUserIDFromContext(ctx context.Context) (primitive.ObjectID, bool) {
	userID, ok := ctx.Value(UserIDKey).(primitive.ObjectID)
	return userID, ok
}

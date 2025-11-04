package http

import (
	"context"
	"net/http"
	"partivo_tickets/internal/service"
)

// contextKey is a custom type for context keys to avoid collisions.
type contextKey string

// UserIDKey is the key for storing the User ID in the request context.
const UserIDKey contextKey = "userID"

// AuthMiddleware defines the function signature for our authentication middleware.
type AuthMiddleware func(http.Handler) http.Handler

// NewAuthMiddleware creates a middleware that checks for the X-User-Id header.
// It currently has no dependencies, but is set up to be provided by Wire.
func NewAuthMiddleware() AuthMiddleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := r.Header.Get("X-User-Id")
			if userID == "" {
				service.WriteHttpError(w, http.StatusUnauthorized, "Unauthorized: Missing X-User-Id header")
				return
			}

			// Add the user ID to the request context for downstream handlers.
			ctx := context.WithValue(r.Context(), UserIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

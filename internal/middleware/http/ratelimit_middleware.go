package http

import (
	"net/http"
	"partivo_tickets/internal/limiter"
	"partivo_tickets/internal/service"
)

// CreateRateLimitMiddleware is a generator function that creates a rate-limiting middleware for a specific policy.
func CreateRateLimitMiddleware(limiterManager *limiter.Manager, policyName string) func(http.Handler) http.Handler {
	// Get the specific limiter for the policy once.
	limiter := limiterManager.Get(policyName)

	// Return the actual middleware.
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get user ID from context, which was set by the AuthMiddleware.
			userID, ok := r.Context().Value(UserIDKey).(string)
			if !ok || userID == "" {
				// This should not happen if AuthMiddleware is always run first.
				service.WriteHttpError(w, http.StatusUnauthorized, "Unauthorized: User ID not found in context.")
				return
			}

			allowed, err := limiter.Allow(r.Context(), userID)
			if err != nil {
				service.WriteHttpError(w, http.StatusInternalServerError, "Failed to check rate limit.")
				return
			}

			if !allowed {
				service.WriteHttpError(w, http.StatusTooManyRequests, "Too Many Requests")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
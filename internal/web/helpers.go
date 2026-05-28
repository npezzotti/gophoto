package web

import (
	"context"
	"net/http"

	"github.com/npezzotti/gophoto/internal/domain"
)

// isAuthenticated checks if the user is authenticated by looking for the IsAuthenticatedContextKey in the request context.
func isAuthenticated(r *http.Request) bool {
	if authenticated, ok := r.Context().Value(IsAuthenticatedContextKey).(bool); ok {
		return authenticated
	}

	return false
}

// extractUserFromContext retrieves the authenticated user from the request context.
func extractUserFromContext(ctx context.Context) (*domain.UserPresentation, bool) {
	user, ok := ctx.Value(AuthenticatedUserContextKey).(*domain.UserPresentation)
	return user, ok
}

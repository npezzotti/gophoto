package web

import (
	"net/http"
)

// isAuthenticated checks if the user is authenticated by looking for the IsAuthenticatedContextKey in the request context.
func isAuthenticated(r *http.Request) bool {
	if authenticated, ok := r.Context().Value(IsAuthenticatedContextKey).(bool); ok {
		return authenticated
	}

	return false
}

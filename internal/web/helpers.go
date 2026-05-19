package web

import (
	"net/http"

	"golang.org/x/crypto/bcrypt"
)

func passwordsMatch(hash, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// isAuthenticated checks if the user is authenticated by looking for the IsAuthenticatedContextKey in the request context.
func isAuthenticated(r *http.Request) bool {
	if authenticated, ok := r.Context().Value(IsAuthenticatedContextKey).(bool); ok {
		return authenticated
	}

	return false
}

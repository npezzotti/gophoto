package web

import (
	"net/http"

	"golang.org/x/crypto/bcrypt"
)

func hashPassword(password string) (string, error) {
	passwdBytes := []byte(password)

	hashedPasswdBytes, err := bcrypt.GenerateFromPassword(passwdBytes, bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	return string(hashedPasswdBytes), nil
}

func passwordsMatch(hash, password string) bool {
	hashBytes := []byte(hash)
	passwdBytes := []byte(password)
	err := bcrypt.CompareHashAndPassword(hashBytes, passwdBytes)

	return err == nil
}

// isAuthenticated checks if the user is authenticated by looking for the IsAuthenticatedContextKey in the request context.
func isAuthenticated(r *http.Request) bool {
	if isAuthenticated, ok := r.Context().Value(IsAuthenticatedContextKey).(bool); ok {
		return isAuthenticated
	}

	return false
}

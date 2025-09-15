package web

import (
	"database/sql"
	"net/http"

	"github.com/npezzotti/gophoto/db"
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

func isAuthenticated(r *http.Request) bool {
	if isAuthenticated, ok := r.Context().Value(isAuthenticatedContextKey).(bool); ok {
		return isAuthenticated
	}

	return false
}

func (a *application) getUserFromRequest(r *http.Request) *db.User {
	if userId, ok := r.Context().Value(authenticatedUserId).(int32); ok {
		user, err := a.database.GetUserById(r.Context(), userId)
		if err != nil {
			if err != sql.ErrNoRows {
				a.ErrorLog.Printf("error querying user: %s\n", err.Error())
			}

			return &db.User{}
		}
		return &user
	}

	return &db.User{}
}

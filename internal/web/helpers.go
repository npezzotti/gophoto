package web

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"

	"github.com/npezzotti/gophoto/internal/db"
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

// getUserFromRequest retrieves the authenticated user's details from the request context.
func (a *application) getUserFromRequest(r *http.Request) *db.GetUserByIdRow {
	if userId, ok := r.Context().Value(AuthenticatedUserId).(int32); ok {
		userRow, err := a.database.GetUserById(r.Context(), userId)
		if err != nil {
			a.ErrorLog.Printf("error getting user by id from request: %s", err.Error())
			if err != sql.ErrNoRows {
				a.ErrorLog.Printf("error querying user: %s\n", err.Error())
			}
			return &db.GetUserByIdRow{}
		}
		return &userRow
	}

	return &db.GetUserByIdRow{}
}

// writeJsonResp writes the provided data as a JSON response with the specified HTTP status code.
func (a *application) writeJsonResp(w http.ResponseWriter, status int, data any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(data)
}

// detectContentType reads the first 512 bytes of the provided file to determine its content type.
// It resets the file's read pointer to the beginning before returning.
func detectContentType(f multipart.File) (string, error) {
	buff := make([]byte, 512)
	_, err := f.Read(buff)
	if err != nil {
		return "", fmt.Errorf("error reading file: %w", err)
	}

	filetype := http.DetectContentType(buff)

	_, err = f.Seek(0, io.SeekStart)
	if err != nil {
		return "", fmt.Errorf("seek: %s", err)
	}
	return filetype, nil
}

package web

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
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

func (a *application) getUserFromRequest(r *http.Request) *db.GetUserByIdRow {
	if userId, ok := r.Context().Value(authenticatedUserId).(int32); ok {
		userRow, err := a.database.GetUserById(r.Context(), userId)
		if err != nil {
			a.InfoLog.Printf("error getting user by id from request: %s", err.Error())
			if err != sql.ErrNoRows {
				a.ErrorLog.Printf("error querying user: %s\n", err.Error())
			}
		}
		return &userRow
	}

	return nil
}

func (a *application) writeJsonResp(w http.ResponseWriter, status int, data any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(data)
}

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

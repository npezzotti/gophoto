package web

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/npezzotti/gophoto/internal/db"
	"github.com/npezzotti/gophoto/internal/utils"
	"golang.org/x/crypto/bcrypt"
)

// writeJsonResp writes the provided data as a JSON response with the specified HTTP status code.
func (a *application) writeJsonResp(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		a.ErrorLog.Println("error writing json response:", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

func (a *application) writeJsonErrorResp(w http.ResponseWriter, status int, message string) {
	resp := map[string]string{"error": message}
	a.writeJsonResp(w, status, resp)
}

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

// Authenticate and extract user
func (a *application) authenticateRequest(r *http.Request) (*db.GetUserByIdRow, error) {
	if !isAuthenticated(r) {
		return nil, fmt.Errorf("not authenticated")
	}

	user := a.extractUserFromRequest(r)
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}

	return user, nil
}

// isAuthenticated checks if the user is authenticated by looking for the IsAuthenticatedContextKey in the request context.
func isAuthenticated(r *http.Request) bool {
	if isAuthenticated, ok := r.Context().Value(IsAuthenticatedContextKey).(bool); ok {
		return isAuthenticated
	}

	return false
}

// extractUserFromRequest retrieves the authenticated user's details from the request context.
func (a *application) extractUserFromRequest(r *http.Request) *db.GetUserByIdRow {
	if userId, ok := r.Context().Value(AuthenticatedUserId).(int32); ok {
		userRow, err := a.database.GetUserById(r.Context(), userId)
		if err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				a.ErrorLog.Printf("error querying user: %s", err.Error())
			}
			return nil
		}
		return &userRow
	}

	return nil
}

func validatePhotoUpload(fileType string, fh *multipart.FileHeader) error {
	if fh.Size > MaxUploadSize {
		return fmt.Errorf("file size exceeds max upload size of %dMB", MaxUploadSize/1024/1024)
	}

	if !strings.HasPrefix(fileType, "image/") || !utils.ValidateMimeType(fileType) {
		return fmt.Errorf("file type not allowed")
	}
	return nil
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

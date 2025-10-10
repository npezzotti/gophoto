package web

import (
	"fmt"
	"io"
	"mime/multipart"
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

package store

import (
	"crypto/hmac"
	"crypto/sha256"
	"strconv"
)

// CreateMessage creates a message string for signing by combining the URL path and expiration timestamp.
// The format is the URL path followed by a colon and the expiration time in Unix timestamp format.
func CreateMessage(path string, expiresUnix int64) string {
	return path + ":" + strconv.FormatInt(expiresUnix, 10)
}

// generateHmac generates an HMAC signature for the given message using the provided secret key.
// It returns the computed HMAC as a byte slice.
func generateHmac(message string, secret []byte) []byte {
	h := hmac.New(sha256.New, secret)
	h.Write([]byte(message))
	return h.Sum(nil)
}

// VerifySignature verifies that the provided HMAC signature matches the expected signature for a given message and secret key.
func VerifySignature(message string, providedMac, secret []byte) bool {
	expectedMac := generateHmac(message, secret)
	if len(providedMac) != len(expectedMac) {
		return false
	}
	return hmac.Equal(providedMac, expectedMac)
}

package signature

import (
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
)

// CreateMessage creates a message string for signing by combining the file path and expiration timestamp.
// The format is the URL path followed by a colon and the expiration time in Unix timestamp format.
func CreateMessage(path string, expiresUnix int64) string {
	return fmt.Sprintf("%s:%d", path, expiresUnix)
}

// GenerateHmac generates an HMAC signature for the given message using the provided secret key.
func GenerateHmac(message string, secret []byte) []byte {
	h := hmac.New(sha256.New, secret)
	h.Write([]byte(message))
	return h.Sum(nil)
}

// VerifySignature verifies that the provided HMAC signature matches the expected signature for a given message and secret key.
func VerifySignature(message string, messageMac, secret []byte) bool {
	expectedMac := GenerateHmac(message, secret)
	return hmac.Equal(messageMac, expectedMac)
}

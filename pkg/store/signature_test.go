package store

import (
	"bytes"
	"encoding/hex"
	"testing"
	"time"
)

func TestCreateMessage(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		expiresUnix int64
		want        string
	}{
		{
			name:        "valid message creation",
			path:        "/test/path",
			expiresUnix: time.Date(2024, 2, 14, 0, 0, 0, 0, time.UTC).Unix(),
			want:        "/test/path:1707868800",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CreateMessage(tt.path, tt.expiresUnix)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_generateHmac(t *testing.T) {
	tests := []struct {
		name          string
		message       string
		secret        []byte
		wantHexString string
	}{
		{
			name:          "generate HMAC for a message",
			message:       "test message",
			secret:        []byte("test secret"),
			wantHexString: "b5664a92da7fef821fa7ff75c00f711ba615dcb610de82edc440bc1337e251ef",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want, err := hex.DecodeString(tt.wantHexString)
			if err != nil {
				t.Fatalf("failed to decode hex string: %v", err)
			}

			got := generateHmac(tt.message, tt.secret)
			if !bytes.Equal(got, want) {
				t.Errorf("got %x, want %x", got, want)
			}
		})
	}
}

func TestVerifySignature(t *testing.T) {
	tests := []struct {
		name        string
		message     string
		providedMac []byte
		secret      []byte
		want        bool
	}{
		{
			name:        "valid signature verification",
			message:     "test message",
			providedMac: generateHmac("test message", []byte("test secret")),
			secret:      []byte("test secret"),
			want:        true,
		},
		{
			name:        "invalid signature verification",
			message:     "test message",
			providedMac: generateHmac("test message", []byte("wrong secret")),
			secret:      []byte("test secret"),
			want:        false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := VerifySignature(tt.message, tt.providedMac, tt.secret)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

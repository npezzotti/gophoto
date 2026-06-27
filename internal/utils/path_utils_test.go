package utils

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/npezzotti/gophoto/internal/domain"
)

func TestBuildPhotoPathForVariant(t *testing.T) {
	tcases := []struct {
		name    string
		key     string
		variant domain.PhotoVariant
		mime    domain.MimeType
		errStr  string
	}{
		{
			name:    "Original JPEG",
			key:     uuid.New().String(),
			variant: domain.PhotoVariantOriginal,
			mime:    domain.MimeTypeJPEG,
		},
		{
			name:   "Invalid UUID",
			key:    "not-a-uuid",
			errStr: "invalid UUID format",
		},
		{
			name:    "Invalid MIME type",
			key:     uuid.New().String(),
			variant: domain.PhotoVariantOriginal,
			mime:    domain.MimeType("image.txt"),
			errStr:  "failed to extract file extension",
		},
		{
			name:    "Invalid variant",
			key:     uuid.New().String(),
			variant: domain.PhotoVariant("invalid"),
			mime:    domain.MimeTypeJPEG,
			errStr:  "invalid photo variant",
		},
	}
	for _, tc := range tcases {
		res, err := BuildPhotoPathForVariant(tc.key, tc.variant, tc.mime)
		if err != nil {
			if tc.errStr == "" {
				t.Errorf("unexpected error: %v", err)
			} else if !strings.Contains(err.Error(), tc.errStr) {
				t.Errorf("expected error %q, got %v", tc.errStr, err)
			}
			continue
		}

		ext, err := extractFileExtension(tc.mime)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		expected := tc.key[0:2] + "/" + tc.key[2:4] + "/" + tc.key + "/" + string(tc.variant) + "." + ext
		if res != expected {
			t.Errorf("expected %s, got %s", expected, res)
		}
	}
}

func Test__extractFileExtension(t *testing.T) {
	tcases := []struct {
		name     string
		input    domain.MimeType
		expected string
		wantErr  bool
	}{
		{
			name:     "JPEG file",
			input:    domain.MimeType("image/jpeg"),
			expected: "jpg",
		},
		{
			name:     "PNG file",
			input:    domain.MimeType("image/png"),
			expected: "png",
		},
		{
			name:     "WEBP file",
			input:    domain.MimeType("image/webp"),
			expected: "webp",
		},
		{
			name:    "Invalid file",
			input:   domain.MimeType("image.txt"),
			wantErr: true,
		},
	}

	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := extractFileExtension(tc.input)
			if err != nil && !tc.wantErr {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if result != tc.expected {
				t.Errorf("expected %s, got %s", tc.expected, result)
			}
		})
	}
}

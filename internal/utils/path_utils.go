package utils

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/npezzotti/gophoto/internal/domain"
)

// BuildPhotoPathForVariant constructs a hierarchical file path for storing photos based on the provided key and variant.
func BuildPhotoPathForVariant(key string, variant domain.PhotoVariant, mimeType domain.MimeType) (string, error) {
	// Validate UUID format of the key.
	if _, err := uuid.Parse(key); err != nil {
		return "", fmt.Errorf("invalid UUID format: %w", err)
	}

	// Use the first four characters of the UUID to create a two-level directory structure.
	shardLvl1 := key[0:2]
	shardLvl2 := key[2:4]

	ext, err := extractFileExtension(mimeType)
	if err != nil {
		return "", fmt.Errorf("failed to extract file extension: %w", err)
	}

	return fmt.Sprintf("/%s/%s/%s/%s.%s", shardLvl1, shardLvl2, key, (string(variant)), ext), nil
}

func extractFileExtension(mimeType domain.MimeType) (string, error) {
	switch mimeType {
	case domain.MimeTypeJPEG:
		return "jpg", nil
	case domain.MimeTypePNG:
		return "png", nil
	case domain.MimeTypeWEBP:
		return "webp", nil
	default:
		return "", fmt.Errorf("unsupported MIME type: %s", mimeType)
	}
}

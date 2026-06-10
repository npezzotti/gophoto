package utils

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/npezzotti/gophoto/internal/domain"
)

// BuildPhotoPathForVariant constructs a hierarchical file path for storing photos based on the provided key and variant.
// The key is expected to be a UUID string, and the path is structured as follows:
// {shardLvl1}/{shardLvl2}/{key}/{variant}.{ext}
// where shardLvl1 and shardLvl2 are derived from the first four characters of the UUID to create a two-level directory structure.
// The file extension, ext, is determined based on the MIME type of the photo.
func BuildPhotoPathForVariant(key string, variant domain.PhotoVariant, mimeType domain.MimeType) (string, error) {
	// Parse the UUID from the key.
	parsedUUID, err := uuid.Parse(key)
	if err != nil {
		return "", fmt.Errorf("invalid UUID format: %w", err)
	}

	parsedUUIDStr := parsedUUID.String()
	// Use the first four characters of the UUID to create a two-level directory structure.
	shardLvl1 := parsedUUIDStr[0:2]
	shardLvl2 := parsedUUIDStr[2:4]

	// Determine the file extension based on the MIME type.
	ext, err := extractFileExtension(mimeType)
	if err != nil {
		return "", fmt.Errorf("failed to extract file extension: %w", err)
	}

	return fmt.Sprintf("%s/%s/%s/%s.%s", shardLvl1, shardLvl2, parsedUUIDStr, (string(variant)), ext), nil
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

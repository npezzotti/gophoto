package utils

import (
	"fmt"

	"github.com/npezzotti/gophoto/internal/domain"
)

type MimeType string

const (
	MimeTypeJPEG MimeType = "image/jpeg"
	MimeTypePNG  MimeType = "image/png"
	MimeTypeWEBP MimeType = "image/webp"
)

var AllowedImageMimeTypes = []MimeType{
	MimeTypeJPEG,
	MimeTypePNG,
	MimeTypeWEBP,
}

func ValidateMimeType(mimeType string) bool {
	for _, allowedType := range AllowedImageMimeTypes {
		if mimeType == string(allowedType) {
			return true
		}
	}
	return false
}

// BuildPhotoPathForVariant constructs a hierarchical file path for storing photos based on the provided key and variant.
func BuildPhotoPathForVariant(key string, variant domain.PhotoVariant, mimeType MimeType) (string, error) {
	// Use the first four characters of the UUID to create a two-level directory structure.
	shardLvl1 := key[0:2]
	shardLvl2 := key[2:4]

	ext, err := extractFileExtension(mimeType)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("/%s/%s/%s/%s.%s", shardLvl1, shardLvl2, key, (string(variant)), ext), nil
}

func extractFileExtension(mimeType MimeType) (string, error) {
	switch mimeType {
	case MimeTypeJPEG:
		return "jpg", nil
	case MimeTypePNG:
		return "png", nil
	case MimeTypeWEBP:
		return "webp", nil
	default:
		return "", fmt.Errorf("unsupported mime type: %s", mimeType)
	}
}

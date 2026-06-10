package domain

import (
	"errors"
	"time"
)

var (
	ErrPhotoNotFound      = errors.New("photo not found")
	ErrAlbumPhotoNotFound = errors.New("album photo not found")
)

type PhotoVariant string

const (
	PhotoVariantOriginal PhotoVariant = "original"
	PhotoVariantLarge    PhotoVariant = "large"
	PhotoVariantSmall    PhotoVariant = "small"
	PhotoVariantMedium   PhotoVariant = "medium"
	PhotoVariantThumb    PhotoVariant = "thumb"
	PhotoVariantAvatar   PhotoVariant = "avatar"
)

func (v PhotoVariant) IsValid() bool {
	switch v {
	case PhotoVariantOriginal, PhotoVariantLarge, PhotoVariantMedium, PhotoVariantSmall, PhotoVariantThumb, PhotoVariantAvatar:
		return true
	default:
		return false
	}
}

type PhotoStatus string

const (
	PhotoStatusProcessing PhotoStatus = "processing"
	PhotoStatusProcessed  PhotoStatus = "processed"
	PhotoStatusErrored    PhotoStatus = "errored"
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

type Photo struct {
	ID        int32
	UserID    *int32
	Key       string
	Status    PhotoStatus
	CreatedAt time.Time
	UpdatedAt time.Time
}

type AlbumPhoto struct {
	ID        int32
	UserID    *int32
	Key       string
	Status    PhotoStatus
	CreatedAt time.Time
	UpdatedAt time.Time
	AlbumID   int32
}

type PhotoMetadatum struct {
	ID        int32
	PhotoID   int32
	Variant   PhotoVariant
	Width     int32
	Height    int32
	FileSize  int64
	MimeType  string
	CreatedAt time.Time
}

// CreatePhotoWithOriginalMetadataParams is used for creating a new photo with its original metadata.
type CreatePhotoWithOriginalMetadataParams struct {
	UserID   int32
	Key      string
	Width    int32
	Height   int32
	FileSize int64
	MimeType string
}

// CreatePhotoMetadataParams is used for creating metadata for processed photo variants.
type CreatePhotoMetadataParams struct {
	PhotoID  int32
	Variant  PhotoVariant
	Width    int32
	Height   int32
	FileSize int64
	MimeType string
}

package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrPhotoNotFound = errors.New("photo not found")

type Photo struct {
	ID        int32
	UserID    *int32
	Key       string
	Status    PhotoStatus
	CreatedAt time.Time
	UpdatedAt time.Time
}

type PhotoStatus string

const (
	PhotoStatusProcessing PhotoStatus = "processing"
	PhotoStatusProcessed  PhotoStatus = "processed"
	PhotoStatusErrored    PhotoStatus = "errored"
)

type AlbumPhoto struct {
	ID        int32
	UserID    *int32
	Key       string
	Status    PhotoStatus
	CreatedAt time.Time
	UpdatedAt time.Time
	AlbumID   int32
}

type ImageResponse struct {
	Image       Photo
	Alt         string
	OriginalSrc string
	DefaultSrc  string
	Sources     []Image
}

type Image struct {
	Width  int32
	Height int32
	URL    string
}

type PhotoMetadatum struct {
	ID        int32
	PhotoID   int32
	Variant   PhotoVariant
	Width     int32
	Height    int32
	FileSize  *int64
	MimeType  string
	CreatedAt time.Time
}

type CreatePhotoWithOriginalMetadataCommand struct {
	UserID   *int32
	Key      string
	Width    int32
	Height   int32
	FileSize *int64
	MimeType string
}

type CreatePhotoMetadataCommand struct {
	PhotoID  int32
	Variant  PhotoVariant
	Width    int32
	Height   int32
	FileSize *int64
	MimeType string
}

type PhotoVariant string

const (
	PhotoVariantOriginal PhotoVariant = "original"
	PhotoVariantLarge    PhotoVariant = "large"
	PhotoVariantSmall    PhotoVariant = "small"
	PhotoVariantMedium   PhotoVariant = "medium"
	PhotoVariantThumb    PhotoVariant = "thumb"
	PhotoVariantAvatar   PhotoVariant = "avatar"
)

func (i *ImageResponse) SrcSet() string {
	var srcset []string
	for _, source := range i.Sources {
		srcset = append(srcset, fmt.Sprintf("%s %dw", source.URL, source.Width))
	}
	return strings.Join(srcset, ", ")
}

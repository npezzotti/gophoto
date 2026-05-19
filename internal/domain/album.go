package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

const DefaultAlbumCover = "images/album_cover.webp"

var ErrAlbumNotFound = errors.New("album not found")

type Album struct {
	ID           int32
	UserID       int32
	Title        string
	CoverPhotoID *int32
	NumPhotos    int32
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type AlbumListProjection struct {
	Album         Album
	CoverPhotoKey string
}

type AlbumListItem struct {
	Album           Album
	AlbumCoverImage ResponsiveImage
}

type ResponsiveImage struct {
	ID          int32
	Alt         string
	OriginalSrc string
	DefaultSrc  string
	Sources     []ImageSource
}

type ImageSource struct {
	Width  int32
	Height int32
	URL    string
}

func (ri ResponsiveImage) SrcSet() string {
	var srcset []string
	for _, source := range ri.Sources {
		srcset = append(srcset, fmt.Sprintf("%s %dw", source.URL, source.Width))
	}

	return strings.Join(srcset, ", ")
}

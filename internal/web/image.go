package web

import (
	"fmt"
	"strings"

	"github.com/npezzotti/gophoto/internal/db"
)

type Image struct {
	Width  int32
	Height int32
	URL    string
}

type Size struct {
	MediaQuery string
	Size       string
}

type ImageResponse struct {
	Image       db.Photo
	Alt         string
	OriginalSrc string
	DefaultSrc  string
	Sources     []Image
}

func (ar *ImageResponse) SrcSet() string {
	var srcset []string
	for _, source := range ar.Sources {
		srcset = append(srcset, fmt.Sprintf("%s %dw", source.URL, source.Width))
	}
	return strings.Join(srcset, ", ")
}

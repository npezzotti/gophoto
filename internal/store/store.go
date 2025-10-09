package store

import (
	"context"
	"errors"
	"io"
)

type FileName string

const (
	FileNameOriginal  FileName = "original"
	FileNameThumbnail FileName = "thumbnail"
	FileNameSmall     FileName = "small"
	FileNameMedium    FileName = "medium"
	FileNameLarge     FileName = "large"
	FileNameAvatar    FileName = "avatar"
)

var ErrNotExist error = errors.New("file does not exist")

type Store interface {
	GenerateURL(ctx context.Context, key string) (string, error)
	Read(ctx context.Context, key string) (io.ReadCloser, error)
	Write(ctx context.Context, key string, file io.Reader) error
	Delete(ctx context.Context, key string) error
}

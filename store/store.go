package store

import (
	"context"
	"errors"
	"io"
)

type FileSuffix string

const (
	FileSuffixOriginal  FileSuffix = "_original"
	FileSuffixThumbnail FileSuffix = "_thumb"
	FileSuffixSmall     FileSuffix = "_small"
	FileSuffixMedium    FileSuffix = "_medium"
	FileSuffixLarge     FileSuffix = "_large"
	FileSuffixAvatar    FileSuffix = "_avatar"
)

var ErrNotExist error = errors.New("file does not exist")

type Store interface {
	GenerateURL(ctx context.Context, key string) (string, error)
	Write(ctx context.Context, key string, file io.Reader) error
	Delete(ctx context.Context, key string) error
}

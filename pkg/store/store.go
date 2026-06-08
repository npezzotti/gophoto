package store

import (
	"context"
	"errors"
	"io"
	"time"
)

var ErrNotExist error = errors.New("file does not exist")

type Store interface {
	// GenerateURL generates a presigned URL for the given key that expires after a short period of time.
	GenerateURL(ctx context.Context, key string, expiry time.Duration) (string, error)
	// Read returns a ReadCloser for the given key. The caller is responsible for closing the reader.
	Read(ctx context.Context, key string) (io.ReadCloser, error)
	// Write writes the given file to the store with the given key.
	Write(ctx context.Context, key string, file io.Reader) error
	// Delete deletes the file with the given key from the store.
	Delete(ctx context.Context, key string) error
}

package store

import (
	"context"
	"errors"
	"io"

	"github.com/npezzotti/gophoto/config"
)

var ErrNotExist error = errors.New("file does not exist")

type Store interface {
	Read(ctx context.Context, key string) (string, error)
	Write(ctx context.Context, key string, file io.Reader) error
	Delete(ctx context.Context, key string) error
}

func NewStore(cfg *config.Config) (Store, error) {
	var photoStore Store

	switch cfg.StorageType {
	case config.StorageTypeDisk:
		s, err := NewFileStore(cfg.BaseDir)
		if err != nil {
			return nil, err
		}

		photoStore = s
	case config.StorageTypeS3:
		s := NewS3Store()
		s.BucketName = cfg.BucketName
		photoStore = s
	default:
		return nil, errors.New("storage type not supported")
	}

	return photoStore, nil
}

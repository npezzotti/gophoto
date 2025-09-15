package store

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const defaultBaseDir = "./uploads"

type FileStore struct {
	BaseDir string
}

func NewFileStore(baseDir string) (*FileStore, error) {
	if baseDir == "" {
		baseDir = defaultBaseDir
	}

	if err := os.MkdirAll(baseDir, os.ModePerm); err != nil {
		return nil, fmt.Errorf("error creating upload directory: %w", err)
	}

	return &FileStore{
		BaseDir: baseDir,
	}, nil
}

func (fs *FileStore) Read(ctx context.Context, key string) (string, error) {
	f := fs.path(key)
	if _, err := os.Stat(f); err != nil {
		return "", err
	}

	return filepath.Join("/", f), nil
}

func (fs *FileStore) Write(ctx context.Context, key string, file io.Reader) error {
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}

	tempFile, err := os.Create(fs.path(key))
	if err != nil {
		return fmt.Errorf("error creating photo %w", err)
	}
	defer tempFile.Close()

	_, err = tempFile.Write(fileBytes)
	if err != nil {
		return fmt.Errorf("error writing file: %w", err)
	}

	return nil
}

func (fs *FileStore) Delete(ctx context.Context, key string) error {
	path := fs.path(key)

	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrNotExist
		} else {
			return err
		}
	}

	if err = os.Remove(path); err != nil {
		return err
	}

	return err
}

func (fs *FileStore) path(key string) string {
	return filepath.Join(fs.BaseDir, key)
}

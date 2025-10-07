package store

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type FileStore struct {
	BaseDir string
}

func NewFileStore(baseDir string) (*FileStore, error) {
	if baseDir == "" {
		return nil, fmt.Errorf("base directory required")
	}

	if err := os.MkdirAll(baseDir, os.ModePerm); err != nil {
		return nil, fmt.Errorf("error creating upload directory: %w", err)
	}

	return &FileStore{
		BaseDir: baseDir,
	}, nil
}

func (fs *FileStore) GenerateURL(ctx context.Context, path string) (string, error) {
	f := fs.path(path)
	if _, err := os.Stat(f); err != nil {
		return "", err
	}

	return filepath.Join("/", f), nil
}

func (fs *FileStore) Write(ctx context.Context, path string, file io.Reader) error {
	// Create the directory if it doesn't exist
	fpath := fs.path(path)
	dir := filepath.Dir(fpath)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return fmt.Errorf("error creating directories for photo: %w", err)
	}

	tempFile, err := os.Create(fpath)
	if err != nil {
		return fmt.Errorf("error creating photo %w", err)
	}
	defer tempFile.Close()

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}

	_, err = tempFile.Write(fileBytes)
	if err != nil {
		return fmt.Errorf("error writing file: %w", err)
	}

	return nil
}

func (fs *FileStore) Delete(ctx context.Context, path string) error {
	prefixedPath := fs.path(path)
	_, err := os.Stat(prefixedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrNotExist
		} else {
			return err
		}
	}

	if err = os.Remove(prefixedPath); err != nil {
		return fmt.Errorf("error deleting file: %w", err)
	}

	// Attempt to remove the parent directory if it's empty
	parentDir := filepath.Dir(prefixedPath)
	entries, err := os.ReadDir(parentDir)
	if err == nil && len(entries) == 0 {
		_ = os.Remove(parentDir)
	}

	return nil
}

func (fs *FileStore) path(key string) string {
	return filepath.Join(fs.BaseDir, key)
}

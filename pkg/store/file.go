package store

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

type FileStore struct {
	BaseDir   string
	secretKey []byte
}

// NewFileStore creates a new FileStore with the given base directory and secret key.
func NewFileStore(baseDir string, secretKey []byte) (*FileStore, error) {
	if baseDir == "" {
		return nil, fmt.Errorf("base directory required")
	}

	if len(secretKey) == 0 {
		return nil, fmt.Errorf("secret key required")
	}

	if err := os.MkdirAll(baseDir, os.ModePerm); err != nil {
		return nil, fmt.Errorf("error creating upload directory: %w", err)
	}

	return &FileStore{
		BaseDir:   baseDir,
		secretKey: secretKey,
	}, nil
}

func (fs *FileStore) GenerateURL(ctx context.Context, path string) (string, error) {
	filePath := fs.path(path)
	if _, err := os.Stat(filePath); err != nil {
		return "", fmt.Errorf("error stating file: %w", err)
	}

	urlPath := filepath.Join("/", filePath)
	expiry := time.Now().Add(15 * time.Minute)
	message := CreateMessage(urlPath, expiry.Unix())
	signature := GenerateHmac(message, fs.secretKey)
	b64Sig := base64.URLEncoding.EncodeToString(signature)

	return fmt.Sprintf("%s?expires=%d&signature=%s", urlPath, expiry.Unix(), b64Sig), nil
}

func (fs *FileStore) Read(ctx context.Context, path string) (io.ReadCloser, error) {
	filePath := fs.path(path)
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("error opening file: %w", err)
	}

	return f, nil
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

// path returns the full path to the file by joining the
// base directory with the provided path
func (fs *FileStore) path(path string) string {
	return filepath.Join(fs.BaseDir, path)
}

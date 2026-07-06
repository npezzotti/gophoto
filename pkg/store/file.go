package store

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"time"
)

type FileStore struct {
	baseDir   string
	secretKey []byte
	urlPrefix string
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
		baseDir:   baseDir,
		secretKey: secretKey,
		urlPrefix: "/uploads",
	}, nil
}

func (fs *FileStore) GenerateURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	if expiry <= 0 {
		return "", fmt.Errorf("expiry duration must be greater than zero")
	}

	filePath := fs.path(key)
	if _, err := os.Stat(filePath); err != nil {
		if os.IsNotExist(err) {
			return "", ErrNotExist
		}
		return "", fmt.Errorf("error stating file: %w", err)
	}

	urlPath := fs.publicPath(key)
	expiryTime := time.Now().Add(expiry)
	message := CreateMessage(urlPath, expiryTime.Unix())
	signature := GenerateSignature(message, fs.secretKey)
	b64Sig := base64.RawURLEncoding.EncodeToString(signature)

	finalURL := url.URL{
		Path: urlPath,
	}
	finalURL.RawQuery = url.Values{
		"expires":   []string{fmt.Sprintf("%d", expiryTime.Unix())},
		"signature": []string{b64Sig},
	}.Encode()

	return finalURL.String(), nil
}

func (fs *FileStore) Read(ctx context.Context, path string) (io.ReadCloser, error) {
	filePath := fs.path(path)
	fh, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("error opening file: %w", err)
	}

	return fh, nil
}

func (fs *FileStore) Write(ctx context.Context, path string, file io.Reader) error {
	// Create the directory if it doesn't exist
	filePath := fs.path(path)
	parentDir := filepath.Dir(filePath)
	if err := os.MkdirAll(parentDir, os.ModePerm); err != nil {
		return fmt.Errorf("error creating parent directory for photo: %w", err)
	}

	photoFile, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("error creating photo: %w", err)
	}
	defer photoFile.Close()

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}

	_, err = photoFile.Write(fileBytes)
	if err != nil {
		return fmt.Errorf("error writing file: %w", err)
	}

	return nil
}

func (fs *FileStore) Delete(ctx context.Context, path string) error {
	filePath := fs.path(path)
	if _, err := os.Stat(filePath); err != nil {
		if os.IsNotExist(err) {
			return ErrNotExist
		}
		return fmt.Errorf("error stating file: %w", err)
	}

	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("error deleting file: %w", err)
	}

	// Attempt to remove the parent directory if this was the last file, but ignore
	// any errors since this is just a best effort cleanup
	parentDir := filepath.Dir(filePath)
	files, err := os.ReadDir(parentDir)
	if err == nil && len(files) == 0 {
		_ = os.Remove(parentDir)
	}

	return nil
}

// public path used in URLs and signatures
func (fs *FileStore) publicPath(key string) string {
	return path.Join(fs.urlPrefix, key)
}

// path returns the full path to the file by joining the
// base directory with the provided path
func (fs *FileStore) path(path string) string {
	return filepath.Join(fs.baseDir, path)
}

package store

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestNewFileStore(t *testing.T) {
	tcases := []struct {
		name        string
		baseDir     string
		secretKey   []byte
		expectError bool
	}{
		{
			name:        "valid baseDir and secretKey",
			baseDir:     t.TempDir() + "/testdir",
			secretKey:   []byte("test-key"),
			expectError: false,
		},
		{
			name:        "empty baseDir",
			baseDir:     "",
			secretKey:   []byte("test-key"),
			expectError: true,
		},
		{
			name:        "empty secret key",
			baseDir:     t.TempDir() + "/testdir",
			secretKey:   []byte(""),
			expectError: true,
		},
	}

	for _, tt := range tcases {
		t.Run(tt.name, func(t *testing.T) {
			store, err := NewFileStore(tt.baseDir, tt.secretKey)
			if tt.expectError {
				if err == nil {
					t.Fatalf("expected error but got none")
				}
				return
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}

			if store == nil {
				t.Fatal("expected store to be non-nil")
			}

			if store.secretKey == nil || !bytes.Equal(store.secretKey, tt.secretKey) {
				t.Fatalf("unexpected secret key: got %q, expected %q", store.secretKey, tt.secretKey)
			}

			if _, err := os.Stat(tt.baseDir); os.IsNotExist(err) {
				t.Fatalf("expected directory %q to exist, but it does not", tt.baseDir)
			}
		})
	}
}

func TestFileStore_GenerateURL(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := tempDir + "/testfile.txt"
		if err := os.WriteFile(filePath, []byte("test data"), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		store, err := NewFileStore(tempDir, []byte("test-key"))
		if err != nil {
			t.Fatalf("failed to create FileStore: %v", err)
		}

		url, err := store.GenerateURL(context.Background(), "testfile.txt", 15*time.Minute)
		if err != nil {
			t.Fatalf("unexpected error generating URL: %v", err)
		}

		if !strings.HasPrefix(url, "/") {
			t.Fatalf("unexpected URL format: got %q, expected it to start with '/'", url)
		}

		if !strings.Contains(url, "/uploads/testfile.txt") {
			t.Fatalf("unexpected URL: got %q, expected it to contain %q", url, "/uploads/testfile.txt")
		}

		// extract the expires and signature values from the URL for further validation
		parts := strings.Split(url, "?")
		if len(parts) != 2 {
			t.Fatalf("unexpected URL format: got %q, expected it to contain query parameters", url)
		}

		queryParams := parts[1]
		queryParts := strings.Split(queryParams, "&")
		if len(queryParts) != 2 {
			t.Fatalf("unexpected query parameters: got %q, expected 2", queryParts)
		}

		expires := queryParts[0]
		signature := queryParts[1]

		if !strings.HasPrefix(expires, "expires=") {
			t.Fatalf("unexpected expires parameter: got %q, expected it to start with 'expires='", expires)
		}

		if !strings.HasPrefix(signature, "signature=") {
			t.Fatalf("unexpected signature parameter: got %q, expected it to start with 'signature='", signature)
		}

		expiresValue := strings.TrimPrefix(expires, "expires=")
		signatureValue := strings.TrimPrefix(signature, "signature=")

		if expiresValue == "" {
			t.Fatalf("unexpected empty expires value")
		}

		if signatureValue == "" {
			t.Fatalf("unexpected empty signature value")
		}
	})

	t.Run("file does not exist", func(t *testing.T) {
		tempDir := t.TempDir()
		store, err := NewFileStore(tempDir, []byte("test-key"))
		if err != nil {
			t.Fatalf("failed to create FileStore: %v", err)
		}

		_, err = store.GenerateURL(context.Background(), "nonexistentfile.txt", 15*time.Minute)
		if err == nil {
			t.Fatal("expected error generating URL for nonexistent file, got none")
		}

		if !errors.Is(err, ErrNotExist) {
			t.Fatalf("unexpected error message: got %v, expected it to be %v", err, ErrNotExist)
		}
	})
}

func TestFileStore_Read(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := tempDir + "/testfile.txt"
		content := []byte("Hello, World!")

		if err := os.WriteFile(filePath, content, 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		store, err := NewFileStore(tempDir, []byte("test-key"))
		if err != nil {
			t.Fatalf("failed to create FileStore: %v", err)
		}

		reader, err := store.Read(context.Background(), "testfile.txt")
		if err != nil {
			t.Fatalf("unexpected error reading file: %v", err)
		}
		defer reader.Close()

		readContent := make([]byte, len(content))
		if _, err := reader.Read(readContent); err != nil {
			t.Fatalf("unexpected error reading from reader: %v", err)
		}

		if !bytes.Equal(readContent, content) {
			t.Fatalf("unexpected content: got %q, expected %q", readContent, content)
		}
	})

	t.Run("file does not exist", func(t *testing.T) {
		tempDir := t.TempDir()
		store, err := NewFileStore(tempDir, []byte("test-key"))
		if err != nil {
			t.Fatalf("failed to create FileStore: %v", err)
		}

		_, err = store.Read(context.Background(), "does/not/exist")
		if err == nil {
			t.Fatal("expected error, got none")
		}

		if !strings.Contains(err.Error(), "error opening file") {
			t.Fatalf("unexpected error message: got %v, expected it to contain %q", err, "error opening file")
		}
	})
}

func TestFileStore_Write(t *testing.T) {
	t.Run("successful write", func(t *testing.T) {
		tempDir := t.TempDir()
		store, err := NewFileStore(tempDir, []byte("test-secret"))
		if err != nil {
			t.Fatalf("failed to create FileStore: %v", err)
		}

		content := "test data"
		err = store.Write(context.Background(), "test/file", strings.NewReader(content))
		if err != nil {
			t.Fatalf("unexpected error writing file: %v", err)
		}

		// Verify the file was written correctly
		writtenContent, err := os.ReadFile(tempDir + "/test/file")
		if err != nil {
			t.Fatalf("failed to read written file: %v", err)
		}

		if string(writtenContent) != content {
			t.Fatalf("unexpected file content: got %q, expected %q", string(writtenContent), content)
		}
	})
}

func TestFileStore_Delete(t *testing.T) {
	t.Run("successful delete", func(t *testing.T) {
		tempDir := t.TempDir()
		store, err := NewFileStore(tempDir, []byte("test-secret"))
		if err != nil {
			t.Fatalf("failed to create FileStore: %v", err)
		}

		// Create a file to delete
		filePath := tempDir + "/test/file"
		if err := os.MkdirAll(tempDir+"/test", 0755); err != nil {
			t.Fatalf("failed to create directory: %v", err)
		}
		if err := os.WriteFile(filePath, []byte("test data"), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		err = store.Delete(context.Background(), "test/file")
		if err != nil {
			t.Fatalf("unexpected error deleting file: %v", err)
		}

		// Verify the file was deleted
		if _, err := os.Stat(filePath); !os.IsNotExist(err) {
			t.Fatalf("expected file to be deleted, but it still exists")
		}
	})
}

func Test_path(t *testing.T) {
	baseDir := t.TempDir()
	store, err := NewFileStore(baseDir, []byte("test-secret"))
	if err != nil {
		t.Fatalf("failed to create FileStore: %v", err)
	}

	expectedPath := baseDir + "/path/to/file"
	if path := store.path("path/to/file"); path != expectedPath {
		t.Fatalf("unexpected path: got %q, expected %q", path, expectedPath)
	}
}

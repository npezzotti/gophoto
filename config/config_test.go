package config

import (
	"os"
	"testing"
)

func TestLoadConfigFromEnv(t *testing.T) {
	tcases := []struct {
		name          string
		env           map[string]string
		expected      *Config
		expectingErr  bool
		expectedError string
	}{
		{
			name: "all env vars set",
			env: map[string]string{
				"GOPHOTO_HTTP_SERVER_ADDR": DefaultAddress,
				"GOPHOTO_DSN":              "postgres://user:pass@localhost:5432/dbname?sslmode=disable",
				"GOPHOTO_REDIS_ADDR":       "localhost:6379",
				"GOPHOTO_BASE_DIR":         DefaultBaseDir,
				"GOPHOTO_STORAGE_TYPE":     string(StorageTypeDisk),
				"GOPHOTO_BUCKET_NAME":      "my-bucket",
			},
			expected: &Config{
				HttpServerAddr: DefaultAddress,
				DatabaseSource: "postgres://user:pass@localhost:5432/dbname?sslmode=disable",
				RedisAddress:   "localhost:6379",
				BaseDir:        DefaultBaseDir,
				StorageType:    StorageTypeDisk,
				BucketName:     "my-bucket",
			},
		},
		{
			name: "missing database source",
			env: map[string]string{
				"GOPHOTO_HTTP_SERVER_ADDR": DefaultAddress,
				"GOPHOTO_REDIS_ADDR":       "localhost:6379",
				"GOPHOTO_BASE_DIR":         DefaultBaseDir,
				"GOPHOTO_STORAGE_TYPE":     string(StorageTypeDisk),
				"GOPHOTO_BUCKET_NAME":      "my-bucket",
			},
			expectingErr:  true,
			expectedError: "database source required",
		},
		{
			name: "missing redis address",
			env: map[string]string{
				"GOPHOTO_HTTP_SERVER_ADDR": DefaultAddress,
				"GOPHOTO_DSN":              "postgres://user:pass@localhost:5432/dbname?sslmode=disable",
				"GOPHOTO_BASE_DIR":         DefaultBaseDir,
				"GOPHOTO_STORAGE_TYPE":     string(StorageTypeDisk),
				"GOPHOTO_BUCKET_NAME":      "my-bucket",
			},
			expectingErr:  true,
			expectedError: "redis address required",
		},
		{
			name: "default base dir",
			env: map[string]string{
				"GOPHOTO_HTTP_SERVER_ADDR": DefaultAddress,
				"GOPHOTO_DSN":              "postgres://user:pass@localhost:5432/dbname?sslmode=disable",
				"GOPHOTO_REDIS_ADDR":       "localhost:6379",
				"GOPHOTO_STORAGE_TYPE":     string(StorageTypeDisk),
				"GOPHOTO_BUCKET_NAME":      "my-bucket",
			},
			expected: &Config{
				HttpServerAddr: DefaultAddress,
				DatabaseSource: "postgres://user:pass@localhost:5432/dbname?sslmode=disable",
				RedisAddress:   "localhost:6379",
				BaseDir:        DefaultBaseDir,
				StorageType:    StorageTypeDisk,
				BucketName:     "my-bucket",
			},
		},
		{
			name: "default storage type",
			env: map[string]string{
				"GOPHOTO_HTTP_SERVER_ADDR": DefaultAddress,
				"GOPHOTO_DSN":              "postgres://user:pass@localhost:5432/dbname?sslmode=disable",
				"GOPHOTO_REDIS_ADDR":       "localhost:6379",
				"GOPHOTO_BASE_DIR":         DefaultBaseDir,
			},
			expected: &Config{
				HttpServerAddr: DefaultAddress,
				DatabaseSource: "postgres://user:pass@localhost:5432/dbname?sslmode=disable",
				RedisAddress:   "localhost:6379",
				BaseDir:        DefaultBaseDir,
				StorageType:    StorageTypeDisk,
			},
		},
		{
			name: "missing bucket name with s3 storage type",
			env: map[string]string{
				"GOPHOTO_HTTP_SERVER_ADDR": DefaultAddress,
				"GOPHOTO_DSN":              "postgres://user:pass@localhost:5432/dbname?sslmode=disable",
				"GOPHOTO_REDIS_ADDR":       "localhost:6379",
				"GOPHOTO_BASE_DIR":         DefaultBaseDir,
				"GOPHOTO_STORAGE_TYPE":     string(StorageTypeS3),
			},
			expectingErr:  true,
			expectedError: "bucket name required for s3 storage",
		},
	}

	for _, tt := range tcases {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				os.Setenv(k, v)
			}

			t.Cleanup(func() {
				for k := range tt.env {
					os.Unsetenv(k)
				}
			})

			cfg, err := LoadConfigFromEnv()
			if err != nil && !tt.expectingErr {
				t.Fatalf("unexpected error when loading config: %v", err)
			} else if err == nil && tt.expectingErr {
				t.Fatal("expected error but got none")
			} else if err != nil && tt.expectingErr {
				if err.Error() != tt.expectedError {
					t.Fatalf("expected error %q, got %q", tt.expectedError, err.Error())
				}
				return
			}

			if cfg.HttpServerAddr != tt.expected.HttpServerAddr {
				t.Errorf("expected HttpServerAddr to be %q, got %q", tt.expected.HttpServerAddr, cfg.HttpServerAddr)
			}

			if cfg.DatabaseSource != tt.expected.DatabaseSource {
				t.Errorf("expected DatabaseSource to be %q, got %q", tt.expected.DatabaseSource, cfg.DatabaseSource)
			}

			if cfg.RedisAddress != tt.expected.RedisAddress {
				t.Errorf("expected RedisAddress to be %q, got %q", tt.expected.RedisAddress, cfg.RedisAddress)
			}

			if cfg.BaseDir != tt.expected.BaseDir {
				t.Errorf("expected BaseDir to be %q, got %q", tt.expected.BaseDir, cfg.BaseDir)
			}

			if cfg.StorageType != tt.expected.StorageType {
				t.Errorf("expected StorageType to be %q, got %q", tt.expected.StorageType, cfg.StorageType)
			}

			if cfg.BucketName != tt.expected.BucketName {
				t.Errorf("expected BucketName to be %q, got %q", tt.expected.BucketName, cfg.BucketName)
			}
		})
	}
}

package config

import (
	"errors"
	"os"
	"time"
)

type storageType string

const (
	StorageTypeDisk      storageType = "disk"
	StorageTypeS3        storageType = "s3"
	DefaultAddress                   = ":8800"
	DefaultBaseDir                   = "uploads"
	DefaultAssetsBaseUrl             = "/assets"
)

var DefaultSigningKey = []byte("default-signing-key")

type Config struct {
	StorageType      storageType
	DatabaseSource   string
	HttpServerAddr   string
	BaseDir          string
	AssetBaseURL     string
	BucketName       string
	UseTemplateCache bool
	RedisAddress     string
	RedisPassword    string
	SigningKey       []byte
	URLExpiry        time.Duration
	Debug            bool
}

func LoadConfigFromEnv() (*Config, error) {
	cfg := &Config{
		UseTemplateCache: os.Getenv("GOPHOTO_USE_TEMPLATE_CACHE") == "true",
		HttpServerAddr:   os.Getenv("GOPHOTO_HTTP_SERVER_ADDR"),
		DatabaseSource:   os.Getenv("GOPHOTO_DSN"),
		RedisAddress:     os.Getenv("GOPHOTO_REDIS_ADDR"),
		BaseDir:          os.Getenv("GOPHOTO_BASE_DIR"),
		StorageType:      storageType(os.Getenv("GOPHOTO_STORAGE_TYPE")),
		BucketName:       os.Getenv("GOPHOTO_BUCKET_NAME"),
		SigningKey:       []byte(os.Getenv("GOPHOTO_SIGNING_KEY")),
		AssetBaseURL:     os.Getenv("GOPHOTO_ASSET_BASE_URL"),
		Debug:            os.Getenv("GOPHOTO_DEBUG") == "true",
		URLExpiry:        15 * time.Minute,
	}

	if cfg.AssetBaseURL == "" {
		cfg.AssetBaseURL = DefaultAssetsBaseUrl
	}

	if cfg.HttpServerAddr == "" {
		cfg.HttpServerAddr = DefaultAddress
	}

	if cfg.DatabaseSource == "" {
		return cfg, errors.New("database source required")
	}

	if cfg.RedisAddress == "" {
		return cfg, errors.New("redis address required")
	}

	if cfg.BaseDir == "" {
		cfg.BaseDir = DefaultBaseDir
	}

	if cfg.StorageType == "" {
		cfg.StorageType = StorageTypeDisk
	}

	if cfg.StorageType == StorageTypeS3 && cfg.BucketName == "" {
		return cfg, errors.New("bucket name required for s3 storage")
	}

	if len(cfg.SigningKey) == 0 {
		cfg.SigningKey = DefaultSigningKey
	}

	return cfg, nil
}

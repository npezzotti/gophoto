package config

import (
	"errors"
	"os"
)

type storageType string

const (
	StorageTypeDisk storageType = "disk"
	StorageTypeS3   storageType = "s3"
	DefaultAddress              = ":8800"
	DefaultBaseDir              = "uploads"
)

type Config struct {
	StorageType      storageType
	DatabaseSource   string
	HttpServerAddr   string
	BaseDir          string
	StaticDir        string
	BucketName       string
	UseTemplateCache bool
	RedisAddress     string
	RedisPassword    string
}

func LoadConfigFromEnv() (*Config, error) {
	cfg := &Config{
		HttpServerAddr: os.Getenv("GOPHOTO_HTTP_SERVER_ADDR"),
		DatabaseSource: os.Getenv("GOPHOTO_DSN"),
		RedisAddress:   os.Getenv("GOPHOTO_REDIS_ADDR"),
		BaseDir:        os.Getenv("GOPHOTO_BASE_DIR"),
		StorageType:    storageType(os.Getenv("GOPHOTO_STORAGE_TYPE")),
		BucketName:     os.Getenv("GOPHOTO_BUCKET_NAME"),
	}

	if cfg.StaticDir == "" {
		cfg.StaticDir = "./assets"
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

	return cfg, nil
}

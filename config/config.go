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
)

type Config struct {
	StorageType      storageType
	DatabaseSource   string
	HttpServerAddr   string
	BaseDir          string
	BucketName       string
	UseTemplateCache bool
}

func LoadConfig() (*Config, error) {
	cfg := &Config{
		StorageType:    storageType(os.Getenv("GOPHOTO_STORAGE_TYPE")),
		DatabaseSource: os.Getenv("GOPHOTO_DATABASE_SOURCE"),
		HttpServerAddr: os.Getenv("GOPHOTO_HTTP_SERVER_ADDR"),
		BaseDir:        os.Getenv("GOPHOTO_BASE_DIR"),
		BucketName:     os.Getenv("GOPHOTO_BUCKET_NAME"),
	}

	if cfg.HttpServerAddr == "" {
		cfg.HttpServerAddr = DefaultAddress
	}

	if cfg.StorageType == "" {
		cfg.StorageType = StorageTypeDisk
	}

	if cfg.DatabaseSource == "" {
		return cfg, errors.New("database source required")
	}

	return cfg, nil
}

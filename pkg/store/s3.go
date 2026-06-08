package store

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type s3Client interface {
	DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
}

type s3Presigner interface {
	PresignGetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error)
}

type s3Uploader interface {
	Upload(ctx context.Context, input *s3.PutObjectInput, opts ...func(*manager.Uploader)) (*manager.UploadOutput, error)
}

type S3Store struct {
	BucketName     string
	client         s3Client
	presigner      s3Presigner
	uploader       s3Uploader
	expiryDuration time.Duration
}

func NewS3Store(bucketName string) (*S3Store, error) {
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion("us-east-1"))
	if err != nil {
		return nil, fmt.Errorf("unable to load SDK config: %w", err)
	}

	svc := s3.NewFromConfig(cfg)

	return &S3Store{
		client:     svc,
		presigner:  s3.NewPresignClient(svc),
		uploader:   manager.NewUploader(svc),
		BucketName: bucketName,
	}, nil
}

func (s *S3Store) GenerateURL(ctx context.Context, path string, expiry time.Duration) (string, error) {
	request, err := s.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.BucketName),
		Key:    aws.String(path),
	}, func(po *s3.PresignOptions) {
		po.Expires = expiry
	})
	if err != nil {
		return "", fmt.Errorf("error creating presign request for path %q: %w", path, err)
	}

	return request.URL, nil
}

func (s *S3Store) Write(ctx context.Context, path string, file io.Reader) error {
	_, err := s.uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.BucketName),
		Key:    aws.String(path),
		Body:   file,
	})
	if err != nil {
		return fmt.Errorf("error uploading file with path %q: %w", path, err)
	}

	return nil
}

func (s *S3Store) Delete(ctx context.Context, path string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.BucketName),
		Key:    aws.String(path),
	})
	if err != nil {
		return fmt.Errorf("error deleting object with path %q: %w", path, err)
	}

	return nil
}

func (s *S3Store) Read(ctx context.Context, path string) (io.ReadCloser, error) {
	resp, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.BucketName),
		Key:    aws.String(path),
	})
	if err != nil {
		return nil, fmt.Errorf("error getting object with path %q: %w", path, err)
	}

	return resp.Body, nil
}

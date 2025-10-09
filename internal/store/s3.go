package store

import (
	"context"
	"fmt"
	"io"
	"log"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go/aws"
)

type S3Store struct {
	BucketName string
	Client     *s3.Client
	Presigner  *s3.PresignClient
}

func NewS3Store(bucketName string) *S3Store {
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion("us-east-1"))
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	svc := s3.NewFromConfig(cfg)

	return &S3Store{
		Client:     svc,
		Presigner:  s3.NewPresignClient(svc),
		BucketName: bucketName,
	}
}

func (s *S3Store) GenerateURL(ctx context.Context, path string) (string, error) {
	request, err := s.Presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.BucketName),
		Key:    aws.String(path),
	}, func(po *s3.PresignOptions) {

	})
	if err != nil {
		return "", fmt.Errorf("error creating presign request: %w", err)
	}

	return request.URL, nil
}

func (s *S3Store) Write(ctx context.Context, path string, file io.Reader) error {
	uploader := manager.NewUploader(s.Client)

	_, err := uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.BucketName),
		Key:    aws.String(path),
		Body:   file,
	})
	if err != nil {
		return fmt.Errorf("error uploading file: %w", err)
	}

	return nil
}

func (s *S3Store) Delete(ctx context.Context, path string) error {
	_, err := s.Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.BucketName),
		Key:    aws.String(path),
	})

	return err
}

func (s *S3Store) Read(ctx context.Context, path string) (io.ReadCloser, error) {
	resp, err := s.Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.BucketName),
		Key:    aws.String(path),
	})
	if err != nil {
		return nil, fmt.Errorf("error getting object from S3: %w", err)
	}

	return resp.Body, nil
}

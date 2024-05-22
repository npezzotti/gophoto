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

func NewS3Store() *S3Store {
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("us-east-1"))
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	svc := s3.NewFromConfig(cfg)

	return &S3Store{
		Client:    svc,
		Presigner: s3.NewPresignClient(svc),
	}
}

func (s *S3Store) Read(key string) (string, error) {
	request, err := s.Presigner.PresignGetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(s.BucketName),
		Key:    aws.String(key),
	}, func(po *s3.PresignOptions) {

	})
	if err != nil {
		return "", fmt.Errorf("error creating presign request: %w", err)
	}

	return request.URL, nil
}

func (s *S3Store) Write(key string, file io.Reader) error {
	uploader := manager.NewUploader(s.Client)

	_, err := uploader.Upload(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(s.BucketName),
		Key:    aws.String(key),
		Body:   file,
	})
	if err != nil {
		return fmt.Errorf("error uploading file: %w", err)
	}

	return nil
}

func (s *S3Store) Delete(key string) error {
	_, err := s.Client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket: aws.String(s.BucketName),
		Key:    aws.String(key),
	})

	return err
}

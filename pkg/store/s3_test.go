package store

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type mockS3Presigner struct {
	request *v4.PresignedHTTPRequest
	err     error
	input   *s3.GetObjectInput
}

func (m *mockS3Presigner) PresignGetObject(_ context.Context, params *s3.GetObjectInput, _ ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error) {
	m.input = params
	return m.request, m.err
}

type mockS3Client struct {
	getOutput   *s3.GetObjectOutput
	getErr      error
	deleteErr   error
	getInput    *s3.GetObjectInput
	deleteInput *s3.DeleteObjectInput
}

func (m *mockS3Client) GetObject(_ context.Context, params *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	m.getInput = params
	return m.getOutput, m.getErr
}

func (m *mockS3Client) DeleteObject(_ context.Context, params *s3.DeleteObjectInput, _ ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	m.deleteInput = params
	return &s3.DeleteObjectOutput{}, m.deleteErr
}

type mockS3Uploader struct {
	err   error
	input *s3.PutObjectInput
}

func (m *mockS3Uploader) Upload(_ context.Context, input *s3.PutObjectInput, _ ...func(*manager.Uploader)) (*manager.UploadOutput, error) {
	m.input = input
	return &manager.UploadOutput{}, m.err
}

func TestS3Store_GenerateURL(t *testing.T) {
	t.Run("presign success", func(t *testing.T) {
		expectedURL := "https://example-bucket.com/photos/test.jpg"
		presigner := &mockS3Presigner{
			request: &v4.PresignedHTTPRequest{URL: expectedURL},
		}
		store := &S3Store{BucketName: "example-bucket", presigner: presigner}

		url, err := store.GenerateURL(context.Background(), "photos/test.jpg", 15*time.Minute)
		if err != nil {
			t.Fatalf("unexpected error generating URL: %v", err)
		}

		if url != expectedURL {
			t.Fatalf("unexpected url: got %q, expected %q", url, expectedURL)
		}

		if presigner.input == nil {
			t.Fatal("expected presign input to be captured")
		}

		if got := *presigner.input.Bucket; got != "example-bucket" {
			t.Fatalf("unexpected bucket: got %q", got)
		}

		if got := *presigner.input.Key; got != "photos/test.jpg" {
			t.Fatalf("unexpected key: got %q", got)
		}
	})

	t.Run("presign error", func(t *testing.T) {
		presigner := &mockS3Presigner{err: errors.New("an error occurred")}
		store := &S3Store{BucketName: "example-bucket", presigner: presigner}

		_, err := store.GenerateURL(context.Background(), "photos/test.jpg", 15*time.Minute)
		if err == nil {
			t.Fatal("expected error generating URL, got nil")
		}

		expectedErrMsg := "error creating presign request for path \"photos/test.jpg\": an error occurred"
		if !strings.Contains(err.Error(), expectedErrMsg) {
			t.Fatalf("unexpected error: got %v, expected %s", err, expectedErrMsg)
		}
	})
}

func TestS3Store_Write(t *testing.T) {
	t.Run("upload success", func(t *testing.T) {
		uploader := &mockS3Uploader{}
		store := &S3Store{BucketName: "example-bucket", uploader: uploader}

		err := store.Write(context.Background(), "photos/test.jpg", strings.NewReader("test data"))
		if err != nil {
			t.Fatalf("Write returned unexpected error: %v", err)
		}

		if uploader.input == nil {
			t.Fatal("expected upload input to be captured")
		}

		if got := *uploader.input.Bucket; got != "example-bucket" {
			t.Fatalf("unexpected bucket: got %q, expected %q", got, "example-bucket")
		}

		if got := *uploader.input.Key; got != "photos/test.jpg" {
			t.Fatalf("unexpected key: got %q, expected %q", got, "photos/test.jpg")
		}

		body, err := io.ReadAll(uploader.input.Body)
		if err != nil {
			t.Fatalf("failed reading captured body: %v", err)
		}

		if string(body) != "test data" {
			t.Fatalf("unexpected body: got %q, expected %q", string(body), "test data")
		}
	})

	t.Run("upload error", func(t *testing.T) {
		uploader := &mockS3Uploader{err: errors.New("an error occurred")}
		store := &S3Store{BucketName: "example-bucket", uploader: uploader}

		err := store.Write(context.Background(), "photos/test.jpg", strings.NewReader("test data"))
		if err == nil {
			t.Fatal("expected error")
		}

		expectedErrMsg := "error uploading file with path \"photos/test.jpg\": an error occurred"
		if !strings.Contains(err.Error(), expectedErrMsg) {
			t.Fatalf("unexpected error: got %v, expected %s", err, expectedErrMsg)
		}
	})
}

func TestS3Store_Read(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		client := &mockS3Client{
			getOutput: &s3.GetObjectOutput{Body: io.NopCloser(strings.NewReader("test data"))},
		}
		store := &S3Store{BucketName: "example-bucket", client: client}

		body, err := store.Read(context.Background(), "photos/test.jpg")
		if err != nil {
			t.Fatalf("Read returned unexpected error: %v", err)
		}
		defer body.Close()

		buf, err := io.ReadAll(body)
		if err != nil {
			t.Fatalf("failed reading body: %v", err)
		}

		if string(buf) != "test data" {
			t.Fatalf("unexpected body: got %q, expected %q", string(buf), "test data")
		}

		if client.getInput == nil {
			t.Fatal("expected get input to be captured")
		}

		if got := *client.getInput.Bucket; got != "example-bucket" {
			t.Fatalf("unexpected bucket: got %q, expected %q", got, "example-bucket")
		}

		if got := *client.getInput.Key; got != "photos/test.jpg" {
			t.Fatalf("unexpected key: got %q", got)
		}
	})

	t.Run("get object error", func(t *testing.T) {
		client := &mockS3Client{getErr: errors.New("an error occurred")}
		store := &S3Store{BucketName: "example-bucket", client: client}

		_, err := store.Read(context.Background(), "photos/test.jpg")
		if err == nil {
			t.Fatal("expected error")
		}

		expectedErrMsg := "error getting object with path \"photos/test.jpg\": an error occurred"
		if !strings.Contains(err.Error(), expectedErrMsg) {
			t.Fatalf("unexpected error: got %v, expected %s", err, expectedErrMsg)
		}
	})
}

func TestS3Store_Delete(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		client := &mockS3Client{}
		store := &S3Store{BucketName: "example-bucket", client: client}

		err := store.Delete(context.Background(), "photos/test.jpg")
		if err != nil {
			t.Fatalf("Delete returned unexpected error: %v", err)
		}

		if client.deleteInput == nil {
			t.Fatal("expected delete input to be captured")
		}

		if got := *client.deleteInput.Bucket; got != "example-bucket" {
			t.Fatalf("unexpected bucket: got %q, expected %q", got, "example-bucket")
		}

		if got := *client.deleteInput.Key; got != "photos/test.jpg" {
			t.Fatalf("unexpected key: got %q, expected %q", got, "photos/test.jpg")
		}
	})

	t.Run("delete error", func(t *testing.T) {
		client := &mockS3Client{deleteErr: errors.New("an error occurred")}
		store := &S3Store{BucketName: "example-bucket", client: client}

		err := store.Delete(context.Background(), "photos/test.jpg")
		if err == nil {
			t.Fatal("expected error")
		}

		expectedErrMsg := "error deleting object with path \"photos/test.jpg\": an error occurred"
		if !strings.Contains(err.Error(), expectedErrMsg) {
			t.Fatalf("unexpected error: got %v, expected %s", err, expectedErrMsg)
		}
	})
}

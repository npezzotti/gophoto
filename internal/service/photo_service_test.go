package service

import (
	"bytes"
	"context"
	"errors"
	"mime/multipart"
	"testing"

	"github.com/npezzotti/gophoto/internal/db"
	"github.com/npezzotti/gophoto/internal/domain"
)

func ptrInt32(i int32) *int32 {
	return &i
}

func newMultiPartFile(t *testing.T) (multipart.File, *multipart.FileHeader) {
	t.Helper()
	var body bytes.Buffer
	multpartWriter := multipart.NewWriter(&body)
	formData, err := multpartWriter.CreateFormFile(FormFileName, "test.png")
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}

	png1x1 := []byte{137, 80, 78, 71, 13, 10, 26, 10, 0, 0, 0, 13, 73, 72, 68, 82, 0, 0, 0, 1, 0, 0, 0, 1, 8, 2, 0, 0, 0, 144, 119, 83, 222, 0, 0, 0, 12, 73, 68, 65, 84, 8, 215, 99, 248, 15, 4, 0, 9, 251, 3, 253, 167, 26, 102, 2, 0, 0, 0, 0, 73, 69, 78, 68, 174, 66, 96, 130}
	if _, err := formData.Write(png1x1); err != nil {
		t.Fatalf("failed to write image payload: %v", err)
	}

	if err := multpartWriter.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	multipartReader := multipart.NewReader(&body, multpartWriter.Boundary())
	form, err := multipartReader.ReadForm(int64(body.Len()))
	if err != nil {
		t.Fatalf("failed to read multipart form: %v", err)
	}
	t.Cleanup(func() {
		form.RemoveAll()
	})

	files := form.File[FormFileName]
	if len(files) == 0 {
		t.Fatalf("no files found in multipart form")
	}

	fh := files[0]
	f, err := fh.Open()
	if err != nil {
		t.Fatalf("failed to open multipart file: %v", err)
	}

	return f, fh
}

func TestPhotoService_GetPhoto(t *testing.T) {
	tcases := []struct {
		name           string
		photoRepoStub  *photoRepoStub
		id             int32
		expectedErr    error
		expectedErrMsg string
	}{
		{
			name: "Photo found",
			photoRepoStub: &photoRepoStub{
				getPhotoByIDFn: func(ctx context.Context, id int32) (domain.Photo, error) {
					return domain.Photo{ID: id}, nil
				},
			},
			id: 1,
		},
		{
			name: "Photo not found",
			photoRepoStub: &photoRepoStub{
				getPhotoByIDFn: func(ctx context.Context, id int32) (domain.Photo, error) {
					return domain.Photo{}, domain.ErrPhotoNotFound
				},
			},
			id:          2,
			expectedErr: domain.ErrPhotoNotFound,
		},
		{
			name: "Unexpected error",
			photoRepoStub: &photoRepoStub{
				getPhotoByIDFn: func(ctx context.Context, id int32) (domain.Photo, error) {
					return domain.Photo{}, errors.New("unexpected error")
				},
			},
			id:             3,
			expectedErrMsg: "error getting photo: unexpected error",
		},
	}

	for _, tt := range tcases {
		t.Run(tt.name, func(t *testing.T) {
			photoSvc := NewPhotoService(tt.photoRepoStub, nil, nil, nil, newTestLogger())
			photo, err := photoSvc.GetPhoto(context.Background(), tt.id)
			if tt.expectedErr != nil {
				if !errors.Is(err, tt.expectedErr) {
					t.Fatalf("expected error %v, got %v", tt.expectedErr, err)
				}
				return
			}
			if tt.expectedErrMsg != "" {
				if err == nil || err.Error() != tt.expectedErrMsg {
					t.Fatalf("expected error message %q, got %v", tt.expectedErrMsg, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if photo.ID != tt.id {
				t.Fatalf("expected photo ID %d, got %d", tt.id, photo.ID)
			}
		})
	}
}

func TestPhotoService_GetAlbumPhoto(t *testing.T) {
	tcases := []struct {
		name           string
		photoRepoStub  *photoRepoStub
		id             int32
		expectedErr    error
		expectedErrMsg string
	}{
		{
			name: "Album photo found",
			photoRepoStub: &photoRepoStub{
				getAlbumPhotoByIDFn: func(ctx context.Context, id int32) (domain.AlbumPhoto, error) {
					return domain.AlbumPhoto{ID: id}, nil
				},
			},
			id: 1,
		},
		{
			name: "Album photo not found",
			photoRepoStub: &photoRepoStub{
				getAlbumPhotoByIDFn: func(ctx context.Context, id int32) (domain.AlbumPhoto, error) {
					return domain.AlbumPhoto{}, db.ErrAlbumPhotoNotFound
				},
			},
			id:          2,
			expectedErr: domain.ErrAlbumPhotoNotFound,
		},
		{
			name: "Unexpected error",
			photoRepoStub: &photoRepoStub{
				getAlbumPhotoByIDFn: func(ctx context.Context, id int32) (domain.AlbumPhoto, error) {
					return domain.AlbumPhoto{}, errors.New("unexpected error")
				},
			},
			id:             3,
			expectedErrMsg: "error getting album photo: unexpected error",
		},
	}

	for _, tt := range tcases {
		t.Run(tt.name, func(t *testing.T) {
			photoSvc := NewPhotoService(tt.photoRepoStub, nil, nil, nil, newTestLogger())
			photo, err := photoSvc.GetAlbumPhoto(context.Background(), tt.id)
			if tt.expectedErr != nil {
				if !errors.Is(err, tt.expectedErr) {
					t.Fatalf("expected error %v, got %v", tt.expectedErr, err)
				}
				return
			}
			if tt.expectedErrMsg != "" {
				if err == nil || err.Error() != tt.expectedErrMsg {
					t.Fatalf("expected error message %q, got %v", tt.expectedErrMsg, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if photo.ID != tt.id {
				t.Fatalf("expected album photo ID %d, got %d", tt.id, photo.ID)
			}
		})
	}
}

func TestPhotoService_CreateAlbumPhotoWithOriginalMetadata(t *testing.T) {
	tcases := []struct {
		name          string
		photoRepoStub *photoRepoStub
		albumRepoStub *albumRepoStub
		storeStub     *storeStub
		queueStub     *queuePublisherStub
		userID        int32
		albumID       int32
		expectedErr   error
	}{
		{
			name: "Successful creation",
			photoRepoStub: &photoRepoStub{
				createAlbumPhotoWithOriginalMetadataFn: func(ctx context.Context, albumID int32, cmd domain.CreatePhotoWithOriginalMetadataParams) (domain.Photo, error) {
					return domain.Photo{
						ID:     42,
						UserID: ptrInt32(cmd.UserID),
						Key:    cmd.Key,
					}, nil
				},
			},
			albumRepoStub: &albumRepoStub{
				getAlbumByIDFn: func(ctx context.Context, id int32) (domain.Album, error) {
					return domain.Album{ID: id, UserID: 1}, nil
				},
			},
			storeStub: &storeStub{},
			queueStub: &queuePublisherStub{},
			userID:    1,
			albumID:   7,
		},
	}

	for _, tt := range tcases {
		t.Run(tt.name, func(t *testing.T) {
			photoSvc := NewPhotoService(tt.photoRepoStub, tt.albumRepoStub, tt.storeStub, tt.queueStub, newTestLogger())
			multipartFile, fileHeader := newMultiPartFile(t)
			photo, err := photoSvc.CreateAlbumPhotoWithOriginalMetadata(context.Background(), multipartFile, fileHeader, tt.userID, tt.albumID)
			if tt.expectedErr != nil {
				if !errors.Is(err, tt.expectedErr) {
					t.Fatalf("expected error %v, got %v", tt.expectedErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if photo.ID != 42 {
				t.Fatalf("expected photo ID 42, got %d", photo.ID)
			}
			if tt.storeStub.lastWrittenKey == "" {
				t.Fatalf("expected photo to be written to storage")
			}
			if tt.queueStub.publishCalls != 1 {
				t.Fatalf("expected one publish call, got %d", tt.queueStub.publishCalls)
			}
		})
	}
}

func TestPhotoService_CreateUserPhotoWithOriginalMetadata(t *testing.T) {
	tcases := []struct {
		name          string
		photoRepoStub *photoRepoStub
		albumRepoStub *albumRepoStub
		storeStub     *storeStub
		queueStub     *queuePublisherStub
		userID        int32
		expectedErr   error
	}{
		{
			name: "Successful creation",
			photoRepoStub: &photoRepoStub{
				createUserPhotoWithOriginalMetadataFn: func(ctx context.Context, userID int32, cmd domain.CreatePhotoWithOriginalMetadataParams) (domain.Photo, error) {
					return domain.Photo{
						ID:     42,
						UserID: ptrInt32(cmd.UserID),
						Key:    cmd.Key,
					}, nil
				},
			},
			albumRepoStub: &albumRepoStub{},
			storeStub:     &storeStub{},
			queueStub:     &queuePublisherStub{},
			userID:        1,
		},
	}

	for _, tt := range tcases {
		t.Run(tt.name, func(t *testing.T) {
			photoSvc := NewPhotoService(tt.photoRepoStub, tt.albumRepoStub, tt.storeStub, tt.queueStub, newTestLogger())
			multipartFile, fileHeader := newMultiPartFile(t)
			photo, err := photoSvc.CreateUserPhotoWithOriginalMetadata(context.Background(), multipartFile, fileHeader, tt.userID)
			if tt.expectedErr != nil {
				if !errors.Is(err, tt.expectedErr) {
					t.Fatalf("expected error %v, got %v", tt.expectedErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if photo.ID != 42 {
				t.Fatalf("expected photo ID 42, got %d", photo.ID)
			}
			if photo.UserID == nil || *photo.UserID != tt.userID {
				t.Fatalf("expected photo UserID %d, got %v", tt.userID, photo.UserID)
			}
			if tt.storeStub.lastWrittenKey == "" {
				t.Fatalf("expected photo to be written to storage")
			}
			if tt.queueStub.publishCalls != 1 {
				t.Fatalf("expected one publish call, got %d", tt.queueStub.publishCalls)
			}
		})
	}
}

func TestRemovePhotoFromAlbum(t *testing.T) {
	tcases := []struct {
		name           string
		photoRepoStub  *photoRepoStub
		photoID        int32
		userID         int32
		expectedErr    error
		expectedErrMsg string
	}{
		{
			name: "Successful removal",
			photoRepoStub: &photoRepoStub{
				getAlbumPhotoByIDFn: func(ctx context.Context, photoID int32) (domain.AlbumPhoto, error) {
					return domain.AlbumPhoto{ID: photoID, AlbumID: 6, UserID: ptrInt32(5)}, nil
				},
				removePhotoFromAlbumFn: func(ctx context.Context, albumId int32, photoId int32) error {
					return nil
				},
			},
			photoID: 1,
			userID:  5,
		},
		{
			name: "Album photo not found",
			photoRepoStub: &photoRepoStub{
				getAlbumPhotoByIDFn: func(ctx context.Context, photoID int32) (domain.AlbumPhoto, error) {
					return domain.AlbumPhoto{}, db.ErrAlbumPhotoNotFound
				},
			},
			photoID:     2,
			userID:      5,
			expectedErr: domain.ErrAlbumPhotoNotFound,
		},
	}

	for _, tt := range tcases {
		t.Run(tt.name, func(t *testing.T) {
			photoSvc := NewPhotoService(tt.photoRepoStub, &albumRepoStub{}, &storeStub{}, &queuePublisherStub{}, newTestLogger())
			err := photoSvc.RemovePhotoFromAlbum(context.Background(), tt.photoID, tt.userID)
			if tt.expectedErr != nil {
				if !errors.Is(err, tt.expectedErr) {
					t.Fatalf("expected error %v, got %v", tt.expectedErr, err)
				}
				return
			}
			if tt.expectedErrMsg != "" {
				if err == nil || err.Error() != tt.expectedErrMsg {
					t.Fatalf("expected error message %q, got %v", tt.expectedErrMsg, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

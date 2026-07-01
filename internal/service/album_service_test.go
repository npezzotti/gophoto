package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/npezzotti/gophoto/internal/config"
	"github.com/npezzotti/gophoto/internal/db"
	"github.com/npezzotti/gophoto/internal/domain"
)

func TestAlbumService_GetAlbumByID(t *testing.T) {
	tcases := []struct {
		name           string
		albumRepoStub  *albumRepoStub
		albumID        int32
		expectedAlbum  domain.Album
		expectedErr    error
		expectedErrStr string
	}{
		{
			name: "Album exists",
			albumRepoStub: &albumRepoStub{
				getAlbumByIDFn: func(ctx context.Context, id int32) (domain.Album, error) {
					return domain.Album{ID: id}, nil
				},
			},
			albumID:       1,
			expectedAlbum: domain.Album{ID: 1},
		},
		{
			name: "Album does not exist",
			albumRepoStub: &albumRepoStub{
				getAlbumByIDFn: func(ctx context.Context, id int32) (domain.Album, error) {
					return domain.Album{}, db.ErrAlbumNotFound
				},
			},
			albumID:     2,
			expectedErr: domain.ErrAlbumNotFound,
		},
		{
			name: "Error getting album",
			albumRepoStub: &albumRepoStub{
				getAlbumByIDFn: func(ctx context.Context, id int32) (domain.Album, error) {
					return domain.Album{}, errors.New("internal error")
				},
			},
			albumID:        3,
			expectedErrStr: "error getting album with ID 3: internal error",
		},
	}

	for _, tt := range tcases {
		t.Run(tt.name, func(t *testing.T) {
			albumSvc := NewAlbumService(tt.albumRepoStub, &photoRepoStub{}, &storeStub{}, nil, newTestLogger())
			album, err := albumSvc.GetAlbumByID(context.Background(), tt.albumID)
			if tt.expectedErr != nil {
				if !errors.Is(err, tt.expectedErr) {
					t.Fatalf("expected %v error, got %v", tt.expectedErr, err)
				}
				return
			}
			if tt.expectedErrStr != "" {
				if err == nil || err.Error() != tt.expectedErrStr {
					t.Fatalf("expected error string '%s', got '%v'", tt.expectedErrStr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if album.ID != tt.albumID {
				t.Fatalf("expected album ID to be %d, got %d", tt.albumID, album.ID)
			}
		})
	}
}

func TestAlbumService_GetAlbumPageView(t *testing.T) {
	photoKey1 := uuid.New().String()
	photoKey2 := uuid.New().String()
	photoKey3 := uuid.New().String()

	albumRepoStub := &albumRepoStub{
		getAlbumByIDFn: func(ctx context.Context, id int32) (domain.Album, error) {
			return domain.Album{ID: 1, UserID: 1, Title: "Test Album", NumPhotos: 3}, nil
		},
		listAlbumPhotoViewRowsFn: func(ctx context.Context, albumID, limit, offset int32) ([]domain.AlbumPhotoViewRow, error) {
			return []domain.AlbumPhotoViewRow{
				{PhotoID: 1, PhotoKey: photoKey1, Variant: domain.PhotoVariantOriginal, MimeType: string(domain.MimeTypeJPEG)},
				{PhotoID: 1, PhotoKey: photoKey1, Variant: domain.PhotoVariantLarge, Width: 1600, Height: 1200, MimeType: string(domain.MimeTypeJPEG)},
				{PhotoID: 1, PhotoKey: photoKey1, Variant: domain.PhotoVariantMedium, Width: 800, Height: 600, MimeType: string(domain.MimeTypeJPEG)},
				{PhotoID: 2, PhotoKey: photoKey2, Variant: domain.PhotoVariantOriginal, MimeType: string(domain.MimeTypeJPEG)},
				{PhotoID: 2, PhotoKey: photoKey2, Variant: domain.PhotoVariantLarge, Width: 1600, Height: 1200, MimeType: string(domain.MimeTypeJPEG)},
				{PhotoID: 2, PhotoKey: photoKey2, Variant: domain.PhotoVariantSmall, Width: 400, Height: 300, MimeType: string(domain.MimeTypeJPEG)},
				{PhotoID: 3, PhotoKey: photoKey3, Variant: domain.PhotoVariantOriginal, MimeType: string(domain.MimeTypeJPEG)},
				{PhotoID: 3, PhotoKey: photoKey3, Variant: domain.PhotoVariantLarge, Width: 1600, Height: 1200, MimeType: string(domain.MimeTypeJPEG)},
				{PhotoID: 3, PhotoKey: photoKey3, Variant: domain.PhotoVariantThumb, Width: 200, Height: 150, MimeType: string(domain.MimeTypeJPEG)},
			}, nil
		},
	}
	albumSvc := NewAlbumService(
		albumRepoStub,
		&photoRepoStub{},
		&storeStub{
			generateURLFn: func(ctx context.Context, key string, expiry time.Duration) (string, error) {
				return fmt.Sprintf("https://example.test/%s", key), nil
			},
		},
		&config.Config{URLExpiry: 10 * time.Minute},
		newTestLogger(),
	)
	albumPageView, err := albumSvc.GetAlbumPageView(context.Background(), 1, 1, 10, 0)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if albumPageView.Album.ID != 1 {
		t.Fatalf("expected album ID to be 1, got %d", albumPageView.Album.ID)
	}
	if albumPageView.Album.Title != "Test Album" {
		t.Fatalf("expected album title to be 'Test Album', got '%s'", albumPageView.Album.Title)
	}
	if albumPageView.TotalPhotos != 3 {
		t.Fatalf("expected total photos to be 3, got %d", albumPageView.TotalPhotos)
	}
	if albumPageView.Album.ID != 1 {
		t.Fatalf("expected album ID to be 1, got %d", albumPageView.Album.ID)
	}
	if len(albumPageView.Photos) != 3 {
		t.Fatalf("expected 3 photos, got %d", len(albumPageView.Photos))
	}
}

func TestUserService_CreateAlbum(t *testing.T) {
	tcases := []struct {
		name           string
		albumRepoStub  *albumRepoStub
		userID         int32
		title          string
		expectedAlbum  domain.Album
		expectedErr    error
		expectedErrStr string
	}{
		{
			name: "Successful album creation",
			albumRepoStub: &albumRepoStub{
				createAlbumFn: func(ctx context.Context, userID int32, title string) (domain.Album, error) {
					return domain.Album{ID: 1, UserID: userID, Title: title, CoverPhotoID: ptrInt32(2), NumPhotos: 3}, nil
				},
			},
			userID:        1,
			title:         "New Album",
			expectedAlbum: domain.Album{ID: 1, UserID: 1, Title: "New Album", CoverPhotoID: ptrInt32(2), NumPhotos: 3},
		},
	}

	for _, tt := range tcases {
		t.Run(tt.name, func(t *testing.T) {
			albumSvc := NewAlbumService(tt.albumRepoStub, &photoRepoStub{}, &storeStub{}, nil, newTestLogger())
			album, err := albumSvc.CreateAlbum(context.Background(), tt.userID, tt.title)
			if tt.expectedErr != nil {
				if !errors.Is(err, tt.expectedErr) {
					t.Fatalf("expected %v error, got %v", tt.expectedErr, err)
				}
				return
			}
			if tt.expectedErrStr != "" {
				if err == nil || err.Error() != tt.expectedErrStr {
					t.Fatalf("expected error string %q, got %v", tt.expectedErrStr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if album.ID != tt.expectedAlbum.ID {
				t.Fatalf("expected album ID to be %d, got %d", tt.expectedAlbum.ID, album.ID)
			}
			if album.UserID != tt.expectedAlbum.UserID {
				t.Fatalf("expected album UserID to be %d, got %d", tt.expectedAlbum.UserID, album.UserID)
			}
			if album.Title != tt.expectedAlbum.Title {
				t.Fatalf("expected album Title to be %q, got %q", tt.expectedAlbum.Title, album.Title)
			}
			if album.CoverPhotoID != nil && tt.expectedAlbum.CoverPhotoID != nil && *album.CoverPhotoID != *tt.expectedAlbum.CoverPhotoID {
				t.Fatalf("expected album CoverPhotoID to be %d, got %d", *tt.expectedAlbum.CoverPhotoID, *album.CoverPhotoID)
			}
			if album.NumPhotos != tt.expectedAlbum.NumPhotos {
				t.Fatalf("expected album NumPhotos to be %d, got %d", tt.expectedAlbum.NumPhotos, album.NumPhotos)
			}
		})
	}
}

func TestUserService_UpdateAlbum(t *testing.T) {
	tcases := []struct {
		name           string
		albumRepoStub  *albumRepoStub
		userID         int32
		albumID        int32
		title          string
		expectedAlbum  domain.Album
		expectedErr    error
		expectedErrStr string
	}{
		{
			name: "Successful album update",
			albumRepoStub: &albumRepoStub{
				getAlbumByIDFn: func(ctx context.Context, id int32) (domain.Album, error) {
					return domain.Album{ID: id, UserID: 1, Title: "Old Title"}, nil
				},
				updateAlbumFn: func(ctx context.Context, albumID int32, userID int32, title string) (domain.Album, error) {
					return domain.Album{ID: albumID, UserID: userID, Title: title}, nil
				},
			},
			userID:        1,
			albumID:       1,
			title:         "New Title",
			expectedAlbum: domain.Album{ID: 1, UserID: 1, Title: "New Title"},
		},
	}

	for _, tt := range tcases {
		t.Run(tt.name, func(t *testing.T) {
			albumSvc := NewAlbumService(tt.albumRepoStub, &photoRepoStub{}, &storeStub{}, nil, newTestLogger())
			updatedAlbum, err := albumSvc.UpdateAlbum(context.Background(), tt.albumID, tt.userID, tt.title)
			if tt.expectedErr != nil {
				if !errors.Is(err, tt.expectedErr) {
					t.Fatalf("expected %v, got %v", tt.expectedErr, err)
				}
				return
			}
			if tt.expectedErrStr != "" {
				if err == nil || err.Error() != tt.expectedErrStr {
					t.Fatalf("expected error string %q, got %v", tt.expectedErrStr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if updatedAlbum.ID != tt.expectedAlbum.ID {
				t.Fatalf("expected album ID to be %d, got %d", tt.expectedAlbum.ID, updatedAlbum.ID)
			}
			if updatedAlbum.UserID != tt.expectedAlbum.UserID {
				t.Fatalf("expected album user ID to be %d, got %d", tt.expectedAlbum.UserID, updatedAlbum.UserID)
			}
			if updatedAlbum.Title != tt.expectedAlbum.Title {
				t.Fatalf("expected album title to be %s, got %s", tt.expectedAlbum.Title, updatedAlbum.Title)
			}
		})
	}
}

func TestUserService_DeleteAlbum(t *testing.T) {
	tcases := []struct {
		name           string
		albumRepoStub  *albumRepoStub
		userID         int32
		albumID        int32
		expectedErr    error
		expectedErrStr string
	}{
		{
			name: "Successful album deletion",
			albumRepoStub: &albumRepoStub{
				getAlbumByIDFn: func(ctx context.Context, id int32) (domain.Album, error) {
					return domain.Album{ID: id, UserID: 1}, nil
				},
				deleteAlbumFn: func(ctx context.Context, albumID int32) error {
					if albumID != 1 {
						return errors.New("unexpected album ID")
					}
					return nil
				},
			},
			userID:  1,
			albumID: 1,
		},
	}

	for _, tt := range tcases {
		t.Run(tt.name, func(t *testing.T) {
			albumSvc := NewAlbumService(tt.albumRepoStub, &photoRepoStub{}, &storeStub{}, nil, newTestLogger())
			err := albumSvc.DeleteAlbum(context.Background(), tt.albumID, tt.userID)
			if tt.expectedErr != nil {
				if !errors.Is(err, tt.expectedErr) {
					t.Fatalf("expected %v, got %v", tt.expectedErr, err)
				}
				return
			}
			if tt.expectedErrStr != "" {
				if err == nil || err.Error() != tt.expectedErrStr {
					t.Fatalf("expected error string %q, got %v", tt.expectedErrStr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

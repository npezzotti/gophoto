package db

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/npezzotti/gophoto/internal/domain"
)

type mockDB struct {
	beginTxFn func(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
}

func (m *mockDB) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	if m.beginTxFn != nil {
		return m.beginTxFn(ctx, opts)
	}
	return nil, nil
}

type mockQuerier struct {
	getPhotoFn                            func(ctx context.Context, id int32) (Photo, error)
	deletePhotoFn                         func(ctx context.Context, photoID int32) error
	getOrphanedPhotosFn                   func(ctx context.Context) ([]Photo, error)
	updatePhotoStatusFn                   func(ctx context.Context, arg UpdatePhotoStatusParams) error
	getPhotoMetadataByPhotoIDAndVariantFn func(ctx context.Context, arg GetPhotoMetadataByPhotoIDAndVariantParams) (PhotoMetadatum, error)
	getAlbumPhotoFn                       func(ctx context.Context, id int32) (GetAlbumPhotoRow, error)
}

func (m *mockQuerier) GetPhoto(ctx context.Context, id int32) (Photo, error) {
	if m.getPhotoFn != nil {
		return m.getPhotoFn(ctx, id)
	}
	return Photo{}, nil
}

func (m *mockQuerier) WithTx(_ *sql.Tx) querier { return m }
func (m *mockQuerier) DeletePhoto(ctx context.Context, photoID int32) error {
	if m.deletePhotoFn != nil {
		return m.deletePhotoFn(ctx, photoID)
	}
	return nil
}
func (m *mockQuerier) GetOrphanedPhotos(_ context.Context) ([]Photo, error) {
	if m.getOrphanedPhotosFn != nil {
		return m.getOrphanedPhotosFn(context.Background())
	}
	return nil, nil
}
func (m *mockQuerier) UpdatePhotoStatus(ctx context.Context, arg UpdatePhotoStatusParams) error {
	if m.updatePhotoStatusFn != nil {
		return m.updatePhotoStatusFn(ctx, arg)
	}
	return nil
}
func (m *mockQuerier) GetPhotoMetadataByPhotoIDAndVariant(ctx context.Context, arg GetPhotoMetadataByPhotoIDAndVariantParams) (PhotoMetadatum, error) {
	if m.getPhotoMetadataByPhotoIDAndVariantFn != nil {
		return m.getPhotoMetadataByPhotoIDAndVariantFn(ctx, arg)
	}
	return PhotoMetadatum{}, nil
}
func (m *mockQuerier) GetAlbumPhoto(ctx context.Context, id int32) (GetAlbumPhotoRow, error) {
	if m.getAlbumPhotoFn != nil {
		return m.getAlbumPhotoFn(ctx, id)
	}
	return GetAlbumPhotoRow{}, nil
}
func (m *mockQuerier) GetPhotoMetadataByPhotoID(_ context.Context, _ int32) ([]PhotoMetadatum, error) {
	return nil, nil
}
func (m *mockQuerier) createPhotoWithOriginalMetadata(_ context.Context, _ CreatePhotoWithOriginalMetadataParams) (Photo, error) {
	return Photo{}, nil
}
func (m *mockQuerier) AddPhotoToAlbumWithCover(_ context.Context, _ AddPhotoToAlbumParams) (Album, error) {
	return Album{}, nil
}
func (m *mockQuerier) IncrementAlbumPhotoCount(_ context.Context, _ IncrementAlbumPhotoCountParams) error {
	return nil
}
func (m *mockQuerier) UpdateUserProfilePicture(_ context.Context, _ UpdateUserProfilePictureParams) (User, error) {
	return User{}, nil
}
func (m *mockQuerier) DeleteAlbumPhoto(_ context.Context, _ DeleteAlbumPhotoParams) (int32, error) {
	return 0, nil
}
func (m *mockQuerier) DecrementAlbumPhotoCount(_ context.Context, _ DecrementAlbumPhotoCountParams) error {
	return nil
}
func (m *mockQuerier) GetLastPhotoFromAlbum(_ context.Context, _ int32) (AlbumPhoto, error) {
	return AlbumPhoto{}, nil
}
func (m *mockQuerier) GetAlbumById(_ context.Context, _ int32) (GetAlbumByIdRow, error) {
	return GetAlbumByIdRow{}, nil
}
func (m *mockQuerier) SetAlbumCoverPhoto(_ context.Context, _ SetAlbumCoverPhotoParams) error {
	return nil
}
func (m *mockQuerier) CreatePhotoMetadata(_ context.Context, _ CreatePhotoMetadataParams) (PhotoMetadatum, error) {
	return PhotoMetadatum{}, nil
}
func (m *mockQuerier) GetUserById(_ context.Context, _ int32) (GetUserByIdRow, error) {
	return GetUserByIdRow{}, nil
}
func (m *mockQuerier) GetUserByEmail(_ context.Context, _ string) (User, error) { return User{}, nil }
func (m *mockQuerier) CreateUser(_ context.Context, _ CreateUserParams) (User, error) {
	return User{}, nil
}
func (m *mockQuerier) UpdateUser(_ context.Context, _ UpdateUserParams) (User, error) {
	return User{}, nil
}
func (m *mockQuerier) DeleteUser(_ context.Context, _ int32) error { return nil }
func (m *mockQuerier) CreateAlbum(_ context.Context, _ CreateAlbumParams) (Album, error) {
	return Album{}, nil
}
func (m *mockQuerier) UpdateAlbum(_ context.Context, _ UpdateAlbumParams) (Album, error) {
	return Album{}, nil
}
func (m *mockQuerier) DeleteAlbum(_ context.Context, _ int32) error { return nil }
func (m *mockQuerier) ListAlbumPhotoViewRows(_ context.Context, _ ListAlbumPhotoViewRowsParams) ([]ListAlbumPhotoViewRowsRow, error) {
	return nil, nil
}
func (m *mockQuerier) ListAlbumsByUser(_ context.Context, _ ListAlbumsByUserParams) ([]ListAlbumsByUserRow, error) {
	return nil, nil
}

func TestRepositoryGetPhoto(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	userID := int32(42)

	tcases := []struct {
		name      string
		getPhoto  func(ctx context.Context, id int32) (Photo, error)
		inputID   int32
		wantPhoto domain.Photo
		wantErr   error
	}{
		{
			name: "returns photo with non-null user_id",
			getPhoto: func(_ context.Context, _ int32) (Photo, error) {
				return Photo{
					ID:        1,
					UserID:    sql.NullInt32{Int32: 42, Valid: true},
					Key:       "photos/abc.jpg",
					Status:    PhotoStatusProcessed,
					CreatedAt: fixedTime,
					UpdatedAt: fixedTime,
				}, nil
			},
			inputID: 1,
			wantPhoto: domain.Photo{
				ID:        1,
				UserID:    &userID,
				Key:       "photos/abc.jpg",
				Status:    domain.PhotoStatusProcessed,
				CreatedAt: fixedTime,
				UpdatedAt: fixedTime,
			},
		},
		{
			name: "returns photo with null user ID",
			getPhoto: func(_ context.Context, _ int32) (Photo, error) {
				return Photo{
					ID:        2,
					UserID:    sql.NullInt32{},
					Key:       "photos/def.jpg",
					Status:    PhotoStatusProcessing,
					CreatedAt: fixedTime,
					UpdatedAt: fixedTime,
				}, nil
			},
			inputID: 2,
			wantPhoto: domain.Photo{
				ID:        2,
				UserID:    nil,
				Key:       "photos/def.jpg",
				Status:    domain.PhotoStatusProcessing,
				CreatedAt: fixedTime,
				UpdatedAt: fixedTime,
			},
		},
		{
			name: "photo not found",
			getPhoto: func(_ context.Context, _ int32) (Photo, error) {
				return Photo{}, sql.ErrNoRows
			},
			inputID: 99,
			wantErr: ErrPhotoNotFound,
		},
	}

	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &Repository{querier: &mockQuerier{getPhotoFn: tc.getPhoto}}

			got, err := repo.GetPhoto(context.Background(), tc.inputID)

			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("expected error %v, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got.ID != tc.wantPhoto.ID {
				t.Errorf("got photo with ID %d, want %d", got.ID, tc.wantPhoto.ID)
			}
			if got.Key != tc.wantPhoto.Key {
				t.Errorf("got photo with key %q, want %q", got.Key, tc.wantPhoto.Key)
			}
			if got.Status != tc.wantPhoto.Status {
				t.Errorf("git photo with status %q, want %q", got.Status, tc.wantPhoto.Status)
			}
			if got.CreatedAt != tc.wantPhoto.CreatedAt {
				t.Errorf("got photo created at %v, want %v", got.CreatedAt, tc.wantPhoto.CreatedAt)
			}
			if got.UpdatedAt != tc.wantPhoto.UpdatedAt {
				t.Errorf("got photo updated at %v, want %v", got.UpdatedAt, tc.wantPhoto.UpdatedAt)
			}
			switch {
			case tc.wantPhoto.UserID == nil && got.UserID != nil:
				t.Errorf("got user%v, want nil", *got.UserID)
			case tc.wantPhoto.UserID != nil && got.UserID == nil:
				t.Errorf("UserID: got nil, want %v", *tc.wantPhoto.UserID)
			case tc.wantPhoto.UserID != nil && got.UserID != nil && *got.UserID != *tc.wantPhoto.UserID:
				t.Errorf("UserID: got %d, want %d", *got.UserID, *tc.wantPhoto.UserID)
			}
		})
	}
}

func TestRepositoryDeletePhoto(t *testing.T) {
	tcases := []struct {
		name          string
		deletePhotoFn func(ctx context.Context, id int32) error
		inputID       int32
		errMatch      string
	}{
		{
			name: "successfully deletes photo",
			deletePhotoFn: func(_ context.Context, _ int32) error {
				return nil
			},
			inputID: 1,
		},
		{
			name: "surfaces error deleting photo",
			deletePhotoFn: func(_ context.Context, _ int32) error {
				return errors.New("internal error")
			},
			inputID:  1,
			errMatch: "internal error",
		},
	}

	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &Repository{querier: &mockQuerier{deletePhotoFn: tc.deletePhotoFn}}

			err := repo.DeletePhoto(context.Background(), tc.inputID)

			if tc.errMatch != "" {
				if err == nil || !strings.Contains(err.Error(), tc.errMatch) {
					t.Fatalf("expected error %q, got %v", tc.errMatch, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestRepositoryGetOrphanedPhotos(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	userID := int32(42)

	tcases := []struct {
		name                string
		getOrphanedPhotosFn func(_ context.Context) ([]Photo, error)
		expectedPhotos      []domain.Photo
		errMatch            string
	}{
		{
			name: "successful request returns some orphaned photos",
			getOrphanedPhotosFn: func(_ context.Context) ([]Photo, error) {
				return []Photo{
					{
						ID:        1,
						UserID:    sql.NullInt32{Valid: true, Int32: userID},
						Key:       "test-key-1",
						Status:    PhotoStatusProcessed,
						CreatedAt: fixedTime,
						UpdatedAt: fixedTime,
					}, {
						ID:        2,
						UserID:    sql.NullInt32{Valid: true, Int32: userID},
						Key:       "test-key-2",
						Status:    PhotoStatusProcessed,
						CreatedAt: fixedTime,
						UpdatedAt: fixedTime,
					},
				}, nil
			},
			expectedPhotos: []domain.Photo{
				{
					ID:        1,
					UserID:    &userID,
					Key:       "test-key-1",
					Status:    domain.PhotoStatusProcessed,
					CreatedAt: fixedTime,
					UpdatedAt: fixedTime,
				},
				{
					ID:        2,
					UserID:    &userID,
					Key:       "test-key-2",
					Status:    domain.PhotoStatusProcessed,
					CreatedAt: fixedTime,
					UpdatedAt: fixedTime,
				},
			},
		},
		{
			name: "no orphaned photos found",
			getOrphanedPhotosFn: func(_ context.Context) ([]Photo, error) {
				return nil, nil
			},
		},
		{
			name: "error querying photos",
			getOrphanedPhotosFn: func(_ context.Context) ([]Photo, error) {
				return nil, errors.New("internal error")
			},
			errMatch: "internal error",
		},
	}

	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &Repository{querier: &mockQuerier{getOrphanedPhotosFn: tc.getOrphanedPhotosFn}}

			photos, err := repo.GetOrphanedPhotos(context.Background())

			if tc.errMatch != "" {
				if err == nil || !strings.Contains(err.Error(), tc.errMatch) {
					t.Fatalf("expected error %q, got %v", tc.errMatch, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !reflect.DeepEqual(photos, tc.expectedPhotos) {
				t.Errorf("photos mismatch:\n got  %+v\n want %+v", photos, tc.expectedPhotos)
			}
		})
	}
}

func TestRepositoryUpdatePhotoStatus(t *testing.T) {
	tcases := []struct {
		name                string
		updatePhotoStatusFn func(ctx context.Context, arg UpdatePhotoStatusParams) error
		errMatch            string
	}{
		{
			name: "successful update to photo status",
			updatePhotoStatusFn: func(_ context.Context, _ UpdatePhotoStatusParams) error {
				return nil
			},
		},
		{
			name: "error updating photo status",
			updatePhotoStatusFn: func(_ context.Context, _ UpdatePhotoStatusParams) error {
				return errors.New("internal error")
			},
			errMatch: "internal error",
		},
	}

	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &Repository{querier: &mockQuerier{updatePhotoStatusFn: tc.updatePhotoStatusFn}}

			err := repo.UpdatePhotoStatus(context.Background(), 1, domain.PhotoStatusProcessed)
			if tc.errMatch != "" {
				if err == nil || !strings.Contains(err.Error(), tc.errMatch) {
					t.Fatalf("expected error %q, got %v", tc.errMatch, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestRepositoryGetPhotoMetadataByPhotoIDAndVariant(t *testing.T) {
	tcases := []struct {
		name                                  string
		getPhotoMetadataByPhotoIDAndVariantFn func(ctx context.Context, arg GetPhotoMetadataByPhotoIDAndVariantParams) (PhotoMetadatum, error)
		photoID                               int32
		variant                               domain.PhotoVariant
		expectedMeta                          domain.PhotoMetadatum
		wantErr                               error
		errMatch                              string
	}{
		{
			name: "success",
			getPhotoMetadataByPhotoIDAndVariantFn: func(_ context.Context, arg GetPhotoMetadataByPhotoIDAndVariantParams) (PhotoMetadatum, error) {
				return PhotoMetadatum{
					ID:      1,
					PhotoID: arg.PhotoID,
					Variant: arg.Variant,
				}, nil
			},
			photoID: 1,
			variant: domain.PhotoVariantLarge,
			expectedMeta: domain.PhotoMetadatum{
				ID:      1,
				PhotoID: 1,
				Variant: domain.PhotoVariantLarge,
			},
		},
		{
			name: "photo not found",
			getPhotoMetadataByPhotoIDAndVariantFn: func(ctx context.Context, arg GetPhotoMetadataByPhotoIDAndVariantParams) (PhotoMetadatum, error) {
				return PhotoMetadatum{}, sql.ErrNoRows
			},
			wantErr: ErrPhotoNotFound,
		},
		{
			name: "error querying metadata",
			getPhotoMetadataByPhotoIDAndVariantFn: func(_ context.Context, arg GetPhotoMetadataByPhotoIDAndVariantParams) (PhotoMetadatum, error) {
				return PhotoMetadatum{}, errors.New("internal error")
			},
			errMatch: "internal error",
		},
	}

	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &Repository{querier: &mockQuerier{getPhotoMetadataByPhotoIDAndVariantFn: tc.getPhotoMetadataByPhotoIDAndVariantFn}}
			meta, err := repo.GetPhotoMetadataByPhotoIDAndVariant(context.Background(), tc.photoID, tc.variant)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("expected error %v, got %v", tc.wantErr, err)
				}
				return
			}
			if tc.errMatch != "" {
				if err == nil || !strings.Contains(err.Error(), tc.errMatch) {
					t.Fatalf("expected error %q, got %v", tc.errMatch, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(meta, tc.expectedMeta) {
				t.Errorf("photos mismatch:\n got  %+v\n want %+v", meta, tc.expectedMeta)
			}
		})
	}
}

func TestRepositoryGetAlbumPhoto(t *testing.T) {
	tcases := []struct {
		name                                  string
		getPhotoMetadataByPhotoIDAndVariantFn func(ctx context.Context, arg GetPhotoMetadataByPhotoIDAndVariantParams) (PhotoMetadatum, error)
		photoID                               int32
		variant                               domain.PhotoVariant
		expectedMeta                          domain.PhotoMetadatum
		wantErr                               error
		errMatch                              string
	}{
		{
			name: "success",
			getPhotoMetadataByPhotoIDAndVariantFn: func(_ context.Context, arg GetPhotoMetadataByPhotoIDAndVariantParams) (PhotoMetadatum, error) {
				return PhotoMetadatum{
					ID:      1,
					PhotoID: arg.PhotoID,
					Variant: arg.Variant,
				}, nil
			},
			photoID: 1,
			variant: domain.PhotoVariantLarge,
			expectedMeta: domain.PhotoMetadatum{
				ID:      1,
				PhotoID: 1,
				Variant: domain.PhotoVariantLarge,
			},
		},
		{
			name: "photo not found",
			getPhotoMetadataByPhotoIDAndVariantFn: func(ctx context.Context, arg GetPhotoMetadataByPhotoIDAndVariantParams) (PhotoMetadatum, error) {
				return PhotoMetadatum{}, sql.ErrNoRows
			},
			wantErr: ErrPhotoNotFound,
		},
		{
			name: "error querying metadata",
			getPhotoMetadataByPhotoIDAndVariantFn: func(_ context.Context, arg GetPhotoMetadataByPhotoIDAndVariantParams) (PhotoMetadatum, error) {
				return PhotoMetadatum{}, errors.New("internal error")
			},
			errMatch: "internal error",
		},
	}

	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &Repository{querier: &mockQuerier{getPhotoMetadataByPhotoIDAndVariantFn: tc.getPhotoMetadataByPhotoIDAndVariantFn}}
			meta, err := repo.GetPhotoMetadataByPhotoIDAndVariant(context.Background(), tc.photoID, tc.variant)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("expected error %v, got %v", tc.wantErr, err)
				}
				return
			}
			if tc.errMatch != "" {
				if err == nil || !strings.Contains(err.Error(), tc.errMatch) {
					t.Fatalf("expected error %q, got %v", tc.errMatch, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(meta, tc.expectedMeta) {
				t.Errorf("metadata mismatch:\n got  %+v\n want %+v", meta, tc.expectedMeta)
			}
		})
	}
}

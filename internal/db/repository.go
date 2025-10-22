package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type Repository struct {
	db *sql.DB
	*Queries
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{
		db:      db,
		Queries: New(db),
	}
}

func (q *Queries) AddPhotoToAlbumWithCover(ctx context.Context, arg AddPhotoToAlbumParams) error {
	album, err := q.GetAlbum(ctx, arg.AlbumID)
	if err != nil {
		return fmt.Errorf("get album: %v", err)
	}

	albumPhoto, err := q.AddPhotoToAlbum(ctx, arg)
	if err != nil {
		return fmt.Errorf("add photo to album: %v", err)
	}

	// Increment the album's photo count
	if err := q.IncrementAlbumPhotoCount(ctx, IncrementAlbumPhotoCountParams{
		ID:        album.ID,
		UpdatedAt: time.Now(),
	}); err != nil {
		return fmt.Errorf("increment album photo count: %v", err)
	}

	if !album.CoverPhotoID.Valid {
		if err := q.SetAlbumCoverPhoto(ctx, SetAlbumCoverPhotoParams{
			ID:           album.ID,
			CoverPhotoID: sql.NullInt32{Int32: albumPhoto.ID, Valid: true},
			UpdatedAt:    time.Now(),
		}); err != nil {
			return fmt.Errorf("set album cover photo: %v", err)
		}
	}

	return nil
}

type CreatePhotoWithOriginalMetadataParams struct {
	UserID   sql.NullInt32
	Key      string
	Width    int32
	Height   int32
	FileSize sql.NullInt64
	MimeType string
}

func (q *Queries) createPhotoWithOriginalMetadata(ctx context.Context, arg CreatePhotoWithOriginalMetadataParams) (Photo, error) {
	photo, err := q.CreatePhoto(ctx, CreatePhotoParams{
		UserID: arg.UserID,
		Key:    arg.Key,
		Status: PhotoStatusProcessing,
	})
	if err != nil {
		return Photo{}, fmt.Errorf("create photo: %v", err)
	}

	if _, err := q.CreatePhotoMetadata(ctx, CreatePhotoMetadataParams{
		PhotoID:  photo.ID,
		Variant:  PhotoVariantOriginal,
		Width:    arg.Width,
		Height:   arg.Height,
		FileSize: sql.NullInt64{Int64: arg.FileSize.Int64, Valid: arg.FileSize.Valid},
		MimeType: arg.MimeType,
	}); err != nil {
		return Photo{}, fmt.Errorf("create photo metadata: %v", err)
	}

	return photo, nil
}

func (r *Repository) CreateAlbumPhotoWithOriginalMetadata(ctx context.Context, albumId int32, arg CreatePhotoWithOriginalMetadataParams) (Photo, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Photo{}, err
	}

	q := r.WithTx(tx)

	photo, err := q.createPhotoWithOriginalMetadata(ctx, arg)
	if err != nil {
		if err := tx.Rollback(); err != nil {
			return Photo{}, fmt.Errorf("create photo with original metadata: %v, unable to rollback: %v", err, err)
		}
		return Photo{}, fmt.Errorf("create photo with original metadata: %v", err)
	}

	if err = q.AddPhotoToAlbumWithCover(ctx, AddPhotoToAlbumParams{
		PhotoID: photo.ID,
		AlbumID: albumId,
	}); err != nil {
		if err := tx.Rollback(); err != nil {
			return Photo{}, fmt.Errorf("add photo to album with cover: %v, unable to rollback: %v", err, err)
		}
		return Photo{}, fmt.Errorf("add photo to album with cover: %v", err)
	}

	if err = tx.Commit(); err != nil {
		if err := tx.Rollback(); err != nil {
			return Photo{}, fmt.Errorf("commit tx: %v, unable to rollback: %v", err, err)
		}
		return Photo{}, fmt.Errorf("commit tx: %v", err)
	}

	return photo, nil
}

func (r *Repository) CreateUserPhotoWithOriginalMetadata(ctx context.Context, user *GetUserByIdRow, arg CreatePhotoWithOriginalMetadataParams) (Photo, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Photo{}, err
	}

	q := r.WithTx(tx)

	photo, err := q.createPhotoWithOriginalMetadata(ctx, arg)
	if err != nil {
		if err := tx.Rollback(); err != nil {
			return Photo{}, fmt.Errorf("create photo with original metadata: %v, unable to rollback: %v", err, err)
		}
		return Photo{}, fmt.Errorf("create photo with original metadata: %v", err)
	}

	if _, err = q.UpdateUser(ctx, UpdateUserParams{
		ID:               user.ID,
		FirstName:        user.FirstName,
		LastName:         user.LastName,
		Email:            user.Email,
		PasswordHash:     user.PasswordHash,
		ProfilePictureID: sql.NullInt32{Int32: photo.ID, Valid: true},
		UpdatedAt:        time.Now(),
	}); err != nil {
		if err := tx.Rollback(); err != nil {
			return Photo{}, fmt.Errorf("update user profile picture: %v, unable to rollback: %v", err, err)
		}
		return Photo{}, fmt.Errorf("update user profile picture: %v", err)
	}

	if err = tx.Commit(); err != nil {
		if err := tx.Rollback(); err != nil {
			return Photo{}, fmt.Errorf("commit tx: %v, unable to rollback: %v", err, err)
		}
		return Photo{}, fmt.Errorf("commit tx: %v", err)
	}

	return photo, nil
}

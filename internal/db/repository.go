package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/npezzotti/gophoto/internal/domain"
)

var (
	ErrUserNotFound          = errors.New("user not found")
	ErrAlbumNotFound         = errors.New("album not found")
	ErrPhotoNotFound         = errors.New("photo not found")
	ErrPhotoMetadataNotFound = errors.New("photo metadata not found")
	ErrAlbumPhotoNotFound    = errors.New("album photo not found")
)

type PhotoRepository interface {
	GetPhoto(ctx context.Context, id int32) (domain.Photo, error)
	GetAlbumPhoto(ctx context.Context, id int32) (domain.AlbumPhoto, error)
	GetPhotoMetadataByPhotoID(ctx context.Context, photoId int32) ([]domain.PhotoMetadatum, error)
	CreateAlbumPhotoWithOriginalMetadata(ctx context.Context, albumID int32, cmd domain.CreatePhotoWithOriginalMetadataParams) (domain.Photo, error)
	CreateUserPhotoWithOriginalMetadata(ctx context.Context, cmd domain.CreatePhotoWithOriginalMetadataParams) (domain.Photo, error)
	RemovePhotoFromAlbum(ctx context.Context, albumId int32, photoId int32) error
	CreatePhotoMetadata(ctx context.Context, arg domain.CreatePhotoMetadataParams) (domain.PhotoMetadatum, error)
	GetPhotoMetadataByPhotoIDAndVariant(ctx context.Context, photoID int32, variant domain.PhotoVariant) (domain.PhotoMetadatum, error)
	UpdatePhotoStatus(ctx context.Context, photoID int32, status domain.PhotoStatus) error
	DeletePhoto(ctx context.Context, photoID int32) error
	GetOrphanedPhotos(ctx context.Context) ([]domain.Photo, error)
}

type AlbumRepository interface {
	GetAlbumByID(ctx context.Context, id int32) (domain.Album, error)
	ListAlbumsByUser(ctx context.Context, userID int32, limit, offset int32) ([]domain.AlbumListItem, error)
	ListAlbumPhotoViewRows(ctx context.Context, albumID, limit, offset int32) ([]domain.AlbumPhotoViewRow, error)
	CreateAlbum(ctx context.Context, userID int32, title string) (domain.Album, error)
	UpdateAlbum(ctx context.Context, albumId int32, userID int32, title string) (domain.Album, error)
	DeleteAlbum(ctx context.Context, albumId int32) error
}

type UserRepository interface {
	GetUserById(ctx context.Context, id int32) (domain.User, error)
	GetUserByEmail(ctx context.Context, email string) (domain.User, error)
	CreateUser(ctx context.Context, firstName, lastName, email, passwordHash string) (domain.User, error)
	UpdateUser(ctx context.Context, params domain.UserUpdateParams) (domain.User, error)
	DeleteUser(ctx context.Context, userID int32) error
}

type Repository struct {
	db      *sql.DB
	querier *Queries
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{
		db:      db,
		querier: New(db),
	}
}

func (r *Repository) GetPhoto(ctx context.Context, id int32) (domain.Photo, error) {
	photo, err := r.querier.GetPhoto(ctx, id)
	if err != nil {
		return domain.Photo{}, err
	}
	return domain.Photo{
		ID:        photo.ID,
		UserID:    nullInt32Ptr(photo.UserID),
		Key:       photo.Key,
		Status:    domain.PhotoStatus(photo.Status),
		CreatedAt: photo.CreatedAt,
		UpdatedAt: photo.UpdatedAt,
	}, nil
}

func (r *Repository) DeletePhoto(ctx context.Context, photoID int32) error {
	return r.querier.DeletePhoto(ctx, photoID)
}

func (r *Repository) GetOrphanedPhotos(ctx context.Context) ([]domain.Photo, error) {
	photos, err := r.querier.GetOrphanedPhotos(ctx)
	if err != nil {
		return nil, err
	}

	var result []domain.Photo
	for _, photo := range photos {
		result = append(result, domain.Photo{
			ID:        photo.ID,
			UserID:    nullInt32Ptr(photo.UserID),
			Key:       photo.Key,
			Status:    domain.PhotoStatus(photo.Status),
			CreatedAt: photo.CreatedAt,
			UpdatedAt: photo.UpdatedAt,
		})
	}
	return result, nil
}

func (r *Repository) UpdatePhotoStatus(ctx context.Context, photoID int32, status domain.PhotoStatus) error {
	return r.querier.UpdatePhotoStatus(ctx, UpdatePhotoStatusParams{
		ID:        photoID,
		Status:    PhotoStatus(status),
		UpdatedAt: time.Now(),
	})
}

func (r *Repository) GetPhotoMetadataByPhotoIDAndVariant(ctx context.Context, photoID int32, variant domain.PhotoVariant) (domain.PhotoMetadatum, error) {
	metadata, err := r.querier.GetPhotoMetadataByPhotoIDAndVariant(ctx, GetPhotoMetadataByPhotoIDAndVariantParams{
		PhotoID: photoID,
		Variant: PhotoVariant(variant),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.PhotoMetadatum{}, ErrPhotoNotFound
		}
		return domain.PhotoMetadatum{}, err
	}

	return domain.PhotoMetadatum{
		ID:        metadata.ID,
		PhotoID:   metadata.PhotoID,
		Variant:   domain.PhotoVariant(metadata.Variant),
		Width:     metadata.Width,
		Height:    metadata.Height,
		FileSize:  metadata.FileSize,
		MimeType:  metadata.MimeType,
		CreatedAt: metadata.CreatedAt,
	}, nil
}

func (r *Repository) GetAlbumPhoto(ctx context.Context, id int32) (domain.AlbumPhoto, error) {
	photo, err := r.querier.GetAlbumPhoto(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.AlbumPhoto{}, ErrAlbumPhotoNotFound
		}
		return domain.AlbumPhoto{}, err
	}
	return domain.AlbumPhoto{
		ID:        photo.ID,
		UserID:    nullInt32Ptr(photo.UserID),
		Key:       photo.Key,
		Status:    domain.PhotoStatus(photo.Status),
		CreatedAt: photo.CreatedAt,
		UpdatedAt: photo.UpdatedAt,
		AlbumID:   photo.AlbumID,
	}, nil
}

func (r *Repository) GetPhotoMetadataByPhotoID(ctx context.Context, photoId int32) ([]domain.PhotoMetadatum, error) {
	metadata, err := r.querier.GetPhotoMetadataByPhotoID(ctx, photoId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrPhotoMetadataNotFound
		}
		return nil, err
	}

	var result []domain.PhotoMetadatum
	for _, m := range metadata {
		result = append(result, domain.PhotoMetadatum{
			ID:        m.ID,
			PhotoID:   m.PhotoID,
			Variant:   domain.PhotoVariant(m.Variant),
			Width:     m.Width,
			Height:    m.Height,
			FileSize:  m.FileSize,
			MimeType:  m.MimeType,
			CreatedAt: m.CreatedAt,
		})
	}
	return result, nil
}

func (q *Queries) AddPhotoToAlbumWithCover(ctx context.Context, arg AddPhotoToAlbumParams) (Album, error) {
	album, err := q.GetAlbumById(ctx, arg.AlbumID)
	if err != nil {
		return Album{}, fmt.Errorf("get album: %w", err)
	}

	albumPhoto, err := q.AddPhotoToAlbum(ctx, arg)
	if err != nil {
		return Album{}, fmt.Errorf("add photo to album: %w", err)
	}

	// Increment the album's photo count
	if err := q.IncrementAlbumPhotoCount(ctx, IncrementAlbumPhotoCountParams{
		ID:        album.ID,
		UpdatedAt: time.Now(),
	}); err != nil {
		return Album{}, fmt.Errorf("increment album photo count: %w", err)
	}

	var updatedAlbum Album
	if !album.CoverPhotoID.Valid {
		if err := q.SetAlbumCoverPhoto(ctx, SetAlbumCoverPhotoParams{
			ID:           album.ID,
			CoverPhotoID: sql.NullInt32{Int32: albumPhoto.ID, Valid: true},
			UpdatedAt:    time.Now(),
		}); err != nil {
			return Album{}, fmt.Errorf("set album cover photo: %w", err)
		}
	}

	return updatedAlbum, nil
}

type CreatePhotoWithOriginalMetadataParams struct {
	UserID   sql.NullInt32
	Key      string
	Width    int32
	Height   int32
	FileSize int64
	MimeType string
}

func toCreatePhotoWithOriginalMetadataParams(arg domain.CreatePhotoWithOriginalMetadataParams) CreatePhotoWithOriginalMetadataParams {
	return CreatePhotoWithOriginalMetadataParams{
		UserID:   sql.NullInt32{Int32: arg.UserID, Valid: true},
		Key:      arg.Key,
		Width:    arg.Width,
		Height:   arg.Height,
		FileSize: arg.FileSize,
		MimeType: arg.MimeType,
	}
}

func (r *Repository) CreateAlbumPhotoWithOriginalMetadata(ctx context.Context, albumId int32, arg domain.CreatePhotoWithOriginalMetadataParams) (domain.Photo, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Photo{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	q := r.querier.WithTx(tx)

	photo, err := q.createPhotoWithOriginalMetadata(ctx, toCreatePhotoWithOriginalMetadataParams(arg))
	if err != nil {
		return domain.Photo{}, fmt.Errorf("create photo with original metadata: %w", err)
	}

	if _, err = q.AddPhotoToAlbumWithCover(ctx, AddPhotoToAlbumParams{
		PhotoID: photo.ID,
		AlbumID: albumId,
	}); err != nil {
		return domain.Photo{}, fmt.Errorf("add photo to album with cover: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return domain.Photo{}, fmt.Errorf("commit transaction: %w", err)
	}

	return domain.Photo{
		ID:        photo.ID,
		UserID:    nullInt32Ptr(photo.UserID),
		Key:       photo.Key,
		Status:    domain.PhotoStatus(photo.Status),
		CreatedAt: photo.CreatedAt,
		UpdatedAt: photo.UpdatedAt,
	}, nil
}

func (q *Queries) createPhotoWithOriginalMetadata(ctx context.Context, arg CreatePhotoWithOriginalMetadataParams) (Photo, error) {
	photo, err := q.CreatePhoto(ctx, CreatePhotoParams{
		UserID: arg.UserID,
		Key:    arg.Key,
		Status: PhotoStatusProcessing,
	})
	if err != nil {
		return Photo{}, fmt.Errorf("create photo: %w", err)
	}

	if _, err := q.CreatePhotoMetadata(ctx, CreatePhotoMetadataParams{
		PhotoID:  photo.ID,
		Variant:  PhotoVariantOriginal,
		Width:    arg.Width,
		Height:   arg.Height,
		FileSize: arg.FileSize,
		MimeType: arg.MimeType,
	}); err != nil {
		return Photo{}, fmt.Errorf("create photo metadata: %w", err)
	}

	return photo, nil
}

func (r *Repository) CreateUserPhotoWithOriginalMetadata(ctx context.Context, arg domain.CreatePhotoWithOriginalMetadataParams) (domain.Photo, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Photo{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	q := r.querier.WithTx(tx)

	photo, err := q.createPhotoWithOriginalMetadata(ctx, toCreatePhotoWithOriginalMetadataParams(arg))
	if err != nil {
		return domain.Photo{}, fmt.Errorf("create photo with original metadata: %w", err)
	}

	if _, err := q.UpdateUserProfilePicture(ctx, UpdateUserProfilePictureParams{
		ID:               arg.UserID,
		ProfilePictureID: sql.NullInt32{Int32: photo.ID, Valid: true},
		UpdatedAt:        time.Now(),
	}); err != nil {
		return domain.Photo{}, fmt.Errorf("update user profile picture: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return domain.Photo{}, fmt.Errorf("commit transaction: %w", err)
	}

	return domain.Photo{
		ID:        photo.ID,
		UserID:    nullInt32Ptr(photo.UserID),
		Key:       photo.Key,
		Status:    domain.PhotoStatus(photo.Status),
		CreatedAt: photo.CreatedAt,
		UpdatedAt: photo.UpdatedAt,
	}, nil
}

func (r *Repository) RemovePhotoFromAlbum(ctx context.Context, albumID int32, photoID int32) error {
	updateTime := time.Now()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	q := r.querier.WithTx(tx)

	album, err := q.GetAlbumById(ctx, albumID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrAlbumNotFound
		}
		return fmt.Errorf("get album by ID: %w", err)
	}

	deletedAlbumPhotoID, err := q.DeleteAlbumPhoto(ctx, DeleteAlbumPhotoParams{
		AlbumID: albumID,
		PhotoID: photoID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrAlbumPhotoNotFound
		}
		return fmt.Errorf("delete photo from album: %w", err)
	}

	if err := q.DecrementAlbumPhotoCount(ctx, DecrementAlbumPhotoCountParams{ID: albumID, UpdatedAt: updateTime}); err != nil {
		return fmt.Errorf("decrement album photo count: %w", err)
	}

	// If the photo being removed is the album's cover, we need to update the album's cover
	// to a different photo or remove the cover altogether.
	var newCoverAlbumPhotoID sql.NullInt32
	if album.CoverPhotoID.Valid && album.CoverPhotoID.Int32 == deletedAlbumPhotoID {
		// Get the most recent photo in the album as the new cover (if there are any photos left)
		newCoverPhoto, err := q.GetLastPhotoFromAlbum(ctx, albumID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				// No photos left in the album, so just remove the cover photo
				newCoverAlbumPhotoID = sql.NullInt32{Valid: false}
			} else {
				return fmt.Errorf("get most recent photo from album: %w", err)
			}
		} else {
			newCoverAlbumPhotoID = sql.NullInt32{Int32: newCoverPhoto.ID, Valid: true}
		}

		if err := q.SetAlbumCoverPhoto(ctx, SetAlbumCoverPhotoParams{
			ID:           albumID,
			CoverPhotoID: newCoverAlbumPhotoID,
			UpdatedAt:    updateTime,
		}); err != nil {
			return fmt.Errorf("update album cover photo: %w", err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func (r *Repository) CreatePhotoMetadata(ctx context.Context, arg domain.CreatePhotoMetadataParams) (domain.PhotoMetadatum, error) {
	metadata, err := r.querier.CreatePhotoMetadata(ctx, CreatePhotoMetadataParams{
		PhotoID:  arg.PhotoID,
		Variant:  PhotoVariant(arg.Variant),
		Width:    arg.Width,
		Height:   arg.Height,
		FileSize: arg.FileSize,
		MimeType: arg.MimeType,
	})
	if err != nil {
		return domain.PhotoMetadatum{}, fmt.Errorf("create photo metadata: %w", err)
	}

	return domain.PhotoMetadatum{
		ID:        metadata.ID,
		PhotoID:   metadata.PhotoID,
		Variant:   domain.PhotoVariant(metadata.Variant),
		Width:     metadata.Width,
		Height:    metadata.Height,
		FileSize:  metadata.FileSize,
		MimeType:  metadata.MimeType,
		CreatedAt: metadata.CreatedAt,
	}, nil
}

func (r *Repository) GetUserById(ctx context.Context, id int32) (domain.User, error) {
	user, err := r.querier.GetUserById(ctx, id)
	if err != nil {
		return domain.User{}, err
	}

	return domain.User{
		ID:               user.ID,
		FirstName:        user.FirstName,
		LastName:         user.LastName,
		Email:            user.Email,
		PasswordHash:     user.PasswordHash,
		ProfilePictureID: nullInt32Ptr(user.ProfilePictureID),
	}, nil
}

func (r *Repository) GetUserByEmail(ctx context.Context, email string) (domain.User, error) {
	user, err := r.querier.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.User{}, ErrUserNotFound
		}
		return domain.User{}, err
	}

	return domain.User{
		ID:               user.ID,
		FirstName:        user.FirstName,
		LastName:         user.LastName,
		Email:            user.Email,
		PasswordHash:     user.PasswordHash,
		ProfilePictureID: nullInt32Ptr(user.ProfilePictureID),
	}, nil
}

func (r *Repository) CreateUser(ctx context.Context, firstName, lastName, email, passwordHash string) (domain.User, error) {
	user, err := r.querier.CreateUser(ctx, CreateUserParams{
		FirstName:    firstName,
		LastName:     lastName,
		Email:        email,
		PasswordHash: passwordHash,
	})
	if err != nil {
		return domain.User{}, err
	}

	return domain.User{
		ID:           user.ID,
		FirstName:    user.FirstName,
		LastName:     user.LastName,
		Email:        user.Email,
		PasswordHash: user.PasswordHash,
	}, nil
}

// UpdateUser updates a user's profile information.
func (r *Repository) UpdateUser(ctx context.Context, user domain.UserUpdateParams) (domain.User, error) {
	updatedUser, err := r.querier.UpdateUser(ctx, UpdateUserParams{
		ID:               user.ID,
		FirstName:        user.FirstName,
		LastName:         user.LastName,
		Email:            user.Email,
		PasswordHash:     user.PasswordHash,
		ProfilePictureID: ptrToNullInt32(user.ProfilePictureID),
		UpdatedAt:        time.Now(),
	})
	if err != nil {
		return domain.User{}, err
	}

	return domain.User{
		ID:               updatedUser.ID,
		FirstName:        updatedUser.FirstName,
		LastName:         updatedUser.LastName,
		Email:            updatedUser.Email,
		PasswordHash:     updatedUser.PasswordHash,
		ProfilePictureID: nullInt32Ptr(updatedUser.ProfilePictureID),
	}, nil
}

func (r *Repository) DeleteUser(ctx context.Context, userID int32) error {
	err := r.querier.DeleteUser(ctx, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrUserNotFound
	}
	return fmt.Errorf("delete user: %w", err)
}

func (r *Repository) GetAlbumByID(ctx context.Context, id int32) (domain.Album, error) {
	album, err := r.querier.GetAlbumById(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Album{}, ErrAlbumNotFound
		}
		return domain.Album{}, err
	}

	return domain.Album{
		ID:           album.ID,
		UserID:       album.UserID,
		Title:        album.Title,
		CoverPhotoID: nullInt32Ptr(album.CoverPhotoID),
		NumPhotos:    album.NumPhotos,
		CreatedAt:    album.CreatedAt,
		UpdatedAt:    album.UpdatedAt,
	}, nil
}

func (r *Repository) ListAlbumPhotoViewRows(ctx context.Context, albumID, limit, offset int32) ([]domain.AlbumPhotoViewRow, error) {
	rows, err := r.querier.ListAlbumPhotoViewRows(ctx, ListAlbumPhotoViewRowsParams{
		AlbumID: albumID,
		Limit:   limit,
		Offset:  offset,
	})
	if err != nil {
		return nil, err
	}
	result := make([]domain.AlbumPhotoViewRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, domain.AlbumPhotoViewRow{
			PhotoID:  row.PhotoID,
			PhotoKey: row.PhotoKey,
			Variant:  domain.PhotoVariant(row.Variant.PhotoVariant),
			Width:    row.Width.Int32,
			Height:   row.Height.Int32,
			MimeType: row.MimeType.String,
		})
	}
	return result, nil
}

func (r *Repository) ListAlbumsByUser(ctx context.Context, userID int32, limit, offset int32) ([]domain.AlbumListItem, error) {
	albums, err := r.querier.ListAlbumsByUser(ctx, ListAlbumsByUserParams{
		UserID: userID,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, err
	}

	result := make([]domain.AlbumListItem, 0, len(albums))
	for _, album := range albums {
		result = append(result, domain.AlbumListItem{
			Album: domain.Album{
				ID:           album.ID,
				UserID:       album.UserID,
				Title:        album.Title,
				CoverPhotoID: nullInt32Ptr(album.CoverPhotoID),
				NumPhotos:    album.NumPhotos,
				CreatedAt:    album.CreatedAt,
				UpdatedAt:    album.UpdatedAt,
			},
			CoverPhotoKey: album.CoverPhotoKey.String,
		})
	}

	return result, nil
}

func (r *Repository) CreateAlbum(ctx context.Context, userID int32, title string) (domain.Album, error) {
	album, err := r.querier.CreateAlbum(ctx, CreateAlbumParams{
		UserID: userID,
		Title:  title,
	})
	if err != nil {
		return domain.Album{}, err
	}

	return domain.Album{
		ID:           album.ID,
		UserID:       album.UserID,
		Title:        album.Title,
		CoverPhotoID: nullInt32Ptr(album.CoverPhotoID),
		NumPhotos:    album.NumPhotos,
		CreatedAt:    album.CreatedAt,
		UpdatedAt:    album.UpdatedAt,
	}, nil
}

func (r *Repository) UpdateAlbum(ctx context.Context, albumId int32, userID int32, title string) (domain.Album, error) {
	album, err := r.querier.UpdateAlbum(ctx, UpdateAlbumParams{
		ID:     albumId,
		UserID: userID,
		Title:  title,
	})
	if err != nil {
		return domain.Album{}, err
	}

	return domain.Album{
		ID:           album.ID,
		UserID:       album.UserID,
		Title:        album.Title,
		CoverPhotoID: nullInt32Ptr(album.CoverPhotoID),
		NumPhotos:    album.NumPhotos,
		CreatedAt:    album.CreatedAt,
		UpdatedAt:    album.UpdatedAt,
	}, nil
}

func (r *Repository) DeleteAlbum(ctx context.Context, albumId int32) error {
	return r.querier.DeleteAlbum(ctx, albumId)
}

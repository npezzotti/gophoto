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
	ErrUserNotFound  = errors.New("user not found")
	ErrAlbumNotFound = errors.New("album not found")
	ErrPhotoNotFound = errors.New("photo not found")
)

type PhotoRepository interface {
	GetPhoto(ctx context.Context, id int32) (domain.Photo, error)
	GetAlbumPhoto(ctx context.Context, id int32) (domain.AlbumPhoto, error)
	ListPhotosByAlbum(ctx context.Context, albumId int32, limit int32, offset int32) ([]domain.Photo, error)
	GetPhotoMetadataByPhotoID(ctx context.Context, photoId int32) ([]domain.PhotoMetadatum, error)
	CreateAlbumPhotoWithOriginalMetadata(ctx context.Context, albumID int32, cmd domain.CreatePhotoWithOriginalMetadataParams) (domain.Photo, error)
	CreateUserPhotoWithOriginalMetadata(ctx context.Context, userID int32, cmd domain.CreatePhotoWithOriginalMetadataParams) (domain.Photo, error)
	RemovePhotoFromAlbum(ctx context.Context, albumId int32, photoId int32) error
	CreatePhotoMetadata(ctx context.Context, arg domain.CreatePhotoMetadataParams) (domain.PhotoMetadatum, error)
	GetPhotoMetadataByPhotoIDAndVariant(ctx context.Context, photoID int32, variant domain.PhotoVariant) (domain.PhotoMetadatum, error)
	UpdatePhotoStatus(ctx context.Context, photoID int32, status domain.PhotoStatus) error
	DeletePhoto(ctx context.Context, photoID int32) error
	GetOrphanedPhotos(ctx context.Context) ([]domain.Photo, error)
}

type AlbumRepository interface {
	GetAlbumByID(ctx context.Context, id int32) (domain.Album, error)
	CountAlbumsByUser(ctx context.Context, userID int32) (int64, error)
	ListAlbumsByUser(ctx context.Context, userID int32, limit, offset int32) ([]domain.AlbumListProjection, error)
	GetPhotoMetadataByPhotoID(ctx context.Context, photoID int32) ([]domain.PhotoMetadatum, error)
	CreateAlbum(ctx context.Context, userID int32, title string) (domain.Album, error)
	UpdateAlbum(ctx context.Context, albumId int32, userID int32, title string, coverPhotoID *int32) (domain.Album, error)
	DeleteAlbum(ctx context.Context, albumId int32) error
}

type UserRepository interface {
	GetUserById(ctx context.Context, id int32) (domain.User, error)
	GetUserByEmail(ctx context.Context, email string) (domain.User, error)
	CreateUser(ctx context.Context, firstName, lastName, email, passwordHash string) (domain.User, error)
	UpdateUser(ctx context.Context, params domain.UserUpdateParams) (domain.User, error)
	DeleteUser(ctx context.Context, userID int32) error
	UserExists(ctx context.Context, userID int32) (bool, error)
	GetPhotoMetadataByPhotoID(ctx context.Context, photoId int32) ([]domain.PhotoMetadatum, error)
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
		return domain.PhotoMetadatum{}, err
	}

	return domain.PhotoMetadatum{
		ID:        metadata.ID,
		PhotoID:   metadata.PhotoID,
		Variant:   domain.PhotoVariant(metadata.Variant),
		Width:     metadata.Width,
		Height:    metadata.Height,
		FileSize:  nullInt64Ptr(metadata.FileSize),
		MimeType:  metadata.MimeType,
		CreatedAt: metadata.CreatedAt,
	}, nil
}

func (r *Repository) GetAlbumPhoto(ctx context.Context, id int32) (domain.AlbumPhoto, error) {
	photo, err := r.querier.GetAlbumPhoto(ctx, id)
	if err != nil {
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

func (r *Repository) ListPhotosByAlbum(ctx context.Context, albumId int32, limit int32, offset int32) ([]domain.Photo, error) {
	photos, err := r.querier.ListPhotosByAlbum(ctx, ListPhotosByAlbumParams{
		AlbumID: albumId,
		Limit:   limit,
		Offset:  offset,
	})
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

func (r *Repository) GetPhotoMetadataByPhotoID(ctx context.Context, photoId int32) ([]domain.PhotoMetadatum, error) {
	metadata, err := r.querier.GetPhotoMetadataByPhotoID(ctx, photoId)
	if err != nil {
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
			FileSize:  nullInt64Ptr(m.FileSize),
			MimeType:  m.MimeType,
			CreatedAt: m.CreatedAt,
		})
	}
	return result, nil
}

func (q *Queries) AddPhotoToAlbumWithCover(ctx context.Context, arg AddPhotoToAlbumParams) (Album, error) {
	album, err := q.GetAlbumById(ctx, arg.AlbumID)
	if err != nil {
		return Album{}, fmt.Errorf("get album: %v", err)
	}

	albumPhoto, err := q.AddPhotoToAlbum(ctx, arg)
	if err != nil {
		return Album{}, fmt.Errorf("add photo to album: %v", err)
	}

	// Increment the album's photo count
	if err := q.IncrementAlbumPhotoCount(ctx, IncrementAlbumPhotoCountParams{
		ID:        album.ID,
		UpdatedAt: time.Now(),
	}); err != nil {
		return Album{}, fmt.Errorf("increment album photo count: %v", err)
	}

	var updatedAlbum Album
	if !album.CoverPhotoID.Valid {
		updatedAlbum, err = q.UpdateAlbum(ctx, UpdateAlbumParams{
			ID:           album.ID,
			UserID:       album.UserID,
			Title:        album.Title,
			CoverPhotoID: sql.NullInt32{Int32: albumPhoto.ID, Valid: true},
			UpdatedAt:    time.Now(),
		})
		if err != nil {
			return Album{}, fmt.Errorf("set album cover photo: %v", err)
		}
	}

	return updatedAlbum, nil
}

type CreatePhotoWithOriginalMetadataParams struct {
	UserID   sql.NullInt32
	Key      string
	Width    int32
	Height   int32
	FileSize sql.NullInt64
	MimeType string
}

func toCreatePhotoWithOriginalMetadataParams(arg domain.CreatePhotoWithOriginalMetadataParams) CreatePhotoWithOriginalMetadataParams {
	return CreatePhotoWithOriginalMetadataParams{
		UserID:   ptrToNullInt32(arg.UserID),
		Key:      arg.Key,
		Width:    arg.Width,
		Height:   arg.Height,
		FileSize: ptrToNullInt64(arg.FileSize),
		MimeType: arg.MimeType,
	}
}

func (r *Repository) CreateAlbumPhotoWithOriginalMetadata(ctx context.Context, albumId int32, arg domain.CreatePhotoWithOriginalMetadataParams) (domain.Photo, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Photo{}, err
	}

	q := r.querier.WithTx(tx)

	photo, err := q.createPhotoWithOriginalMetadata(ctx, toCreatePhotoWithOriginalMetadataParams(arg))
	if err != nil {
		if err := tx.Rollback(); err != nil {
			return domain.Photo{}, fmt.Errorf("create photo with original metadata: %v, unable to rollback: %v", err, err)
		}
		return domain.Photo{}, fmt.Errorf("create photo with original metadata: %v", err)
	}

	if _, err = q.AddPhotoToAlbumWithCover(ctx, AddPhotoToAlbumParams{
		PhotoID: photo.ID,
		AlbumID: albumId,
	}); err != nil {
		if err := tx.Rollback(); err != nil {
			return domain.Photo{}, fmt.Errorf("add photo to album with cover: %v, unable to rollback: %v", err, err)
		}
		return domain.Photo{}, fmt.Errorf("add photo to album with cover: %v", err)
	}

	if err = tx.Commit(); err != nil {
		if err := tx.Rollback(); err != nil {
			return domain.Photo{}, fmt.Errorf("commit tx: %v, unable to rollback: %v", err, err)
		}
		return domain.Photo{}, fmt.Errorf("commit tx: %v", err)
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

func (r *Repository) CreateUserPhotoWithOriginalMetadata(ctx context.Context, userID int32, arg domain.CreatePhotoWithOriginalMetadataParams) (domain.Photo, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Photo{}, err
	}

	q := r.querier.WithTx(tx)

	user, err := q.GetUserById(ctx, userID)
	if err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return domain.Photo{}, fmt.Errorf("get user by id: %v, unable to rollback: %v", err, rbErr)
		}
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Photo{}, ErrUserNotFound
		}
		return domain.Photo{}, fmt.Errorf("get user by id: %v", err)
	}

	photo, err := q.createPhotoWithOriginalMetadata(ctx, toCreatePhotoWithOriginalMetadataParams(arg))
	if err != nil {
		if err := tx.Rollback(); err != nil {
			return domain.Photo{}, fmt.Errorf("create photo with original metadata: %v, unable to rollback: %v", err, err)
		}
		return domain.Photo{}, fmt.Errorf("create photo with original metadata: %v", err)
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
			return domain.Photo{}, fmt.Errorf("update user profile picture: %v, unable to rollback: %v", err, err)
		}
		return domain.Photo{}, fmt.Errorf("update user profile picture: %v", err)
	}

	if err = tx.Commit(); err != nil {
		if err := tx.Rollback(); err != nil {
			return domain.Photo{}, fmt.Errorf("commit tx: %v, unable to rollback: %v", err, err)
		}
		return domain.Photo{}, fmt.Errorf("commit tx: %v", err)
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

func (r *Repository) RemovePhotoFromAlbum(ctx context.Context, albumId int32, photoId int32) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	q := r.querier.WithTx(tx)

	if err := q.DeleteAlbumPhoto(ctx, DeleteAlbumPhotoParams{
		AlbumID: albumId,
		PhotoID: photoId,
	}); err != nil {
		if err := tx.Rollback(); err != nil {
			return fmt.Errorf("delete photo from album: %v, unable to rollback: %v", err, err)
		}
		return fmt.Errorf("delete photo from album: %v", err)
	}

	if err := q.DecrementAlbumPhotoCount(ctx, DecrementAlbumPhotoCountParams{ID: albumId, UpdatedAt: time.Now()}); err != nil {
		if err := tx.Rollback(); err != nil {
			return fmt.Errorf("decrement album photo count: %v, unable to rollback: %v", err, err)
		}
		return fmt.Errorf("decrement album photo count: %v", err)
	}

	if err = tx.Commit(); err != nil {
		if err := tx.Rollback(); err != nil {
			return fmt.Errorf("commit tx: %v, unable to rollback: %v", err, err)
		}
		return fmt.Errorf("commit tx: %v", err)
	}

	return nil
}

func (r *Repository) CreatePhotoMetadata(ctx context.Context, arg domain.CreatePhotoMetadataParams) (domain.PhotoMetadatum, error) {
	metadata, err := r.querier.CreatePhotoMetadata(ctx, CreatePhotoMetadataParams{
		PhotoID:  arg.PhotoID,
		Variant:  PhotoVariant(arg.Variant),
		Width:    arg.Width,
		Height:   arg.Height,
		FileSize: ptrToNullInt64(arg.FileSize),
		MimeType: arg.MimeType,
	})
	if err != nil {
		return domain.PhotoMetadatum{}, fmt.Errorf("create photo metadata: %v", err)
	}

	return domain.PhotoMetadatum{
		ID:        metadata.ID,
		PhotoID:   metadata.PhotoID,
		Variant:   domain.PhotoVariant(metadata.Variant),
		Width:     metadata.Width,
		Height:    metadata.Height,
		FileSize:  nullInt64Ptr(metadata.FileSize),
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

func (r *Repository) UserExists(ctx context.Context, userID int32) (bool, error) {
	return r.querier.UserExists(ctx, userID)
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
	return r.querier.DeleteUser(ctx, userID)
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

func (r *Repository) CountAlbumsByUser(ctx context.Context, userID int32) (int64, error) {
	return r.querier.CountAlbumsByUser(ctx, userID)
}

func (r *Repository) ListAlbumsByUser(ctx context.Context, userID int32, limit, offset int32) ([]domain.AlbumListProjection, error) {
	albums, err := r.querier.ListAlbumsByUser(ctx, ListAlbumsByUserParams{
		UserID: userID,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, err
	}

	result := make([]domain.AlbumListProjection, 0, len(albums))
	for _, album := range albums {
		result = append(result, domain.AlbumListProjection{
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

func (r *Repository) UpdateAlbum(ctx context.Context, albumId int32, userID int32, title string, coverPhotoID *int32) (domain.Album, error) {
	coverPhoto := sql.NullInt32{Valid: false}
	if coverPhotoID != nil {
		coverPhoto = sql.NullInt32{Int32: *coverPhotoID, Valid: true}
	}

	album, err := r.querier.UpdateAlbum(ctx, UpdateAlbumParams{
		ID:           albumId,
		UserID:       userID,
		Title:        title,
		CoverPhotoID: coverPhoto,
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

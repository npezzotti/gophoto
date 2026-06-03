package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/h2non/bimg"
	"github.com/npezzotti/gophoto/internal/db"
	"github.com/npezzotti/gophoto/internal/domain"
	"github.com/npezzotti/gophoto/internal/utils"
	"github.com/npezzotti/gophoto/internal/workers"
	"github.com/npezzotti/gophoto/pkg/logging"
	"github.com/npezzotti/gophoto/pkg/store"
	"github.com/redis/go-redis/v9"
)

const (
	FormFileName  = "file"
	MaxUploadSize = 50 << (10 * 2) // 50 MB
)

var (
	ErrFileTooLarge    = fmt.Errorf("file size exceeds max upload size of %dMB", MaxUploadSize/1024/1024)
	ErrInvalidFileType = fmt.Errorf("file type not allowed")
)

type PhotoService struct {
	photoRepo   db.PhotoRepository
	albumRepo   db.AlbumRepository
	store       store.Store
	redisClient queuePublisher
	logger      *logging.Logger
}

type queuePublisher interface {
	Publish(ctx context.Context, channel string, message interface{}) *redis.IntCmd
}

func NewPhotoService(photoRepo db.PhotoRepository, albumRepo db.AlbumRepository, store store.Store, redisClient queuePublisher, logger *logging.Logger) *PhotoService {
	return &PhotoService{
		photoRepo:   photoRepo,
		albumRepo:   albumRepo,
		store:       store,
		redisClient: redisClient,
		logger:      logger,
	}
}

func (s *PhotoService) GetPhoto(ctx context.Context, id int32) (domain.Photo, error) {
	photo, err := s.photoRepo.GetPhoto(ctx, id)
	if err != nil {
		return domain.Photo{}, fmt.Errorf("error getting photo: %w", err)
	}
	return photo, nil
}

func (s *PhotoService) GetAlbumPhoto(ctx context.Context, id int32) (domain.AlbumPhoto, error) {
	photo, err := s.photoRepo.GetAlbumPhoto(ctx, id)
	if err != nil {
		return domain.AlbumPhoto{}, fmt.Errorf("error getting album photo: %w", err)
	}
	return photo, nil
}

func (s *PhotoService) GetPhotoMetadataByPhotoID(ctx context.Context, photoId int32) ([]domain.PhotoMetadatum, error) {
	metadata, err := s.photoRepo.GetPhotoMetadataByPhotoID(ctx, photoId)
	if err != nil {
		return nil, fmt.Errorf("error getting photo metadata: %w", err)
	}
	return metadata, nil
}

func (s *PhotoService) CreateAlbumPhotoWithOriginalMetadata(ctx context.Context, f multipart.File, fh *multipart.FileHeader, userID int32, albumID int32) (domain.Photo, error) {
	album, err := s.albumRepo.GetAlbumByID(ctx, albumID)
	if err != nil {
		return domain.Photo{}, fmt.Errorf("error getting album: %w", err)
	}

	if album.UserID != userID {
		return domain.Photo{}, domain.ErrAlbumNotFound
	}

	buf, fileType, meta, err := s.processUploadedFile(f, fh)
	if err != nil {
		return domain.Photo{}, fmt.Errorf("error processing uploaded file: %w", err)
	}

	key := uuid.NewString()
	photo, err := s.photoRepo.CreateAlbumPhotoWithOriginalMetadata(ctx, albumID, domain.CreatePhotoWithOriginalMetadataParams{
		UserID:   userID,
		Key:      key,
		Width:    int32(meta.Width),
		Height:   int32(meta.Height),
		FileSize: int64(len(buf)),
		MimeType: string(fileType),
	})
	if err != nil {
		return domain.Photo{}, fmt.Errorf("error creating album photo: %w", err)
	}

	if err := s.uploadPhotoToStorage(ctx, photo, buf, fileType); err != nil {
		return domain.Photo{}, fmt.Errorf("error uploading photo to storage: %w", err)
	}

	if err := s.queuePhotoProcessing(ctx, photo); err != nil {
		return domain.Photo{}, fmt.Errorf("error queueing photo processing: %w", err)
	}

	return photo, nil
}

func (s *PhotoService) CreateUserPhotoWithOriginalMetadata(ctx context.Context, f multipart.File, fh *multipart.FileHeader, userID int32) (domain.Photo, error) {
	buf, fileType, meta, err := s.processUploadedFile(f, fh)
	if err != nil {
		return domain.Photo{}, fmt.Errorf("error processing uploaded file: %w", err)
	}

	key := uuid.New().String()
	photo, err := s.photoRepo.CreateUserPhotoWithOriginalMetadata(ctx, domain.CreatePhotoWithOriginalMetadataParams{
		UserID:   userID,
		Key:      key,
		Width:    int32(meta.Width),
		Height:   int32(meta.Height),
		FileSize: int64(len(buf)),
		MimeType: string(fileType),
	})
	if err != nil {
		return domain.Photo{}, fmt.Errorf("error creating user photo: %w", err)
	}

	if err := s.uploadPhotoToStorage(ctx, photo, buf, fileType); err != nil {
		return domain.Photo{}, fmt.Errorf("error uploading photo to storage: %w", err)
	}

	if err := s.queuePhotoProcessing(ctx, photo); err != nil {
		return domain.Photo{}, fmt.Errorf("error queueing photo processing: %w", err)
	}

	return photo, nil
}

func (s *PhotoService) RemovePhotoFromAlbum(ctx context.Context, photoID, userID int32) error {
	albumPhoto, err := s.GetAlbumPhoto(ctx, photoID)
	if err != nil {
		return fmt.Errorf("error getting album photo: %w", err)
	}

	if albumPhoto.UserID == nil || *albumPhoto.UserID != userID {
		return domain.ErrPhotoNotFound
	}

	if err := s.photoRepo.RemovePhotoFromAlbum(ctx, albumPhoto.AlbumID, photoID); err != nil {
		return fmt.Errorf("error removing photo from album: %w", err)
	}
	return nil
}

func (s *PhotoService) processUploadedFile(f multipart.File, fh *multipart.FileHeader) ([]byte, utils.MimeType, bimg.ImageSize, error) {
	fileType, err := detectContentType(f)
	if err != nil {
		return nil, "", bimg.ImageSize{}, fmt.Errorf("error detecting content type: %w", err)
	}

	if err := validatePhotoUpload(fileType, fh); err != nil {
		return nil, "", bimg.ImageSize{}, fmt.Errorf("photo validation failed: %w", err)
	}

	buf, err := io.ReadAll(f)
	if err != nil {
		return nil, "", bimg.ImageSize{}, fmt.Errorf("error reading uploaded file: %w", err)
	}

	meta, err := bimg.NewImage(buf).Size()
	if err != nil {
		return nil, "", bimg.ImageSize{}, fmt.Errorf("error calculating image size: %w", err)
	}

	return buf, utils.MimeType(fileType), meta, nil
}

func validatePhotoUpload(fileType string, fh *multipart.FileHeader) error {
	if fh.Size > MaxUploadSize {
		return ErrFileTooLarge
	}

	if !strings.HasPrefix(fileType, "image/") || !utils.ValidateMimeType(fileType) {
		return ErrInvalidFileType
	}
	return nil
}

// detectContentType reads the first 512 bytes of the provided file to determine its content type.
// It resets the file's read pointer to the beginning before returning.
func detectContentType(f multipart.File) (string, error) {
	buff := make([]byte, 512)
	_, err := f.Read(buff)
	if err != nil {
		return "", fmt.Errorf("unable to read file: %w", err)
	}

	filetype := http.DetectContentType(buff)

	_, err = f.Seek(0, io.SeekStart)
	if err != nil {
		return "", fmt.Errorf("failed to reset file pointer: %w", err)
	}
	return filetype, nil
}

func (s *PhotoService) uploadPhotoToStorage(ctx context.Context, photo domain.Photo, buf []byte, fileType utils.MimeType) error {
	path, err := utils.BuildPhotoPathForVariant(photo.Key, domain.PhotoVariantOriginal, fileType)
	if err != nil {
		return fmt.Errorf("error building photo path: %w", err)
	}

	if err := s.store.Write(ctx, path, bytes.NewReader(buf)); err != nil {
		return fmt.Errorf("error writing photo to storage: %w", err)
	}
	return nil
}

func (s *PhotoService) queuePhotoProcessing(ctx context.Context, photo domain.Photo) error {
	processingJob, err := json.Marshal(workers.PhotoProcessingJob{Type: workers.JobTypeAlbumPhoto, PhotoID: photo.ID})
	if err != nil {
		return fmt.Errorf("error marshalling photo processing job for photo %d: %w", photo.ID, err)
	}

	err = s.redisClient.Publish(ctx, workers.PhotoProcessingQueue, processingJob).Err()
	if err != nil {
		return fmt.Errorf("error publishing photo processing job for photo %d: %w", photo.ID, err)
	}
	return nil
}

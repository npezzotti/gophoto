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
	"github.com/npezzotti/gophoto/pkg/store"
	"github.com/redis/go-redis/v9"
)

const (
	FormFileName  = "file"
	MaxUploadSize = 50 << (10 * 2) // 50 MB
)

type PhotoService struct {
	repo        db.PhotoRepository
	store       store.Store
	redisClient *redis.Client
}

func NewPhotoService(r db.PhotoRepository, s store.Store, redisClient *redis.Client) *PhotoService {
	return &PhotoService{repo: r, store: s, redisClient: redisClient}
}

func (s *PhotoService) GetPhoto(ctx context.Context, id int32) (domain.Photo, error) {
	photo, err := s.repo.GetPhoto(ctx, id)
	if err != nil {
		return domain.Photo{}, fmt.Errorf("error getting photo: %w", err)
	}
	return photo, nil
}

func (s *PhotoService) GetAlbumPhoto(ctx context.Context, id int32) (domain.AlbumPhoto, error) {
	photo, err := s.repo.GetAlbumPhoto(ctx, id)
	if err != nil {
		return domain.AlbumPhoto{}, fmt.Errorf("error getting album photo: %w", err)
	}
	return photo, nil
}

func (s *PhotoService) ListPhotosByAlbum(ctx context.Context, albumId int32, limit int32, offset int32) ([]domain.ResponsiveImage, error) {
	photos, err := s.repo.ListPhotosByAlbum(ctx, albumId, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("error listing photos by album: %w", err)
	}

	images := []domain.ResponsiveImage{}
	for _, photo := range photos {
		imageResponse := s.generateAlbumImageResponse(ctx, photo)
		images = append(images, imageResponse)
	}

	return images, nil
}

func (s *PhotoService) GetPhotoMetadataByPhotoID(ctx context.Context, photoId int32) ([]domain.PhotoMetadatum, error) {
	metadata, err := s.repo.GetPhotoMetadataByPhotoID(ctx, photoId)
	if err != nil {
		return nil, fmt.Errorf("error getting photo metadata: %w", err)
	}
	return metadata, nil
}

func (s *PhotoService) CreateAlbumPhotoWithOriginalMetadata(ctx context.Context, f multipart.File, fh *multipart.FileHeader, userID int32, albumID int32) (domain.Photo, error) {
	buf, fileType, meta, err := s.processUploadedFile(f, fh)
	if err != nil {
		return domain.Photo{}, fmt.Errorf("error processing uploaded file: %w", err)
	}

	key := uuid.New().String()
	photo, err := s.repo.CreateAlbumPhotoWithOriginalMetadata(ctx, albumID, domain.CreatePhotoWithOriginalMetadataParams{
		UserID:   &userID,
		Key:      key,
		Width:    int32(meta.Width),
		Height:   int32(meta.Height),
		FileSize: nil,
		MimeType: string(fileType),
	})
	if err != nil {
		return domain.Photo{}, err
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
	fileSize := int64(len(buf))
	photo, err := s.repo.CreateUserPhotoWithOriginalMetadata(ctx, userID, domain.CreatePhotoWithOriginalMetadataParams{
		UserID:   &userID,
		Key:      key,
		Width:    int32(meta.Width),
		Height:   int32(meta.Height),
		FileSize: &fileSize,
		MimeType: string(fileType),
	})
	if err != nil {
		return domain.Photo{}, fmt.Errorf("error creating user photo")
	}

	if err := s.uploadPhotoToStorage(ctx, photo, buf, fileType); err != nil {
		return domain.Photo{}, fmt.Errorf("error uploading photo to storage: %w", err)
	}

	if err := s.queuePhotoProcessing(ctx, photo); err != nil {
		return domain.Photo{}, fmt.Errorf("error queueing photo processing: %w", err)
	}

	return photo, nil
}

func (s *PhotoService) RemovePhotoFromAlbum(ctx context.Context, albumId int32, photoId int32) error {
	if err := s.repo.RemovePhotoFromAlbum(ctx, albumId, photoId); err != nil {
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
		return nil, "", bimg.ImageSize{}, fmt.Errorf("error validating photo upload: %w", err)
	}

	buf, err := io.ReadAll(f)
	if err != nil {
		return nil, "", bimg.ImageSize{}, fmt.Errorf("error reading uploaded file: %w", err)
	}

	meta, err := bimg.NewImage(buf).Size()
	if err != nil {
		return nil, "", bimg.ImageSize{}, fmt.Errorf("error getting image size: %w", err)
	}

	return buf, utils.MimeType(fileType), meta, nil
}

func validatePhotoUpload(fileType string, fh *multipart.FileHeader) error {
	if fh.Size > MaxUploadSize {
		return fmt.Errorf("file size exceeds max upload size of %dMB", MaxUploadSize/1024/1024)
	}

	if !strings.HasPrefix(fileType, "image/") || !utils.ValidateMimeType(fileType) {
		return fmt.Errorf("file type not allowed")
	}
	return nil
}

// detectContentType reads the first 512 bytes of the provided file to determine its content type.
// It resets the file's read pointer to the beginning before returning.
func detectContentType(f multipart.File) (string, error) {
	buff := make([]byte, 512)
	_, err := f.Read(buff)
	if err != nil {
		return "", fmt.Errorf("error reading file: %w", err)
	}

	filetype := http.DetectContentType(buff)

	_, err = f.Seek(0, io.SeekStart)
	if err != nil {
		return "", fmt.Errorf("seek: %s", err)
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
		return fmt.Errorf("error marshalling photo processing job for photo %d: %s", photo.ID, err.Error())
	}

	err = s.redisClient.Publish(ctx, workers.PhotoProcessingQueue, processingJob).Err()
	if err != nil {
		return fmt.Errorf("error publishing photo processing job for photo %d: %s", photo.ID, err.Error())
	}
	return nil
}

func (s *PhotoService) generateAlbumImageResponse(ctx context.Context, photo domain.Photo) domain.ResponsiveImage {
	photoMeta, err := s.repo.GetPhotoMetadataByPhotoID(ctx, photo.ID)
	if err != nil {
		return domain.ResponsiveImage{}
	}

	var sources []domain.ImageSource
	var originalUrl, defaultUrl string
	for _, meta := range photoMeta {
		path, err := utils.BuildPhotoPathForVariant(photo.Key, meta.Variant, utils.MimeType(meta.MimeType))
		if err != nil {
			continue
		}

		url, err := s.store.GenerateURL(ctx, path)
		if err != nil {
			continue
		}

		if meta.Variant != domain.PhotoVariantOriginal {
			sources = append(sources, domain.ImageSource{
				Width:  meta.Width,
				Height: meta.Height,
				URL:    url,
			})
		}

		switch meta.Variant {
		case domain.PhotoVariantOriginal:
			originalUrl = url
		case domain.PhotoVariantLarge:
			defaultUrl = url
		default:
		}
	}

	return domain.ResponsiveImage{
		ID:          photo.ID,
		Alt:         photo.Key,
		OriginalSrc: originalUrl,
		DefaultSrc:  defaultUrl,
		Sources:     sources,
	}
}

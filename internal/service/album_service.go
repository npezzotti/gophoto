package service

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/npezzotti/gophoto/internal/config"
	"github.com/npezzotti/gophoto/internal/db"
	"github.com/npezzotti/gophoto/internal/domain"
	"github.com/npezzotti/gophoto/internal/utils"
	"github.com/npezzotti/gophoto/pkg/store"
)

type AlbumService struct {
	albumRepo db.AlbumRepository
	photoRepo db.PhotoRepository
	store     store.Store
	config    *config.Config
}

func NewAlbumService(r db.AlbumRepository, p db.PhotoRepository, s store.Store, c *config.Config) *AlbumService {
	return &AlbumService{albumRepo: r, photoRepo: p, store: s, config: c}
}

func (s *AlbumService) GetAlbumByID(ctx context.Context, albumID int32) (domain.Album, error) {
	return s.albumRepo.GetAlbumByID(ctx, albumID)
}

func (s *AlbumService) GetAlbumPageView(ctx context.Context, userID, albumID, limit, offset int32) (domain.AlbumPageView, error) {
	album, err := s.albumRepo.GetAlbumByID(ctx, albumID)
	if err != nil {
		return domain.AlbumPageView{}, fmt.Errorf("error getting album: %w", err)
	}

	if album.UserID != userID {
		return domain.AlbumPageView{}, domain.ErrAlbumNotFound
	}

	albumViewRows, err := s.albumRepo.ListAlbumPhotoViewRows(ctx, album.ID, limit, offset)
	if err != nil {
		return domain.AlbumPageView{}, fmt.Errorf("error getting album page view: %w", err)
	}

	res := domain.AlbumPageView{
		Album:       album,
		Photos:      make([]domain.ResponsiveImage, 0, len(albumViewRows)),
		TotalPhotos: album.NumPhotos,
	}

	photosByID := make(map[int32]*domain.ResponsiveImage, len(albumViewRows))
	orderedPhotoIDs := make([]int32, 0, len(albumViewRows))

	for _, photo := range albumViewRows {
		image, ok := photosByID[photo.PhotoID]
		if !ok {
			image = &domain.ResponsiveImage{
				ID:      photo.PhotoID,
				Alt:     photo.PhotoKey,
				Sources: make([]domain.ImageSource, 0, 3),
			}
			photosByID[photo.PhotoID] = image
			orderedPhotoIDs = append(orderedPhotoIDs, photo.PhotoID)
		}

		if photo.Variant == "" || photo.MimeType == "" {
			continue
		}

		path, err := utils.BuildPhotoPathForVariant(photo.PhotoKey, photo.Variant, utils.MimeType(photo.MimeType))
		if err != nil {
			continue
		}

		url, err := s.store.GenerateURL(ctx, path)
		if err != nil {
			continue
		}

		switch photo.Variant {
		case domain.PhotoVariantOriginal:
			image.OriginalSrc = url
		case domain.PhotoVariantLarge:
			image.DefaultSrc = url
			image.Sources = append(image.Sources, domain.ImageSource{
				Width:  photo.Width,
				Height: photo.Height,
				URL:    url,
			})
		default:
			image.Sources = append(image.Sources, domain.ImageSource{
				Width:  photo.Width,
				Height: photo.Height,
				URL:    url,
			})
		}
	}

	for _, photoID := range orderedPhotoIDs {
		image := photosByID[photoID]
		if image.DefaultSrc == "" {
			image.DefaultSrc = image.OriginalSrc
		}

		if image.DefaultSrc == "" {
			continue
		}

		res.Photos = append(res.Photos, *image)
	}

	return res, nil
}

func (s *AlbumService) ListAlbumsByUser(ctx context.Context, userID int32, limit, offset int32) ([]*domain.AlbumListItem, error) {
	albums, err := s.albumRepo.ListAlbumsByUser(ctx, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("error listing albums by user: %w", err)
	}

	result := make([]*domain.AlbumListItem, 0, len(albums))
	for _, album := range albums {
		result = append(result, s.newAlbumListItem(ctx, album))
	}

	return result, nil
}

func (s *AlbumService) CreateAlbum(ctx context.Context, userID int32, title string) (domain.Album, error) {
	if title == "" {
		return domain.Album{}, fmt.Errorf("album title cannot be empty")
	}

	dbAlbum, err := s.albumRepo.CreateAlbum(ctx, userID, title)
	if err != nil {
		return domain.Album{}, fmt.Errorf("error creating album: %w", err)
	}

	return domain.Album{
		ID:           dbAlbum.ID,
		UserID:       dbAlbum.UserID,
		Title:        dbAlbum.Title,
		CoverPhotoID: dbAlbum.CoverPhotoID,
		NumPhotos:    dbAlbum.NumPhotos,
	}, nil
}

func (s *AlbumService) UpdateAlbum(ctx context.Context, albumID int32, userID int32, title string, coverPhotoID *int32) (domain.Album, error) {
	album, err := s.albumRepo.GetAlbumByID(ctx, albumID)
	if err != nil {
		return domain.Album{}, fmt.Errorf("error getting album with ID %d: %w", albumID, err)
	}

	if album.UserID != userID {
		return domain.Album{}, domain.ErrAlbumNotFound
	}

	updatedAlbum, err := s.albumRepo.UpdateAlbum(ctx, albumID, userID, title, coverPhotoID)
	if err != nil {
		return domain.Album{}, fmt.Errorf("error updating album with ID %d: %w", album.ID, err)
	}
	return updatedAlbum, nil
}

func (s *AlbumService) DeleteAlbum(ctx context.Context, albumID, userID int32) error {
	album, err := s.albumRepo.GetAlbumByID(ctx, albumID)
	if err != nil {
		return fmt.Errorf("error getting album with ID %d: %w", albumID, err)
	}

	if album.UserID != userID {
		return domain.ErrAlbumNotFound
	}

	if err := s.albumRepo.DeleteAlbum(ctx, albumID); err != nil {
		return fmt.Errorf("error deleting album with ID %d: %w", album.ID, err)
	}
	return nil
}

func (s *AlbumService) newAlbumListItem(ctx context.Context, album domain.AlbumListItem) *domain.AlbumListItem {
	coverImage := domain.ResponsiveImage{
		DefaultSrc: filepath.Join(s.config.StaticDir, domain.DefaultAlbumCover),
		Sources: []domain.ImageSource{
			{Width: 400, Height: 300, URL: filepath.Join(s.config.StaticDir, domain.DefaultAlbumCover)},
		},
		Alt: "Default album cover",
	}

	if album.Album.CoverPhotoID != nil {
		meta, err := s.photoRepo.GetPhotoMetadataByPhotoID(ctx, *album.Album.CoverPhotoID)
		if err == nil {
			var sources []domain.ImageSource
			var defaultSrc string
			for _, m := range meta {
				path, pathErr := utils.BuildPhotoPathForVariant(album.CoverPhotoKey, m.Variant, utils.MimeType(m.MimeType))
				if pathErr != nil {
					continue
				}

				url, urlErr := s.store.GenerateURL(ctx, path)
				if urlErr != nil {
					continue
				}

				sources = append(sources, domain.ImageSource{
					Width:  m.Width,
					Height: m.Height,
					URL:    url,
				})

				if m.Variant == domain.PhotoVariantLarge {
					defaultSrc = url
				}
			}

			if len(sources) > 0 {
				coverImage = domain.ResponsiveImage{
					Alt:        album.CoverPhotoKey,
					DefaultSrc: defaultSrc,
					Sources:    sources,
				}
			}
		}
	}

	return &domain.AlbumListItem{
		Album:           album.Album,
		AlbumCoverImage: coverImage,
	}
}

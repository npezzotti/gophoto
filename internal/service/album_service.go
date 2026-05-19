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
	repo   db.AlbumRepository
	store  store.Store
	config *config.Config
}

func NewAlbumService(r db.AlbumRepository, s store.Store, c *config.Config) *AlbumService {
	return &AlbumService{repo: r, store: s, config: c}
}

func (s *AlbumService) GetAlbumByID(ctx context.Context, albumID int32) (domain.Album, error) {
	return s.repo.GetAlbumByID(ctx, albumID)
}

func (s *AlbumService) CountAlbumsByUser(ctx context.Context, userID int32) (int64, error) {
	return s.repo.CountAlbumsByUser(ctx, userID)
}

func (s *AlbumService) ListAlbumsByUser(ctx context.Context, userID int32, limit, offset int32) ([]*domain.AlbumListItem, error) {
	albums, err := s.repo.ListAlbumsByUser(ctx, userID, limit, offset)
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
	dbAlbum, err := s.repo.CreateAlbum(ctx, userID, title)
	if err != nil {
		return domain.Album{}, err
	}
	return domain.Album{
		ID:           dbAlbum.ID,
		UserID:       dbAlbum.UserID,
		Title:        dbAlbum.Title,
		CoverPhotoID: dbAlbum.CoverPhotoID,
		NumPhotos:    dbAlbum.NumPhotos,
	}, nil
}

func (s *AlbumService) UpdateAlbum(ctx context.Context, albumId int32, userID int32, title string, coverPhotoID *int32) (domain.Album, error) {
	album, err := s.repo.UpdateAlbum(ctx, albumId, userID, title, coverPhotoID)
	if err != nil {
		return domain.Album{}, err
	}
	return album, nil
}

func (s *AlbumService) DeleteAlbum(ctx context.Context, albumId int32) error {
	if err := s.repo.DeleteAlbum(ctx, albumId); err != nil {
		return err
	}
	return nil
}

func (s *AlbumService) newAlbumListItem(ctx context.Context, album domain.AlbumListProjection) *domain.AlbumListItem {
	coverImage := domain.ResponsiveImage{
		DefaultSrc: filepath.Join(s.config.StaticDir, domain.DefaultAlbumCover),
		Sources: []domain.ImageSource{
			{Width: 400, Height: 300, URL: filepath.Join(s.config.StaticDir, domain.DefaultAlbumCover)},
		},
		Alt: "Default album cover",
	}

	if album.Album.CoverPhotoID != nil {
		meta, err := s.repo.GetPhotoMetadataByPhotoID(ctx, *album.Album.CoverPhotoID)
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

package service

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"time"

	"github.com/npezzotti/gophoto/internal/config"
	"github.com/npezzotti/gophoto/internal/db"
	"github.com/npezzotti/gophoto/internal/utils"
	"github.com/npezzotti/gophoto/pkg/store"
)

type AlbumService struct {
	repo   *db.Repository
	store  store.Store
	config *config.Config
}

func NewAlbumService(r *db.Repository, s store.Store, c *config.Config) *AlbumService {
	return &AlbumService{repo: r, store: s, config: c}
}

type AlbumResponse struct {
	Album           db.ListAlbumsByUserRow
	AlbumCoverImage ImageResponse
}

func (s *AlbumService) GetAlbumById(ctx context.Context, albumId int32) (db.GetAlbumByIdRow, error) {
	album, err := s.repo.GetAlbumById(ctx, albumId)
	if err != nil {
		return db.GetAlbumByIdRow{}, err
	}
	return album, nil
}

func (s *AlbumService) CreateAlbum(ctx context.Context, userID int32, title string) (db.Album, error) {
	album, err := s.repo.CreateAlbum(ctx, db.CreateAlbumParams{
		UserID: userID,
		Title:  title,
	})
	if err != nil {
		return db.Album{}, err
	}
	return album, nil
}

func (s *AlbumService) UpdateAlbum(ctx context.Context, albumId int32, userID int32, title string, coverPhotoID sql.NullInt32) (db.Album, error) {
	album, err := s.repo.UpdateAlbum(ctx, db.UpdateAlbumParams{
		ID:           albumId,
		UserID:       userID,
		Title:        title,
		CoverPhotoID: coverPhotoID,
		UpdatedAt:    time.Now(),
	})
	if err != nil {
		return db.Album{}, err
	}
	return album, nil
}

func (s *AlbumService) DeleteAlbum(ctx context.Context, albumId int32) error {
	if err := s.repo.DeleteAlbum(ctx, albumId); err != nil {
		return err
	}
	return nil
}

func (s *AlbumService) CountAlbumsByUser(ctx context.Context, userID int32) (int64, error) {
	count, err := s.repo.CountAlbumsByUser(ctx, userID)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (s *AlbumService) ListAlbumsByUser(ctx context.Context, userId int32, limit, offset int32) ([]*AlbumResponse, error) {
	albums, err := s.repo.ListAlbumsByUser(ctx, db.ListAlbumsByUserParams{
		UserID: userId,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("error listing albums by user: %w", err)
	}

	var albumResponse []*AlbumResponse
	for _, album := range albums {
		a := s.newAlbumResponse(ctx, album)
		albumResponse = append(albumResponse, a)
	}

	return albumResponse, nil
}

func (s *AlbumService) newAlbumResponse(ctx context.Context, album db.ListAlbumsByUserRow) *AlbumResponse {
	var imageResp ImageResponse
	if album.CoverPhotoID.Valid {
		meta, err := s.repo.GetPhotoMetadataByPhotoID(ctx, album.CoverPhotoID.Int32)
		if err != nil {
			return nil
		}

		var sources []Image
		var defaultSrc string
		for _, m := range meta {
			path, err := utils.BuildPhotoPath(album.CoverPhotoKey.String, m.Variant, utils.MimeType(m.MimeType))
			if err != nil {
				continue
			}

			url, err := s.store.GenerateURL(ctx, path)
			if err != nil {
				continue
			}

			sources = append(sources, Image{
				Width:  m.Width,
				Height: m.Height,
				URL:    url,
			})

			if m.Variant == db.PhotoVariantLarge {
				defaultSrc = url
			}
		}

		imageResp = ImageResponse{
			Image:      db.Photo{ID: album.CoverPhotoID.Int32, Key: album.CoverPhotoKey.String},
			Alt:        album.CoverPhotoKey.String,
			DefaultSrc: defaultSrc,
			Sources:    sources,
		}
	} else {
		imageResp = ImageResponse{
			DefaultSrc: filepath.Join(s.config.StaticDir, DefaultAlbumCover),
			Sources: []Image{
				{Width: 400, Height: 300, URL: filepath.Join(s.config.StaticDir, DefaultAlbumCover)},
			},
			Alt: "Default album cover",
		}
	}

	return &AlbumResponse{
		Album:           album,
		AlbumCoverImage: imageResp,
	}
}

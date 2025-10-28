package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/npezzotti/gophoto/internal/db"
)

type AlbumService struct {
	repo *db.Repository
}

func NewAlbumService(r *db.Repository) *AlbumService {
	return &AlbumService{repo: r}
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

func (s *AlbumService) ListAlbumsByUser(ctx context.Context, userId int32, limit, offset int32) ([]db.ListAlbumsByUserRow, error) {
	albums, err := s.repo.ListAlbumsByUser(ctx, db.ListAlbumsByUserParams{
		UserID: userId,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("error listing albums by user: %w", err)
	}
	return albums, nil
}

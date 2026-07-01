package web

import (
	"context"

	"github.com/npezzotti/gophoto/internal/domain"
)

type albumServiceStub struct {
	getAlbumPageViewFn func(ctx context.Context, userID, albumID, limit, offset int32) (domain.AlbumPageView, error)
	listAlbumsByUserFn func(ctx context.Context, userID int32, limit, offset int32) ([]*domain.AlbumListItem, error)
	createAlbumFn      func(ctx context.Context, userID int32, title string) (domain.Album, error)
	updateAlbumFn      func(ctx context.Context, userID, albumID int32, title string) (domain.Album, error)
	deleteAlbumFn      func(ctx context.Context, userID, albumID int32) error
}

func (s *albumServiceStub) GetAlbumPageView(ctx context.Context, userID, albumID, limit, offset int32) (domain.AlbumPageView, error) {
	if s.getAlbumPageViewFn != nil {
		return s.getAlbumPageViewFn(ctx, userID, albumID, limit, offset)
	}
	return domain.AlbumPageView{}, nil
}

func (s *albumServiceStub) ListAlbumsByUser(ctx context.Context, userID int32, limit, offset int32) ([]*domain.AlbumListItem, error) {
	if s.listAlbumsByUserFn != nil {
		return s.listAlbumsByUserFn(ctx, userID, limit, offset)
	}
	return nil, nil
}

func (s *albumServiceStub) CreateAlbum(ctx context.Context, userID int32, title string) (domain.Album, error) {
	if s.createAlbumFn != nil {
		return s.createAlbumFn(ctx, userID, title)
	}
	return domain.Album{}, nil
}

func (s *albumServiceStub) UpdateAlbum(ctx context.Context, userID, albumID int32, title string) (domain.Album, error) {
	if s.updateAlbumFn != nil {
		return s.updateAlbumFn(ctx, userID, albumID, title)
	}
	return domain.Album{}, nil
}

func (s *albumServiceStub) DeleteAlbum(ctx context.Context, userID, albumID int32) error {
	if s.deleteAlbumFn != nil {
		return s.deleteAlbumFn(ctx, userID, albumID)
	}
	return nil
}

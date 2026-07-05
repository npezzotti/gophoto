package web

import (
	"context"
	"mime/multipart"

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

type photoServiceStub struct {
	createAlbumPhotoWithOriginalMetadataFn func(ctx context.Context, f multipart.File, fh *multipart.FileHeader, userID, albumID int32) (domain.Photo, error)
	removePhotoFromAlbumFn                 func(ctx context.Context, photoID, userID int32) error
	getPhotoFn                             func(ctx context.Context, id int32) (domain.Photo, error)
	createUserPhotoWithOriginalMetadataFn  func(ctx context.Context, f multipart.File, fh *multipart.FileHeader, userID int32) (domain.Photo, error)
}

func (s *photoServiceStub) CreateAlbumPhotoWithOriginalMetadata(ctx context.Context, f multipart.File, fh *multipart.FileHeader, userID, albumID int32) (domain.Photo, error) {
	if s.createAlbumPhotoWithOriginalMetadataFn != nil {
		return s.createAlbumPhotoWithOriginalMetadataFn(ctx, f, fh, userID, albumID)
	}
	return domain.Photo{}, nil
}

func (s *photoServiceStub) CreateUserPhotoWithOriginalMetadata(ctx context.Context, f multipart.File, fh *multipart.FileHeader, userID int32) (domain.Photo, error) {
	if s.createUserPhotoWithOriginalMetadataFn != nil {
		return s.createUserPhotoWithOriginalMetadataFn(ctx, f, fh, userID)
	}
	return domain.Photo{}, nil
}

func (s *photoServiceStub) GetPhoto(ctx context.Context, id int32) (domain.Photo, error) {
	if s.getPhotoFn != nil {
		return s.getPhotoFn(ctx, id)
	}
	return domain.Photo{}, nil
}

func (s *photoServiceStub) RemovePhotoFromAlbum(ctx context.Context, photoID, userID int32) error {
	if s.removePhotoFromAlbumFn != nil {
		return s.removePhotoFromAlbumFn(ctx, photoID, userID)
	}
	return nil
}

type userServiceStub struct {
	getUserFn             func(ctx context.Context, id int32) (domain.UserPresentation, error)
	updateUserFn          func(ctx context.Context, userID int32, firstName, lastName, email, password string) (*domain.UserPresentation, error)
	deleteUserFn          func(ctx context.Context, userID int32) error
	authenticateByEmailFn func(ctx context.Context, email, password string) (domain.UserPresentation, error)
	createUserFn          func(ctx context.Context, firstName, lastName, email, password string) (*domain.UserPresentation, error)
}

func (s *userServiceStub) GetUserByID(ctx context.Context, id int32) (domain.UserPresentation, error) {
	if s.getUserFn != nil {
		return s.getUserFn(ctx, id)
	}
	return domain.UserPresentation{}, nil
}

func (s *userServiceStub) UpdateUser(ctx context.Context, userID int32, firstName, lastName, email, password string) (*domain.UserPresentation, error) {
	if s.updateUserFn != nil {
		return s.updateUserFn(ctx, userID, firstName, lastName, email, password)
	}
	return nil, nil
}

func (s *userServiceStub) DeleteUser(ctx context.Context, userID int32) error {
	if s.deleteUserFn != nil {
		return s.deleteUserFn(ctx, userID)
	}
	return nil
}

func (s *userServiceStub) AuthenticateByEmail(ctx context.Context, email, password string) (domain.UserPresentation, error) {
	if s.authenticateByEmailFn != nil {
		return s.authenticateByEmailFn(ctx, email, password)
	}
	return domain.UserPresentation{}, nil
}

func (s *userServiceStub) CreateUser(ctx context.Context, firstName, lastName, email, password string) (*domain.UserPresentation, error) {
	if s.createUserFn != nil {
		return s.createUserFn(ctx, firstName, lastName, email, password)
	}
	return nil, nil
}

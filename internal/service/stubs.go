package service

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/npezzotti/gophoto/internal/db"
	"github.com/npezzotti/gophoto/internal/domain"
	"github.com/redis/go-redis/v9"
)

type userRepoStub struct {
	getByIDFn    func(ctx context.Context, id int32) (domain.User, error)
	getByEmailFn func(ctx context.Context, email string) (domain.User, error)
	createFn     func(ctx context.Context, firstName, lastName, email, passwordHash string) (domain.User, error)
	updateFn     func(ctx context.Context, params domain.UserUpdateParams) (domain.User, error)
	deleteFn     func(ctx context.Context, userID int32) error
}

func (r *userRepoStub) GetUserByID(ctx context.Context, id int32) (domain.User, error) {
	if r.getByIDFn != nil {
		return r.getByIDFn(ctx, id)
	}
	return domain.User{ID: id}, nil
}

func (r *userRepoStub) GetUserByEmail(ctx context.Context, email string) (domain.User, error) {
	if r.getByEmailFn != nil {
		return r.getByEmailFn(ctx, email)
	}
	return domain.User{}, nil
}

func (r *userRepoStub) CreateUser(ctx context.Context, firstName, lastName, email, passwordHash string) (domain.User, error) {
	if r.createFn != nil {
		return r.createFn(ctx, firstName, lastName, email, passwordHash)
	}
	return domain.User{}, nil
}

func (r *userRepoStub) UpdateUser(ctx context.Context, params domain.UserUpdateParams) (domain.User, error) {
	if r.updateFn != nil {
		return r.updateFn(ctx, params)
	}
	return domain.User{}, nil
}

func (r *userRepoStub) DeleteUser(ctx context.Context, userID int32) error {
	if r.deleteFn != nil {
		return r.deleteFn(ctx, userID)
	}
	return nil
}

type photoRepoStub struct {
	getPhotoByIDFn                         func(ctx context.Context, id int32) (domain.Photo, error)
	getAlbumPhotoByIDFn                    func(ctx context.Context, photoID int32) (domain.AlbumPhoto, error)
	createAlbumPhotoWithOriginalMetadataFn func(ctx context.Context, albumID int32, cmd domain.CreatePhotoWithOriginalMetadataParams) (domain.Photo, error)
	createUserPhotoWithOriginalMetadataFn  func(ctx context.Context, userID int32, cmd domain.CreatePhotoWithOriginalMetadataParams) (domain.Photo, error)
	removePhotoFromAlbumFn                 func(ctx context.Context, albumId int32, photoId int32) error
}

func (p *photoRepoStub) GetPhoto(ctx context.Context, id int32) (domain.Photo, error) {
	if p.getPhotoByIDFn != nil {
		return p.getPhotoByIDFn(ctx, id)
	}
	return domain.Photo{}, nil
}

func (p *photoRepoStub) GetAlbumPhoto(ctx context.Context, photoID int32) (domain.AlbumPhoto, error) {
	if p.getAlbumPhotoByIDFn != nil {
		return p.getAlbumPhotoByIDFn(ctx, photoID)
	}
	return domain.AlbumPhoto{}, nil
}

func (p *photoRepoStub) GetPhotoMetadataByPhotoID(ctx context.Context, photoId int32) ([]domain.PhotoMetadatum, error) {
	return nil, db.ErrPhotoMetadataNotFound
}

func (p *photoRepoStub) CreateAlbumPhotoWithOriginalMetadata(ctx context.Context, albumID int32, cmd domain.CreatePhotoWithOriginalMetadataParams) (domain.Photo, error) {
	if p.createAlbumPhotoWithOriginalMetadataFn != nil {
		return p.createAlbumPhotoWithOriginalMetadataFn(ctx, albumID, cmd)
	}
	return domain.Photo{}, nil
}

func (p *photoRepoStub) CreateUserPhotoWithOriginalMetadata(ctx context.Context, cmd domain.CreatePhotoWithOriginalMetadataParams) (domain.Photo, error) {
	if p.createUserPhotoWithOriginalMetadataFn != nil {
		return p.createUserPhotoWithOriginalMetadataFn(ctx, cmd.UserID, cmd)
	}
	return domain.Photo{}, nil
}

func (p *photoRepoStub) RemovePhotoFromAlbum(ctx context.Context, albumId int32, photoId int32) error {
	if p.removePhotoFromAlbumFn != nil {
		return p.removePhotoFromAlbumFn(ctx, albumId, photoId)
	}
	return nil
}

func (p *photoRepoStub) CreatePhotoMetadata(ctx context.Context, arg domain.CreatePhotoMetadataParams) (domain.PhotoMetadatum, error) {
	return domain.PhotoMetadatum{}, nil
}

func (p *photoRepoStub) GetPhotoMetadataByPhotoIDAndVariant(ctx context.Context, photoID int32, variant domain.PhotoVariant) (domain.PhotoMetadatum, error) {
	return domain.PhotoMetadatum{}, db.ErrPhotoMetadataNotFound
}

func (p *photoRepoStub) UpdatePhotoStatus(ctx context.Context, photoID int32, status domain.PhotoStatus) error {
	return nil
}

func (p *photoRepoStub) DeletePhoto(ctx context.Context, photoID int32) error {
	return nil
}

func (p *photoRepoStub) GetOrphanedPhotos(ctx context.Context) ([]domain.Photo, error) {
	return nil, nil
}

type albumRepoStub struct {
	getAlbumByIDFn           func(ctx context.Context, id int32) (domain.Album, error)
	listAlbumPhotoViewRowsFn func(ctx context.Context, albumID, limit, offset int32) ([]domain.AlbumPhotoViewRow, error)
	createAlbumFn            func(ctx context.Context, userID int32, title string) (domain.Album, error)
	updateAlbumFn            func(ctx context.Context, albumID int32, userID int32, title string) (domain.Album, error)
	deleteAlbumFn            func(ctx context.Context, albumID int32) error
}

func (a *albumRepoStub) GetAlbumByID(ctx context.Context, id int32) (domain.Album, error) {
	if a.getAlbumByIDFn != nil {
		return a.getAlbumByIDFn(ctx, id)
	}
	return domain.Album{}, nil
}

func (a *albumRepoStub) ListAlbumsByUser(ctx context.Context, userID int32, limit, offset int32) ([]domain.AlbumListItem, error) {
	return nil, nil
}

func (a *albumRepoStub) ListAlbumPhotoViewRows(ctx context.Context, albumID, limit, offset int32) ([]domain.AlbumPhotoViewRow, error) {
	if a.listAlbumPhotoViewRowsFn != nil {
		return a.listAlbumPhotoViewRowsFn(ctx, albumID, limit, offset)
	}
	return nil, nil
}

func (a *albumRepoStub) CreateAlbum(ctx context.Context, userID int32, title string) (domain.Album, error) {
	if a.createAlbumFn != nil {
		return a.createAlbumFn(ctx, userID, title)
	}
	return domain.Album{}, nil
}

func (a *albumRepoStub) UpdateAlbum(ctx context.Context, albumId int32, userID int32, title string) (domain.Album, error) {
	if a.updateAlbumFn != nil {
		return a.updateAlbumFn(ctx, albumId, userID, title)
	}
	return domain.Album{}, nil
}

func (a *albumRepoStub) DeleteAlbum(ctx context.Context, albumId int32) error {
	if a.deleteAlbumFn != nil {
		return a.deleteAlbumFn(ctx, albumId)
	}
	return nil
}

type storeStub struct {
	generateURLFn   func(ctx context.Context, key string, expiry time.Duration) (string, error)
	lastWrittenKey  string
	lastWrittenData []byte
	writeErr        error
}

func (s *storeStub) GenerateURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	if s.generateURLFn != nil {
		return s.generateURLFn(ctx, key, expiry)
	}
	return "", nil
}
func (s *storeStub) Read(ctx context.Context, key string) (io.ReadCloser, error) {
	return nil, nil
}
func (s *storeStub) Write(ctx context.Context, key string, file io.Reader) error {
	s.lastWrittenKey = key
	return s.writeErr
}
func (s *storeStub) Delete(ctx context.Context, key string) error {
	return nil
}

type queuePublisherStub struct {
	publishCalls    int
	expectedChannel string
	publishErr      error
}

func (q *queuePublisherStub) Publish(ctx context.Context, channel string, message interface{}) *redis.IntCmd {
	q.publishCalls++
	if q.expectedChannel != "" && channel != q.expectedChannel {
		return redis.NewIntResult(0, fmt.Errorf("unexpected channel: %s", channel))
	}
	if len(message.([]byte)) == 0 {
		return redis.NewIntResult(0, fmt.Errorf("empty message"))
	}
	return redis.NewIntResult(1, q.publishErr)
}

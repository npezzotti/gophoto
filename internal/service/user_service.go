package service

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/npezzotti/gophoto/internal/config"
	"github.com/npezzotti/gophoto/internal/db"
	"github.com/npezzotti/gophoto/internal/domain"
	"github.com/npezzotti/gophoto/internal/utils"
	"github.com/npezzotti/gophoto/pkg/store"
	"golang.org/x/crypto/bcrypt"
)

type UserService struct {
	repo   db.UserRepository
	photos db.PhotoRepository
	store  store.Store
	config *config.Config
}

func NewUserService(r db.UserRepository, p db.PhotoRepository, s store.Store, c *config.Config) *UserService {
	return &UserService{repo: r, photos: p, store: s, config: c}
}

func (s *UserService) defaultProfileImage() domain.ResponsiveImage {
	thumbnailPath := filepath.Join(s.config.StaticDir, domain.DefaultProfileThumbnailPath)
	avatarPath := filepath.Join(s.config.StaticDir, domain.DefaultProfileAvatarPath)

	return domain.ResponsiveImage{
		DefaultSrc: thumbnailPath,
		Sources: []domain.ImageSource{
			{
				Width:  300,
				Height: 300,
				URL:    thumbnailPath,
			},
			{
				Width:  100,
				Height: 100,
				URL:    avatarPath,
			},
		},
	}
}

func (s *UserService) buildProfileImage(ctx context.Context, user domain.User) domain.ResponsiveImage {
	image := s.defaultProfileImage()

	if user.ProfilePictureID == nil {
		// No profile picture set, return default image
		return image
	}

	photo, err := s.photos.GetPhoto(ctx, *user.ProfilePictureID)
	if err != nil || photo.Key == "" {
		return image
	}

	meta, err := s.photos.GetPhotoMetadataByPhotoID(ctx, *user.ProfilePictureID)
	if err != nil {
		return image
	}

	var sources []domain.ImageSource
	var defaultSrc string

	for _, m := range meta {
		if m.Variant == domain.PhotoVariantOriginal {
			continue
		}

		path, err := utils.BuildPhotoPathForVariant(photo.Key, m.Variant, utils.MimeType(m.MimeType))
		if err != nil {
			continue
		}

		url, err := s.store.GenerateURL(ctx, path)
		if err != nil {
			continue
		}

		sources = append(sources, domain.ImageSource{
			Width:  m.Width,
			Height: m.Height,
			URL:    url,
		})

		if defaultSrc == "" || m.Variant == domain.PhotoVariantLarge {
			defaultSrc = url
		}
	}

	if len(sources) == 0 {
		return image
	}

	if defaultSrc == "" {
		// If no large variant is found, use the first available source as the default
		defaultSrc = sources[0].URL
	}

	image.Sources = sources
	image.DefaultSrc = defaultSrc

	return image
}

func (s *UserService) newUserPresentation(ctx context.Context, user domain.User) *domain.UserPresentation {
	return &domain.UserPresentation{
		ID:        user.ID,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Email:     user.Email,
		ProfilePicture: func() domain.ResponsiveImage {
			image := s.buildProfileImage(ctx, user)
			image.Alt = fmt.Sprintf("%s %s's profile picture", user.FirstName, user.LastName)
			return image
		}(),
	}
}

func (s *UserService) GetUserByID(ctx context.Context, id int32) (*domain.UserPresentation, error) {
	user, err := s.repo.GetUserById(ctx, id)
	if err != nil {
		if errors.Is(err, db.ErrUserNotFound) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("error getting user by id: %w", err)
	}

	return s.newUserPresentation(ctx, user), nil
}

func (s *UserService) GetUserByEmail(ctx context.Context, email string) (domain.User, error) {
	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, db.ErrUserNotFound) {
			return domain.User{}, domain.ErrUserNotFound
		}
		return domain.User{}, fmt.Errorf("error getting user by email: %w", err)
	}
	return user, nil
}

func (s *UserService) CreateUser(ctx context.Context, firstName, lastName, email, password string) (*domain.UserPresentation, error) {
	passwdHash, err := hashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("error hashing password: %w", err)
	}

	user, err := s.repo.CreateUser(ctx, firstName, lastName, email, passwdHash)
	if err != nil {
		return nil, fmt.Errorf("error creating user: %w", err)
	}

	return s.newUserPresentation(ctx, user), nil
}

func (s *UserService) UpdateUser(ctx context.Context, userID int32, firstName, lastName, email, password string) (*domain.UserPresentation, error) {
	user, err := s.repo.GetUserById(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("error fetching user for update: %w", err)
	}

	var pwdHash string
	if password != "" {
		hash, err := hashPassword(password)
		if err != nil {
			return nil, fmt.Errorf("error hashing new password: %w", err)
		}
		pwdHash = hash
	} else {
		pwdHash = user.PasswordHash
	}

	updatedUser, err := s.repo.UpdateUser(ctx, domain.UserUpdateParams{
		ID:               userID,
		FirstName:        firstName,
		LastName:         lastName,
		Email:            email,
		PasswordHash:     pwdHash,
		ProfilePictureID: user.ProfilePictureID,
	})
	if err != nil {
		return nil, fmt.Errorf("error updating user: %w", err)
	}

	return s.newUserPresentation(ctx, domain.User{
		ID:               updatedUser.ID,
		FirstName:        updatedUser.FirstName,
		LastName:         updatedUser.LastName,
		Email:            updatedUser.Email,
		PasswordHash:     updatedUser.PasswordHash,
		ProfilePictureID: updatedUser.ProfilePictureID,
	}), nil
}

func (s *UserService) DeleteUser(ctx context.Context, userID int32) error {
	if err := s.repo.DeleteUser(ctx, userID); err != nil {
		if errors.Is(err, db.ErrUserNotFound) {
			return domain.ErrUserNotFound
		}
		return fmt.Errorf("error deleting user: %w", err)
	}
	return nil
}

func (s *UserService) Authenticate(hash, password string) (bool, error) {
	return passwordsMatch(hash, password)
}

func hashPassword(password string) (string, error) {
	passwdBytes := []byte(password)

	hashedPasswdBytes, err := bcrypt.GenerateFromPassword(passwdBytes, bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	return string(hashedPasswdBytes), nil
}

func passwordsMatch(hash, password string) (bool, error) {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err == nil {
		return true, nil
	}

	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		return false, nil
	}

	return false, fmt.Errorf("error comparing password hash: %w", err)
}

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
	"github.com/npezzotti/gophoto/pkg/logging"
	"github.com/npezzotti/gophoto/pkg/store"
	"golang.org/x/crypto/bcrypt"
)

type UserService struct {
	userRepo  db.UserRepository
	photoRepo db.PhotoRepository
	store     store.Store
	config    *config.Config
	logger    *logging.Logger
}

func NewUserService(r db.UserRepository, p db.PhotoRepository, s store.Store, c *config.Config, l *logging.Logger) *UserService {
	return &UserService{userRepo: r, photoRepo: p, store: s, config: c, logger: l}
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

	profilePictureID := *user.ProfilePictureID

	photo, err := s.photoRepo.GetPhoto(ctx, profilePictureID)
	if err != nil {
		if !errors.Is(err, db.ErrPhotoNotFound) {
			s.logger.Warn("no photo found for profile picture user_id=%d profile_picture_id=%d", user.ID, profilePictureID)
		} else {
			s.logger.Warn("failed to get photo user_id=%d profile_picture_id=%d error=%q", user.ID, profilePictureID, err.Error())
		}
		return image
	}

	if photo.Key == "" {
		s.logger.Warn("no key found for profile picture user_id=%d profile_picture_id=%d", user.ID, profilePictureID)

		return image
	}

	meta, err := s.photoRepo.GetPhotoMetadataByPhotoID(ctx, profilePictureID)
	if err != nil {
		if errors.Is(err, db.ErrPhotoMetadataNotFound) {
			s.logger.Warn("no photo metadata found for profile picture user_id=%d profile_picture_id=%d", user.ID, profilePictureID)
			return image
		}
		s.logger.Warn("failed to get photo metadata user_id=%d profile_picture_id=%d error=%q", user.ID, profilePictureID, err.Error())
		return image
	}

	var sources []domain.ImageSource
	var defaultSrc string

	for _, m := range meta {
		if m.Variant == domain.PhotoVariantOriginal {
			continue
		}

		path, err := utils.BuildPhotoPathForVariant(photo.Key, m.Variant, domain.MimeType(m.MimeType))
		if err != nil {
			s.logger.Error("Error building photo path: %s", err)
			continue
		}

		url, err := s.store.GenerateURL(ctx, path, s.config.URLExpiry)
		if err != nil {
			s.logger.Error("Error generating photo URL: %s", err)
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
		s.logger.Warn("no photo variants found for profile picture, falling "+
			"back to default image user_id=%d profile_picture_id=%d", user.ID, profilePictureID)
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
	user, err := s.userRepo.GetUserById(ctx, id)
	if err != nil {
		if errors.Is(err, db.ErrUserNotFound) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("error getting user by id: %w", err)
	}

	return s.newUserPresentation(ctx, user), nil
}

func (s *UserService) GetUserByEmail(ctx context.Context, email string) (domain.User, error) {
	user, err := s.userRepo.GetUserByEmail(ctx, email)
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

	user, err := s.userRepo.CreateUser(ctx, firstName, lastName, email, passwdHash)
	if err != nil {
		return nil, fmt.Errorf("error creating user: %w", err)
	}

	return s.newUserPresentation(ctx, user), nil
}

func (s *UserService) UpdateUser(ctx context.Context, userID int32, firstName, lastName, email, password string) (*domain.UserPresentation, error) {
	user, err := s.userRepo.GetUserById(ctx, userID)
	if err != nil {
		if errors.Is(err, db.ErrUserNotFound) {
			return nil, domain.ErrUserNotFound
		}
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

	updatedUser, err := s.userRepo.UpdateUser(ctx, domain.UserUpdateParams{
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

// DeleteUser deletes the user with the given ID. This also cascades to delete all albums and album_photos records
// for the user. Photos are not immediately deleted, but will be cleaned up by the storage cleaner worker.
func (s *UserService) DeleteUser(ctx context.Context, userID int32) error {
	if err := s.userRepo.DeleteUser(ctx, userID); err != nil {
		if errors.Is(err, db.ErrUserNotFound) {
			return domain.ErrUserNotFound
		}
		return fmt.Errorf("error deleting user: %w", err)
	}
	return nil
}

func (s *UserService) AuthenticateByEmail(ctx context.Context, email, password string) (domain.User, error) {
	user, err := s.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return domain.User{}, domain.ErrInvalidCredentials
		}
		return domain.User{}, fmt.Errorf("error fetching user for authentication: %w", err)
	}

	authenticated, err := passwordsMatch(user.PasswordHash, password)
	if err != nil {
		s.logger.Error("error comparing password hash for user ID %d: %v", user.ID, err)
		return domain.User{}, fmt.Errorf("error authenticating user: %w", err)
	}
	if authenticated {
		return user, nil
	}
	return domain.User{}, domain.ErrInvalidCredentials
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

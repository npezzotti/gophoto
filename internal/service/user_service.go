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
	store  store.Store
	config *config.Config
}

func NewUserService(r db.UserRepository, s store.Store, c *config.Config) *UserService {
	return &UserService{repo: r, store: s, config: c}
}

func (s *UserService) newUserResponse(ctx context.Context, user domain.User) *domain.UserResponse {
	var sources []domain.ImageSource
	var defaultSrc string
	if user.ProfilePictureKey != nil && user.ProfilePictureID != nil {
		meta, err := s.repo.GetPhotoMetadataByPhotoID(ctx, *user.ProfilePictureID)
		if err != nil {
			return nil
		}

		for _, m := range meta {
			// Skip original variant as it's not meant to be directly served
			if m.Variant == domain.PhotoVariantOriginal {
				continue
			}

			path, err := utils.BuildPhotoPathForVariant(*user.ProfilePictureKey, m.Variant, utils.MimeType(m.MimeType))
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

			// Set default source for medium variant
			if m.Variant == domain.PhotoVariantMedium {
				defaultSrc = url
			}
		}
	}

	if len(sources) == 0 {
		thumbnailPath := filepath.Join(s.config.StaticDir, domain.DefaultProfileThumbnailPath)
		sources = append(sources,
			domain.ImageSource{
				Width:  300,
				Height: 300,
				URL:    thumbnailPath,
			},
			domain.ImageSource{
				Width:  100,
				Height: 100,
				URL:    filepath.Join(s.config.StaticDir, domain.DefaultProfileAvatarPath),
			},
		)

		defaultSrc = thumbnailPath
	}

	return &domain.UserResponse{
		ID:        user.ID,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Email:     user.Email,
		ProfilePicture: domain.ResponsiveImage{
			Alt:        fmt.Sprintf("%s %s's profile picture", user.FirstName, user.LastName),
			Sources:    sources,
			DefaultSrc: defaultSrc,
		},
	}
}

func (s *UserService) GetUserByID(ctx context.Context, id int32) (*domain.UserResponse, error) {
	user, err := s.repo.GetUserById(ctx, id)
	if err != nil {
		if errors.Is(err, db.ErrUserNotFound) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("error getting user by id: %w", err)
	}

	userResp := s.newUserResponse(ctx, user)

	return userResp, nil
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

func (s *UserService) CreateUser(ctx context.Context, firstName, lastName, email, password string) (domain.UserResponse, error) {
	passwdHash, err := hashPassword(password)
	if err != nil {
		return domain.UserResponse{}, fmt.Errorf("error hashing password: %w", err)
	}

	user, err := s.repo.CreateUser(ctx, firstName, lastName, email, passwdHash)
	if err != nil {
		return domain.UserResponse{}, fmt.Errorf("error creating user: %w", err)
	}

	return user, nil
}

func (s *UserService) UpdateUser(ctx context.Context, userID int32, firstName, lastName, email, password string) (domain.UserResponse, error) {
	dbUser, err := s.repo.GetUserById(ctx, userID)
	if err != nil {
		return domain.UserResponse{}, fmt.Errorf("error fetching user for update: %w", err)
	}

	var pwdHash string
	if password != "" {
		hash, err := hashPassword(password)
		if err != nil {
			return domain.UserResponse{}, fmt.Errorf("error hashing new password: %w", err)
		}
		pwdHash = hash
	} else {
		pwdHash = dbUser.PasswordHash
	}

	updatedUser, err := s.repo.UpdateUser(ctx, domain.UserUpdateParams{
		ID:               userID,
		FirstName:        firstName,
		LastName:         lastName,
		Email:            email,
		PasswordHash:     pwdHash,
		ProfilePictureID: dbUser.ProfilePictureID,
	})
	if err != nil {
		return domain.UserResponse{}, fmt.Errorf("error updating user: %w", err)
	}

	resp := s.newUserResponse(ctx, domain.User{
		ID:                updatedUser.ID,
		FirstName:         updatedUser.FirstName,
		LastName:          updatedUser.LastName,
		Email:             updatedUser.Email,
		PasswordHash:      updatedUser.PasswordHash,
		ProfilePictureID:  dbUser.ProfilePictureID,
		ProfilePictureKey: dbUser.ProfilePictureKey,
	})
	if resp == nil {
		return domain.UserResponse{}, fmt.Errorf("error preparing updated user response")
	}

	return *resp, nil
}

func (s *UserService) DeleteUser(ctx context.Context, userID int32) error {
	if err := s.repo.DeleteUser(ctx, userID); err != nil {
		return fmt.Errorf("error deleting user: %w", err)
	}
	return nil
}

func (s *UserService) UserExists(ctx context.Context, userID int32) (bool, error) {
	return s.repo.UserExists(ctx, userID)
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

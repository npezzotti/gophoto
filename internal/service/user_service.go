package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/npezzotti/gophoto/internal/db"
	"golang.org/x/crypto/bcrypt"
)

type UserService struct {
	repo *db.Repository
}

func NewUserService(r *db.Repository) *UserService {
	return &UserService{repo: r}
}

func (s *UserService) GetUserByID(ctx context.Context, id int32) (db.GetUserByIdRow, error) {
	user, err := s.repo.GetUserById(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return db.GetUserByIdRow{}, fmt.Errorf("user not found: %w", err)
		}
		return db.GetUserByIdRow{}, fmt.Errorf("error getting user by id: %w", err)
	}
	return user, nil
}

func (s *UserService) GetUserByEmail(ctx context.Context, email string) (db.User, error) {
	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return db.User{}, fmt.Errorf("user not found: %w", err)
		}
		return db.User{}, fmt.Errorf("error getting user by email: %w", err)
	}
	return user, nil
}

func (s *UserService) CreateUser(ctx context.Context, firstName, lastName, email, password string) (db.User, error) {
	passwdHash, err := hashPassword(password)
	if err != nil {
		return db.User{}, fmt.Errorf("error hashing password: %w", err)
	}

	user, err := s.repo.CreateUser(ctx, db.CreateUserParams{
		FirstName:    firstName,
		LastName:     lastName,
		Email:        email,
		PasswordHash: passwdHash,
	})
	if err != nil {
		return db.User{}, fmt.Errorf("error creating user: %w", err)
	}

	return user, nil
}

func (s *UserService) UpdateUser(ctx context.Context, user *db.GetUserByIdRow, firstName, lastName, email, password string) (*db.GetUserByIdRow, error) {
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

	_, err := s.repo.UpdateUser(ctx, db.UpdateUserParams{
		ID:               user.ID,
		FirstName:        firstName,
		LastName:         lastName,
		Email:            email,
		PasswordHash:     pwdHash,
		ProfilePictureID: sql.NullInt32{Int32: user.ProfilePictureID.Int32, Valid: user.ProfilePictureID.Valid},
		UpdatedAt:        time.Now(),
	})
	if err != nil {
		return nil, fmt.Errorf("error updating user: %w", err)
	}
	return user, nil
}

func (s *UserService) DeleteUser(ctx context.Context, userID int32) error {
	if err := s.repo.DeleteUser(ctx, userID); err != nil {
		return fmt.Errorf("error deleting user: %w", err)
	}
	return nil
}

func hashPassword(password string) (string, error) {
	passwdBytes := []byte(password)

	hashedPasswdBytes, err := bcrypt.GenerateFromPassword(passwdBytes, bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	return string(hashedPasswdBytes), nil
}

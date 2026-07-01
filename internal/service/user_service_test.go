package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/npezzotti/gophoto/internal/config"
	"github.com/npezzotti/gophoto/internal/db"
	"github.com/npezzotti/gophoto/internal/domain"
	"github.com/npezzotti/gophoto/pkg/logging"
)

const hash = "$2a$10$eOHnK/9wnbnsI553DW3uneF4dT.V593i8p1gGTL6Ua5ZtxK9ABoIq" // bcrypt hash for "password"

func newTestLogger() *logging.Logger {
	return logging.NewLogger(io.Discard, false)
}

func TestUserService_GetUserByID(t *testing.T) {
	tcases := []struct {
		name          string
		userRepo      *userRepoStub
		id            int32
		expected      *domain.UserPresentation
		expectedErr   error
		expectedMatch string
	}{
		{
			name: "Valid user",
			userRepo: &userRepoStub{
				getByIDFn: func(ctx context.Context, id int32) (domain.User, error) {
					t.Helper()
					return domain.User{ID: id}, nil
				},
			},
			id:            1,
			expected:      &domain.UserPresentation{ID: 1},
			expectedErr:   nil,
			expectedMatch: "",
		},
		{
			name: "User not found",
			userRepo: &userRepoStub{
				getByIDFn: func(ctx context.Context, id int32) (domain.User, error) {
					return domain.User{}, db.ErrUserNotFound
				},
			},
			id:            1,
			expected:      nil,
			expectedErr:   domain.ErrUserNotFound,
			expectedMatch: "",
		},
		{
			name: "Internal error",
			userRepo: &userRepoStub{
				getByIDFn: func(ctx context.Context, id int32) (domain.User, error) {
					return domain.User{}, errors.New("internal error")
				},
			},
			id:            1,
			expected:      nil,
			expectedErr:   nil,
			expectedMatch: "error getting user by id: internal error",
		},
	}

	for _, tt := range tcases {
		userSvc := NewUserService(tt.userRepo, &photoRepoStub{}, nil, &config.Config{}, newTestLogger())
		res, err := userSvc.GetUserByID(context.Background(), tt.id)
		if tt.expectedErr != nil {
			if !errors.Is(err, tt.expectedErr) {
				t.Fatalf("expected error %v, got %v", tt.expectedErr, err)
			}
			continue
		}
		if tt.expectedMatch != "" {
			if err == nil || !strings.Contains(err.Error(), tt.expectedMatch) {
				t.Fatalf("expected error to contain %q, got %v", tt.expectedMatch, err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.ID != tt.expected.ID {
			t.Errorf("Expected user with ID %d, got %v", tt.expected.ID, res)
		}
	}
}

func TestUserService_GetUserByEmail(t *testing.T) {
	t.Parallel()
	tcases := []struct {
		name          string
		userRepo      *userRepoStub
		email         string
		expected      *domain.User
		expectedErr   error
		expectedMatch string
	}{
		{
			name: "Valid user",
			userRepo: &userRepoStub{
				getByEmailFn: func(ctx context.Context, email string) (domain.User, error) {
					t.Helper()
					return domain.User{Email: email}, nil
				},
			},
			email:         "test@example.com",
			expected:      &domain.User{Email: "test@example.com"},
			expectedErr:   nil,
			expectedMatch: "",
		},
		{
			name: "User not found",
			userRepo: &userRepoStub{
				getByEmailFn: func(ctx context.Context, email string) (domain.User, error) {
					return domain.User{}, db.ErrUserNotFound
				},
			},
			email:         "test@example.com",
			expected:      nil,
			expectedErr:   domain.ErrUserNotFound,
			expectedMatch: "",
		},
		{
			name: "Internal error",
			userRepo: &userRepoStub{
				getByEmailFn: func(ctx context.Context, email string) (domain.User, error) {
					return domain.User{}, errors.New("internal error")
				},
			},
			email:         "test@example.com",
			expected:      nil,
			expectedErr:   nil,
			expectedMatch: "error getting user by email",
		},
	}

	for _, tt := range tcases {
		userSvc := NewUserService(tt.userRepo, &photoRepoStub{}, nil, &config.Config{}, newTestLogger())
		res, err := userSvc.GetUserByEmail(context.Background(), tt.email)
		if tt.expectedErr != nil {
			if !errors.Is(err, tt.expectedErr) {
				t.Fatalf("expected error %v, got %v", tt.expectedErr, err)
			}
			continue
		}
		if tt.expectedMatch != "" {
			if err == nil || !strings.Contains(err.Error(), tt.expectedMatch) {
				t.Fatalf("expected error to contain %q, got %v", tt.expectedMatch, err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Email != tt.expected.Email {
			t.Errorf("Expected user with email %s, got %v", tt.expected.Email, res)
		}
	}
}

func TestUserService_CreateUser(t *testing.T) {
	tcases := []struct {
		name             string
		userRepo         *userRepoStub
		firstName        string
		lastName         string
		email            string
		password         string
		hashFn           func(password string) (string, error)
		expectedErrMatch string
	}{
		{
			name: "Valid user",
			userRepo: &userRepoStub{
				createFn: func(ctx context.Context, firstName, lastName, email, passwordHash string) (domain.User, error) {
					return domain.User{ID: 1, FirstName: firstName, LastName: lastName, Email: email}, nil
				},
			},
			firstName: "test",
			lastName:  "test",
			email:     "test@example.com",
			password:  "password123",
		},
		{
			name: "Hashing error",
			userRepo: &userRepoStub{
				createFn: func(ctx context.Context, firstName, lastName, email, passwordHash string) (domain.User, error) {
					return domain.User{}, nil
				},
			},
			firstName: "test",
			lastName:  "test",
			email:     "test@example.com",
			password:  "password123",
			hashFn: func(password string) (string, error) {
				return "", fmt.Errorf("internal error")
			},
			expectedErrMatch: "error hashing password: internal error",
		},
		{
			name: "repo error",
			userRepo: &userRepoStub{
				createFn: func(ctx context.Context, firstName, lastName, email, passwordHash string) (domain.User, error) {
					return domain.User{}, errors.New("internal error")
				},
			},
			firstName:        "test",
			lastName:         "test",
			email:            "test@example.com",
			password:         "password123",
			expectedErrMatch: "error creating user",
		},
	}

	for _, tt := range tcases {
		t.Run(tt.name, func(t *testing.T) {
			userSvc := NewUserService(tt.userRepo, &photoRepoStub{}, nil, &config.Config{}, newTestLogger())
			if tt.hashFn != nil {
				userSvc.hashFn = tt.hashFn
			}

			resp, err := userSvc.CreateUser(context.Background(), tt.firstName, tt.lastName, tt.email, tt.password)
			if tt.expectedErrMatch != "" {
				if err == nil || !strings.Contains(err.Error(), tt.expectedErrMatch) {
					t.Fatalf("expected error to contain %q, got %v", tt.expectedErrMatch, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp == nil {
				t.Fatalf("expected non-nil response")
			}
			if resp.ID == 0 {
				t.Fatalf("expected non-zero user ID")
			}
			if resp.Email != tt.email {
				t.Fatalf("expected email %s, got %s", tt.email, resp.Email)
			}
			if resp.FirstName != tt.firstName {
				t.Fatalf("expected first name %s, got %s", tt.firstName, resp.FirstName)
			}
			if resp.LastName != tt.lastName {
				t.Fatalf("expected last name %s, got %s", tt.lastName, resp.LastName)
			}
		})
	}
}

func TestUserService_UpdateUser(t *testing.T) {
	tcases := []struct {
		name          string
		userRepo      *userRepoStub
		id            int32
		firstName     string
		lastName      string
		email         string
		password      string
		hashFn        func(password string) (string, error)
		expectedErr   error
		expectedMatch string
		assert        func(t *testing.T, user *domain.UserPresentation)
	}{
		{
			name: "Valid update",
			userRepo: &userRepoStub{
				getByIDFn: func(ctx context.Context, id int32) (domain.User, error) {
					return domain.User{ID: id, FirstName: "OldFirstName", LastName: "OldLastName", Email: "old@example.com"}, nil
				},
				updateFn: func(ctx context.Context, params domain.UserUpdateParams) (domain.User, error) {
					return domain.User{ID: params.ID, FirstName: params.FirstName, LastName: params.LastName, Email: params.Email}, nil
				},
			},
			id:        1,
			firstName: "NewFirstName",
			lastName:  "NewLastName",
			email:     "new@example.com",
			password:  "password123",
			assert: func(t *testing.T, user *domain.UserPresentation) {
				if user.FirstName != "NewFirstName" || user.LastName != "NewLastName" || user.Email != "new@example.com" {
					t.Fatalf("expected user to be updated correctly, got: %+v", user)
				}
			},
		},
		{
			name: "User not found",
			userRepo: &userRepoStub{
				getByIDFn: func(ctx context.Context, id int32) (domain.User, error) {
					return domain.User{}, db.ErrUserNotFound
				},
			},
			id:            1,
			firstName:     "NewFirstName",
			lastName:      "NewLastName",
			email:         "new@example.com",
			expectedErr:   domain.ErrUserNotFound,
			expectedMatch: "",
		},
		{
			name: "Internal error on get",
			userRepo: &userRepoStub{
				getByIDFn: func(ctx context.Context, id int32) (domain.User, error) {
					return domain.User{}, errors.New("internal error")
				},
			},
			id:            1,
			firstName:     "NewFirstName",
			lastName:      "NewLastName",
			email:         "new@example.com",
			expectedMatch: "error fetching user for update",
		},
		{
			name: "Hashing new password fails",
			userRepo: &userRepoStub{
				getByIDFn: func(ctx context.Context, id int32) (domain.User, error) {
					return domain.User{ID: id, PasswordHash: "existing-hash"}, nil
				},
				updateFn: func(ctx context.Context, params domain.UserUpdateParams) (domain.User, error) {
					t.Fatalf("update should not be called when hashing fails")
					return domain.User{}, nil
				},
			},
			id:        1,
			firstName: "NewFirstName",
			lastName:  "NewLastName",
			email:     "new@example.com",
			password:  "new-password",
			hashFn: func(password string) (string, error) {
				return "", fmt.Errorf("internal error")
			},
			expectedMatch: "error hashing new password: internal error",
		},
		{
			name: "Internal error on update",
			userRepo: &userRepoStub{
				getByIDFn: func(ctx context.Context, id int32) (domain.User, error) {
					return domain.User{ID: id, FirstName: "OldFirstName", LastName: "OldLastName", Email: "old@example.com"}, nil
				},
				updateFn: func(ctx context.Context, params domain.UserUpdateParams) (domain.User, error) {
					return domain.User{}, errors.New("internal error")
				},
			},
			id:            1,
			firstName:     "NewFirstName",
			lastName:      "NewLastName",
			email:         "new@example.com",
			expectedMatch: "error updating user",
		},
	}

	for _, tt := range tcases {
		t.Run(tt.name, func(t *testing.T) {
			userSvc := NewUserService(tt.userRepo, &photoRepoStub{}, nil, &config.Config{}, newTestLogger())
			if tt.hashFn != nil {
				userSvc.hashFn = tt.hashFn
			}
			res, err := userSvc.UpdateUser(context.Background(), tt.id, tt.firstName, tt.lastName, tt.email, tt.password)
			if tt.expectedErr != nil {
				if !errors.Is(err, tt.expectedErr) {
					t.Fatalf("expected error %v, got %v", tt.expectedErr, err)
				}
				return
			}
			if tt.expectedMatch != "" {
				if err == nil || !strings.Contains(err.Error(), tt.expectedMatch) {
					t.Fatalf("expected error to contain %q, got %s", tt.expectedMatch, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.assert != nil {
				tt.assert(t, res)
			}
		})
	}
}

func TestUserService_DeleteUser(t *testing.T) {
	tcases := []struct {
		name          string
		userRepo      *userRepoStub
		id            int32
		expectedErr   error
		expectedMatch string
	}{
		{
			name: "Valid delete",
			userRepo: &userRepoStub{
				deleteFn: func(ctx context.Context, userID int32) error {
					return nil
				},
			},
			id: 1,
		},
		{
			name: "User not found",
			userRepo: &userRepoStub{
				deleteFn: func(ctx context.Context, userID int32) error {
					return db.ErrUserNotFound
				},
			},
			id:            1,
			expectedErr:   domain.ErrUserNotFound,
			expectedMatch: "",
		},
		{
			name: "Internal error on delete",
			userRepo: &userRepoStub{
				deleteFn: func(ctx context.Context, userID int32) error {
					return errors.New("internal error")
				},
			},
			id:            1,
			expectedMatch: "error deleting user",
		},
	}

	for _, tt := range tcases {
		t.Run(tt.name, func(t *testing.T) {
			userSvc := NewUserService(tt.userRepo, &photoRepoStub{}, nil, &config.Config{}, newTestLogger())
			err := userSvc.DeleteUser(context.Background(), tt.id)
			if tt.expectedErr != nil {
				if !errors.Is(err, tt.expectedErr) {
					t.Fatalf("expected error %v, got %v", tt.expectedErr, err)
				}
				return
			}
			if tt.expectedMatch != "" {
				if err == nil || !strings.Contains(err.Error(), tt.expectedMatch) {
					t.Fatalf("expected error to contain %q, got %v", tt.expectedMatch, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestUserService_AuthenticateByEmail(t *testing.T) {
	tcases := []struct {
		name          string
		userRepo      *userRepoStub
		email         string
		password      string
		matchFn       func(hashedPassword, password string) (bool, error)
		expectedErr   error
		expectedMatch string
	}{
		{
			name: "Valid authentication",
			userRepo: &userRepoStub{
				getByEmailFn: func(ctx context.Context, email string) (domain.User, error) {
					return domain.User{Email: email, PasswordHash: hash}, nil
				},
			},
			email:    "test@example.com",
			password: "password",
		},
		{
			name: "Invalid credentials",
			userRepo: &userRepoStub{
				getByEmailFn: func(ctx context.Context, email string) (domain.User, error) {
					return domain.User{Email: email, PasswordHash: hash}, nil
				},
			},
			email:       "test@example.com",
			password:    "wrongpassword",
			expectedErr: domain.ErrInvalidCredentials,
		},
		{
			name: "User not found",
			userRepo: &userRepoStub{
				getByEmailFn: func(ctx context.Context, email string) (domain.User, error) {
					return domain.User{}, db.ErrUserNotFound
				},
			},
			email:       "test@example.com",
			password:    "password",
			expectedErr: domain.ErrInvalidCredentials,
		},
		{
			name: "Internal error fetching user",
			userRepo: &userRepoStub{
				getByEmailFn: func(ctx context.Context, email string) (domain.User, error) {
					return domain.User{}, errors.New("internal error")
				},
			},
			email:         "test@example.com",
			password:      "password",
			expectedMatch: "error fetching user for authentication",
		},
		{
			name: "Internal error comparing password",
			userRepo: &userRepoStub{
				getByEmailFn: func(ctx context.Context, email string) (domain.User, error) {
					return domain.User{Email: email, PasswordHash: hash}, nil
				},
			},
			email:    "test@example.com",
			password: "password",
			matchFn: func(hashedPassword, password string) (bool, error) {
				return false, fmt.Errorf("internal error")
			},
			expectedMatch: "error authenticating user",
		},
	}

	for _, tt := range tcases {
		userRepo := &userRepoStub{
			getByEmailFn: func(ctx context.Context, email string) (domain.User, error) {
				return domain.User{Email: email, PasswordHash: hash}, nil
			},
		}
		userSvc := NewUserService(userRepo, &photoRepoStub{}, nil, &config.Config{}, newTestLogger())

		user, err := userSvc.AuthenticateByEmail(context.Background(), tt.email, tt.password)
		if tt.expectedErr != nil {
			if !errors.Is(err, tt.expectedErr) {
				t.Fatalf("expected error %v, got %v", tt.expectedErr, err)
			}
			return
		}
		if tt.expectedMatch != "" {
			if err == nil || !strings.Contains(err.Error(), tt.expectedMatch) {
				t.Fatalf("expected error to contain %q, got %v", tt.expectedMatch, err)
			}
			return
		}
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.Email != tt.email {
			t.Fatalf("expected email %q, got %v", tt.email, user.Email)
		}
	}
}

func Test_hashPassword(t *testing.T) {
	password := "password"
	hash, err := hashPassword(password)
	if err != nil {
		t.Fatalf("unexpected error hashing password: %v", err)
	}
	if match, err := passwordsMatch(hash, password); err != nil || !match {
		t.Fatalf("expected passwords to match, got match=%v, err=%v", match, err)
	}
}

func Test_passwordsMatch(t *testing.T) {
	t.Run("Valid match", func(t *testing.T) {
		if match, err := passwordsMatch(hash, "password"); err != nil || !match {
			t.Fatalf("expected passwords to match, got match=%v, err=%v", match, err)
		}
	})

	t.Run("Invalid match", func(t *testing.T) {
		if match, err := passwordsMatch(hash, "wrongpassword"); err != nil || match {
			t.Fatalf("expected passwords not to match, got match=%v, err=%v", match, err)
		}
	})
}

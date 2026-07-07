package web

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/npezzotti/gophoto/internal/config"
	"github.com/npezzotti/gophoto/internal/domain"
	"github.com/npezzotti/gophoto/pkg/logging"
)

// errDeleteStore wraps an in-memory scs.Store and makes Delete always fail.
type errDeleteStore struct {
	inner scs.Store
}

func (s errDeleteStore) Find(token string) ([]byte, bool, error) { return s.inner.Find(token) }
func (s errDeleteStore) Commit(token string, b []byte, expiry time.Time) error {
	return s.inner.Commit(token, b, expiry)
}
func (s errDeleteStore) Delete(token string) error { return errors.New("store error") }

func Test_signupHandler_GET(t *testing.T) {
	app := &application{
		sessionManager: scs.New(),
		Logger:         logging.NewLogger(io.Discard, false),
		config:         &config.Config{},
	}

	req := httptest.NewRequest(http.MethodGet, "/signup", nil)
	rr := httptest.NewRecorder()
	req = withAuthenticatedUser(req, &domain.UserPresentation{ID: 1})

	app.sessionManager.LoadAndSave(http.HandlerFunc(app.signupHandler)).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func Test_signupHandler_POST(t *testing.T) {
	tcases := []struct {
		name               string
		formData           url.Values
		userServiceStub    *userServiceStub
		expectedStatusCode int
		expectedLocation   string
		flash              *Flash
	}{
		{
			name: "valid form data",
			formData: url.Values{
				"first_name":       {"John"},
				"last_name":        {"Doe"},
				"email":            {"john.doe@example.com"},
				"password":         {"password123"},
				"confirm_password": {"password123"},
			},
			expectedStatusCode: http.StatusSeeOther,
			expectedLocation:   "/login",
			userServiceStub: &userServiceStub{
				createUserFn: func(ctx context.Context, firstName, lastName, email, password string) (*domain.UserPresentation, error) {
					return &domain.UserPresentation{
						ID:        1,
						FirstName: firstName,
						LastName:  lastName,
						Email:     email,
					}, nil
				},
			},
			flash: &Flash{
				Message: "Account successfully created.",
				Level:   flashInfo,
			},
		},
		{
			name: "invalid form data",
			formData: url.Values{
				"first_name":       {""},
				"last_name":        {"Doe"},
				"email":            {"john.doe@example.com"},
				"password":         {"password123"},
				"confirm_password": {"password123"},
			},
			expectedStatusCode: http.StatusBadRequest,
			userServiceStub:    &userServiceStub{},
		},
		{
			name: "user already exists",
			formData: url.Values{
				"first_name":       {"John"},
				"last_name":        {"Doe"},
				"email":            {"john.doe@example.com"},
				"password":         {"password123"},
				"confirm_password": {"password123"},
			},
			expectedStatusCode: http.StatusSeeOther,
			expectedLocation:   "/login",
			userServiceStub: &userServiceStub{
				createUserFn: func(ctx context.Context, firstName, lastName, email, password string) (*domain.UserPresentation, error) {
					return nil, domain.ErrUserAlreadyExists
				},
			},
			flash: &Flash{
				Message: "An account with this email already exists.",
				Level:   flashErr,
			},
		},
		{
			name: "user service error",
			formData: url.Values{
				"first_name":       {"John"},
				"last_name":        {"Doe"},
				"email":            {"john.doe@example.com"},
				"password":         {"password123"},
				"confirm_password": {"password123"},
			},
			expectedStatusCode: http.StatusSeeOther,
			expectedLocation:   "/signup",
			userServiceStub: &userServiceStub{
				createUserFn: func(ctx context.Context, firstName, lastName, email, password string) (*domain.UserPresentation, error) {
					return nil, errors.New("user service error")
				},
			},
			flash: &Flash{
				Message: "Bad Request",
				Level:   flashErr,
			},
		},
	}

	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			app := &application{
				sessionManager: scs.New(),
				Logger:         logging.NewLogger(io.Discard, false),
				config:         &config.Config{},
				userService:    tc.userServiceStub,
			}

			req := httptest.NewRequest(http.MethodPost, "/signup", strings.NewReader(tc.formData.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rr := httptest.NewRecorder()

			app.sessionManager.LoadAndSave(http.HandlerFunc(app.signupHandler)).ServeHTTP(rr, req)
			resp := rr.Result()
			defer resp.Body.Close()

			if resp.StatusCode != tc.expectedStatusCode {
				t.Fatalf("expected status %d, got %d", tc.expectedStatusCode, resp.StatusCode)
			}

			if tc.expectedLocation != "" {
				if resp.Header.Get("Location") != tc.expectedLocation {
					t.Fatalf("expected redirect to %s, got %s", tc.expectedLocation, resp.Header.Get("Location"))
				}
			}

			if tc.flash != nil {
				validateFlashInSession(t, app, req, resp, tc.flash.Message, tc.flash.Level)
			}
		})
	}
}

func Test_profileHandler(t *testing.T) {
	tcases := []struct {
		name               string
		method             string
		expectedStatusCode int
	}{
		{
			name:               "GET returns 200",
			method:             http.MethodGet,
			expectedStatusCode: http.StatusOK,
		},
		{
			name:               "POST returns 405",
			method:             http.MethodPost,
			expectedStatusCode: http.StatusMethodNotAllowed,
		},
	}

	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			app := &application{
				sessionManager: scs.New(),
				Logger:         logging.NewLogger(io.Discard, false),
				config:         &config.Config{},
			}

			req := httptest.NewRequest(tc.method, "/profile", nil)
			req = withAuthenticatedUser(req, &domain.UserPresentation{ID: 1})
			rr := httptest.NewRecorder()

			app.sessionManager.LoadAndSave(http.HandlerFunc(app.profileHandler)).ServeHTTP(rr, req)

			if rr.Code != tc.expectedStatusCode {
				t.Fatalf("expected status %d, got %d", tc.expectedStatusCode, rr.Code)
			}
		})
	}
}

func Test_deleteAccountHandler(t *testing.T) {
	tcases := []struct {
		name               string
		method             string
		user               *domain.UserPresentation
		userServiceStub    *userServiceStub
		expectedStatusCode int
		expectedLocation   string
		flash              *Flash
	}{
		{
			name:               "non-POST method returns 405",
			method:             http.MethodGet,
			user:               &domain.UserPresentation{ID: 1},
			userServiceStub:    &userServiceStub{},
			expectedStatusCode: http.StatusMethodNotAllowed,
		},
		{
			name:               "no user in context redirects to login",
			method:             http.MethodPost,
			user:               nil,
			userServiceStub:    &userServiceStub{},
			expectedStatusCode: http.StatusSeeOther,
			expectedLocation:   "/login",
			flash: &Flash{
				Message: "User not found.",
				Level:   flashErr,
			},
		},
		{
			name:   "DeleteUser returns ErrUserNotFound redirects to login",
			method: http.MethodPost,
			user:   &domain.UserPresentation{ID: 1},
			userServiceStub: &userServiceStub{
				deleteUserFn: func(ctx context.Context, userID int32) error {
					return domain.ErrUserNotFound
				},
			},
			expectedStatusCode: http.StatusSeeOther,
			expectedLocation:   "/login",
			flash: &Flash{
				Message: "User not found.",
				Level:   flashErr,
			},
		},
		{
			name:   "unexpected service error redirects to profile",
			method: http.MethodPost,
			user:   &domain.UserPresentation{ID: 1},
			userServiceStub: &userServiceStub{
				deleteUserFn: func(ctx context.Context, userID int32) error {
					return errors.New("db error")
				},
			},
			expectedStatusCode: http.StatusSeeOther,
			expectedLocation:   "/profile",
			flash: &Flash{
				Message: "Error deleting account. Please try again.",
				Level:   flashErr,
			},
		},
		{
			name:               "success redirects to login with info flash",
			method:             http.MethodPost,
			user:               &domain.UserPresentation{ID: 1},
			userServiceStub:    &userServiceStub{},
			expectedStatusCode: http.StatusSeeOther,
			expectedLocation:   "/login",
			flash: &Flash{
				Message: "Your account has been deleted.",
				Level:   flashInfo,
			},
		},
	}

	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			app := &application{
				sessionManager: scs.New(),
				Logger:         logging.NewLogger(io.Discard, false),
				config:         &config.Config{},
				userService:    tc.userServiceStub,
			}

			req := httptest.NewRequest(tc.method, "/account/delete", nil)
			if tc.user != nil {
				req = withAuthenticatedUser(req, tc.user)
			}
			rr := httptest.NewRecorder()

			app.sessionManager.LoadAndSave(http.HandlerFunc(app.deleteAccountHandler)).ServeHTTP(rr, req)
			resp := rr.Result()
			defer resp.Body.Close()

			if resp.StatusCode != tc.expectedStatusCode {
				t.Fatalf("expected status %d, got %d", tc.expectedStatusCode, resp.StatusCode)
			}

			if tc.expectedLocation != "" {
				if resp.Header.Get("Location") != tc.expectedLocation {
					t.Fatalf("expected redirect to %s, got %s", tc.expectedLocation, resp.Header.Get("Location"))
				}
			}

			if tc.flash != nil {
				validateFlashInSession(t, app, req, resp, tc.flash.Message, tc.flash.Level)
			}
		})
	}
}

func Test_loginHandler_GET(t *testing.T) {
	tcases := []struct {
		name               string
		user               *domain.UserPresentation
		expectedStatusCode int
		expectedLocation   string
	}{
		{
			name:               "login with authenticated session redirects",
			user:               &domain.UserPresentation{ID: 1},
			expectedStatusCode: http.StatusSeeOther,
			expectedLocation:   "/albums",
		},
		{
			name:               "regular get renders login page",
			expectedStatusCode: http.StatusOK,
		},
	}

	for _, tt := range tcases {
		t.Run(tt.name, func(t *testing.T) {
			app := &application{
				sessionManager: scs.New(),
				Logger:         logging.NewLogger(io.Discard, false),
				config:         &config.Config{},
			}

			req := httptest.NewRequest(http.MethodGet, "/login", nil)
			rr := httptest.NewRecorder()
			if tt.user != nil {
				req = withAuthenticatedUser(req, tt.user)
			}

			app.sessionManager.LoadAndSave(http.HandlerFunc(app.loginHandler)).ServeHTTP(rr, req)
			resp := rr.Result()

			if resp.StatusCode != tt.expectedStatusCode {
				t.Fatalf("expected status code %d, got %d", tt.expectedStatusCode, resp.StatusCode)
			}

			if tt.expectedLocation != "" {
				if location := resp.Header.Get("Location"); location != tt.expectedLocation {
					t.Fatalf("expected redirect to %q, got %q", tt.expectedLocation, location)
				}
			}
		})
	}
}

func Test_loginHandler_POST(t *testing.T) {
	tcases := []struct {
		name               string
		method             string
		email              string
		password           string
		userServiceStub    *userServiceStub
		expectedStatusCode int
		expectedLocation   string
		flash              *Flash
	}{
		{
			name:     "successful authentication",
			email:    "john.doe@example.com",
			password: "password123",
			userServiceStub: &userServiceStub{
				authenticateByEmailFn: func(ctx context.Context, email, password string) (domain.User, error) {
					return domain.User{ID: 1}, nil
				},
			},
			expectedStatusCode: http.StatusSeeOther,
			expectedLocation:   "/albums",
		},
	}

	for _, tt := range tcases {
		t.Run(tt.name, func(t *testing.T) {
			app := &application{
				sessionManager: scs.New(),
				Logger:         logging.NewLogger(os.Stdout, false),
				config:         &config.Config{},
				userService:    tt.userServiceStub,
			}

			formData := url.Values{}
			formData.Set("email", tt.email)
			formData.Set("password", tt.password)
			req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(formData.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rr := httptest.NewRecorder()

			app.sessionManager.LoadAndSave(http.HandlerFunc(app.loginHandler)).ServeHTTP(rr, req)
			resp := rr.Result()

			if resp.StatusCode != tt.expectedStatusCode {
				t.Fatalf("expected status code %d, got %d", tt.expectedStatusCode, resp.StatusCode)
			}

			if tt.expectedLocation != "" {
				if location := resp.Header.Get("Location"); location != tt.expectedLocation {
					t.Fatalf("expected redirect to %q, got %q", tt.expectedLocation, location)
				}
			}

			if tt.flash != nil {
				validateFlashInSession(t, app, req, resp, tt.flash.Message, tt.flash.Level)
			}
		})
	}
}

func Test_logoutHandler(t *testing.T) {
	tcases := []struct {
		name               string
		method             string
		referer            string
		sessionManager     *scs.SessionManager
		expectedStatusCode int
		expectedLocation   string
		flash              *Flash
	}{
		{
			name:               "non-GET method returns 405",
			method:             http.MethodPost,
			sessionManager:     scs.New(),
			expectedStatusCode: http.StatusMethodNotAllowed,
		},
		{
			name:    "Destroy error redirects to referer with error flash",
			method:  http.MethodGet,
			referer: "/albums",
			sessionManager: func() *scs.SessionManager {
				sm := scs.New()
				sm.Store = errDeleteStore{inner: scs.New().Store}
				return sm
			}(),
			expectedStatusCode: http.StatusSeeOther,
			expectedLocation:   "/albums",
			flash: &Flash{
				Message: "Error logging out. Please try again.",
				Level:   flashErr,
			},
		},
		{
			name:               "success redirects to login",
			method:             http.MethodGet,
			sessionManager:     scs.New(),
			expectedStatusCode: http.StatusSeeOther,
			expectedLocation:   "/login",
		},
	}

	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			app := &application{
				sessionManager: tc.sessionManager,
				Logger:         logging.NewLogger(io.Discard, false),
				config:         &config.Config{},
			}

			req := httptest.NewRequest(tc.method, "/logout", nil)
			if tc.referer != "" {
				req.Header.Set("Referer", tc.referer)
			}
			rr := httptest.NewRecorder()

			app.sessionManager.LoadAndSave(http.HandlerFunc(app.logoutHandler)).ServeHTTP(rr, req)
			resp := rr.Result()
			defer resp.Body.Close()

			if resp.StatusCode != tc.expectedStatusCode {
				t.Fatalf("expected status %d, got %d", tc.expectedStatusCode, resp.StatusCode)
			}

			if tc.expectedLocation != "" {
				if got := resp.Header.Get("Location"); got != tc.expectedLocation {
					t.Fatalf("expected redirect to %s, got %s", tc.expectedLocation, got)
				}
			}

			if tc.flash != nil {
				validateFlashInSession(t, app, req, resp, tc.flash.Message, tc.flash.Level)
			}
		})
	}
}

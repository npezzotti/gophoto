package web

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/alexedwards/scs/v2"
	"github.com/npezzotti/gophoto/internal/config"
	"github.com/npezzotti/gophoto/internal/domain"
	"github.com/npezzotti/gophoto/pkg/logging"
)

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

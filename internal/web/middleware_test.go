package web

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/justinas/nosurf"
	"github.com/npezzotti/gophoto/internal/config"
	"github.com/npezzotti/gophoto/internal/domain"
	"github.com/npezzotti/gophoto/pkg/store"
)

func Test_authenticate(t *testing.T) {
	tcases := []struct {
		name            string
		userID          int32
		userServiceStub *userServiceStub
		expectedUser    *domain.UserPresentation
	}{
		{
			name:   "authenticated user",
			userID: 1,
			userServiceStub: &userServiceStub{
				getUserFn: func(ctx context.Context, id int32) (*domain.UserPresentation, error) {
					return &domain.UserPresentation{ID: id}, nil
				},
			},
			expectedUser: &domain.UserPresentation{ID: 1},
		},
		{
			name:         "unauthenticated user",
			userID:       0,
			expectedUser: nil,
		},
		{
			name:   "authenticated user not found",
			userID: 1,
			userServiceStub: &userServiceStub{
				getUserFn: func(ctx context.Context, id int32) (*domain.UserPresentation, error) {
					return nil, nil
				},
			},
			expectedUser: nil,
		},
	}

	for _, tt := range tcases {
		t.Run(tt.name, func(t *testing.T) {
			app := &application{
				sessionManager: scs.New(),
				userService:    tt.userServiceStub,
			}
			req := httptest.NewRequest("GET", "/", nil)
			rr := httptest.NewRecorder()

			app.sessionManager.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.userID != 0 {
					app.sessionManager.Put(r.Context(), SessionKeyUserID, tt.userID)
				}
				app.authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					user, ok := extractUserFromContext(r.Context())
					if !ok && tt.expectedUser != nil {
						t.Errorf("expected user in context, but got none")
					}
					if ok && tt.expectedUser == nil {
						t.Errorf("expected no user in context, but got one")
					}
					if ok && tt.expectedUser != nil && user.ID != tt.expectedUser.ID {
						t.Errorf("expected user %v, got %v", tt.expectedUser, user)
					}
				})).ServeHTTP(w, r)
			})).ServeHTTP(rr, req)
		})
	}
}

func Test_protected(t *testing.T) {
	tcases := []struct {
		name             string
		authenticated    bool
		expectedStatus   int
		expectedLocation string
	}{
		{
			name:           "authenticated user",
			authenticated:  true,
			expectedStatus: http.StatusOK,
		},
		{
			name:             "unauthenticated user",
			authenticated:    false,
			expectedStatus:   http.StatusSeeOther,
			expectedLocation: "/login",
		},
	}

	for _, tt := range tcases {
		t.Run(tt.name, func(t *testing.T) {
			app := &application{
				sessionManager: scs.New(),
			}
			req := httptest.NewRequest("GET", "/", nil)
			rr := httptest.NewRecorder()
			if tt.authenticated {
				req = withAuthenticatedUser(req, &domain.UserPresentation{ID: 1})
			}

			app.sessionManager.LoadAndSave(
				app.protected(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				}))).ServeHTTP(rr, req)

			resp := rr.Result()

			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}

			if !tt.authenticated {
				if loc := resp.Header.Get("Location"); loc != tt.expectedLocation {
					t.Errorf("expected Location header %q, got %q", tt.expectedLocation, loc)
				}
				validateFlashInSession(t, app, req, resp, "You must be logged in to access this.", flashErr)
				return
			}

			if h := resp.Header.Get("Cache-Control"); h != "no-store, no-cache, must-revalidate, max-age=0" {
				t.Errorf("expected Cache-Control header %q, got %q", "no-store, no-cache, must-revalidate, max-age=0", h)
			}

			if h := resp.Header.Get("Pragma"); h != "no-cache" {
				t.Errorf("expected Pragma header %q, got %q", "no-cache", h)
			}

			if h := resp.Header.Get("Expires"); h != "0" {
				t.Errorf("expected Expires header %q, got %q", "0", h)
			}
		})
	}
}

func Test_noSurf(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()

	noSurf(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rr, req)

	cookies := rr.Result().Cookies()
	for _, cookie := range cookies {
		if cookie.Name == nosurf.CookieName {
			if cookie.HttpOnly != true {
				t.Errorf("expected HttpOnly to be true, got false")
			}
			return
		}
	}
	t.Errorf("expected a CSRF cookie to be set, but none was found")
}

func createSignature(path string, expires int64, key []byte) string {
	message := store.CreateMessage(path, expires)
	signature := store.GenerateSignature(message, key)
	return base64.RawURLEncoding.EncodeToString(signature)
}

func buildSignedURL(path string, expires int64, signature string) string {
	q := url.Values{}
	q.Set("expires", strconv.FormatInt(expires, 10))
	q.Set("signature", signature)
	return path + "?" + q.Encode()
}

func Test_validatePresignedURL(t *testing.T) {
	signingKey := []byte("test-signing-key")

	validExpiry := time.Now().Add(10 * time.Minute).Unix()
	validPath := "/uploads/photo.jpg"
	validSignature := createSignature(validPath, validExpiry, signingKey)

	tcases := []struct {
		name           string
		requestURL     string
		expectedStatus int
	}{
		{
			name:           "allows request with valid signature and non-expired timestamp",
			requestURL:     buildSignedURL(validPath, validExpiry, validSignature),
			expectedStatus: http.StatusOK,
		},
		{
			name:           "blocks request when signature is invalid",
			requestURL:     buildSignedURL(validPath, validExpiry, "invalid-signature"),
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "blocks request when URL is expired",
			requestURL:     buildSignedURL(validPath, time.Now().Add(-1*time.Minute).Unix(), validSignature),
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "blocks request when required query params are missing",
			requestURL:     validPath,
			expectedStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tcases {
		t.Run(tt.name, func(t *testing.T) {
			app := &application{
				config: &config.Config{
					SigningKey: signingKey,
				},
			}
			req := httptest.NewRequest("GET", tt.requestURL, nil)
			rr := httptest.NewRecorder()

			app.validatePresignedURL(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})).ServeHTTP(rr, req)

			resp := rr.Result()

			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}
		})
	}
}

func Test_validPresignedURL(t *testing.T) {
	signingKey := []byte("test-signing-key")

	validExpiry := time.Now().Add(10 * time.Minute).Unix()
	validPath := "/uploads/photo.jpg"
	validSignature := createSignature(validPath, validExpiry, signingKey)

	tcases := []struct {
		name  string
		url   string
		valid bool
	}{
		{
			name:  "valid signature and non-expired timestamp",
			url:   buildSignedURL(validPath, validExpiry, validSignature),
			valid: true,
		},
		{
			name:  "invalid signature",
			url:   buildSignedURL(validPath, validExpiry, "invalid-signature"),
			valid: false,
		},
		{
			name:  "expired URL",
			url:   buildSignedURL(validPath, time.Now().Add(-1*time.Minute).Unix(), validSignature),
			valid: false,
		},
		{
			name:  "missing query params",
			url:   validPath,
			valid: false,
		},
	}

	for _, tt := range tcases {
		t.Run(tt.name, func(t *testing.T) {
			rawURL, err := url.Parse(tt.url)
			if err != nil {
				t.Fatalf("failed to parse URL: %v", err)
			}
			valid := validPresignedURL(rawURL, signingKey)
			if tt.valid && !valid {
				t.Errorf("expected URL to be valid, but it was not")
			}
			if !tt.valid && valid {
				t.Errorf("expected URL to be invalid, but it was valid")
			}
		})
	}
}

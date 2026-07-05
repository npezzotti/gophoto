package web

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/alexedwards/scs/v2"
	"github.com/npezzotti/gophoto/internal/config"
	"github.com/npezzotti/gophoto/internal/domain"
	"github.com/npezzotti/gophoto/internal/utils"
	"github.com/npezzotti/gophoto/pkg/logging"
)

func init() {
	gob.Register(Flash{})
}

// withAuthenticatedUser adds the authenticated user to the request context.
func withAuthenticatedUser(req *http.Request, user *domain.UserPresentation) *http.Request {
	ctx := context.WithValue(req.Context(), IsAuthenticatedContextKey, true)
	ctx = context.WithValue(ctx, AuthenticatedUserContextKey, user)
	req = req.WithContext(ctx)
	return req
}

// validateFlashInSession checks if the flash message in the session matches the expected message and level.
func validateFlashInSession(t *testing.T, app *application, req *http.Request, resp *http.Response, expectedMessage string, expectedLevel flashClass) {
	t.Helper()
	for _, cookie := range resp.Cookies() {
		if cookie.Name == app.sessionManager.Cookie.Name {
			// Load the session from the cookie
			loadedCtx, err := app.sessionManager.Load(req.Context(), cookie.Value)
			if err != nil {
				t.Fatalf("failed to load session: %v", err)
			}
			val := app.sessionManager.Get(loadedCtx, sessionKeyFlash)
			flashMsg, ok := val.(Flash)
			if !ok {
				t.Fatalf("expected flash in session, got %T", val)
			}
			if flashMsg.Message != expectedMessage || flashMsg.Level != expectedLevel {
				t.Fatalf("unexpected flash message: got %v", flashMsg)
			}
		}
	}
}
func Test_application_getAlbumHandler(t *testing.T) {
	tcases := []struct {
		name          string
		albumService  *albumServiceStub
		req           *http.Request
		authenticated bool
		wantStatus    int
		wantLocation  string
		wantFlash     *Flash
		validateBody  []string
	}{
		{
			name: "successful request",
			albumService: &albumServiceStub{
				getAlbumPageViewFn: func(ctx context.Context, userID, albumID, limit, offset int32) (domain.AlbumPageView, error) {
					return domain.AlbumPageView{
						Album: domain.Album{
							ID:     albumID,
							UserID: userID,
							Title:  "Test Album",
						},
						Photos: []domain.ResponsiveImage{
							{ID: 1, Alt: "Photo 1", OriginalSrc: "", Sources: []domain.ImageSource{}},
							{ID: 2, Alt: "Photo 2", OriginalSrc: "", Sources: []domain.ImageSource{}},
						},
					}, nil
				},
			},
			req:           httptest.NewRequest(http.MethodGet, "/albums?id=1", nil),
			authenticated: true,
			wantStatus:    http.StatusOK,
			validateBody:  []string{"Test Album", "Photo 1", "Photo 2"},
		},
		{
			name:          "redirects to login when user is missing from context",
			req:           httptest.NewRequest(http.MethodGet, "/albums?id=1", nil),
			authenticated: false,
			wantStatus:    http.StatusSeeOther,
			wantLocation:  "/login",
			wantFlash:     &Flash{Message: "User not found.", Level: flashErr},
		},
		{
			name:          "redirects to /albums for invalid album id",
			req:           httptest.NewRequest(http.MethodGet, "/albums?id=bad-id", nil),
			authenticated: true,
			wantStatus:    http.StatusSeeOther,
			wantLocation:  "/albums",
			wantFlash:     &Flash{Message: "Invalid album ID.", Level: flashErr},
		},
		{
			name: "returns user's albums when no id parameter",
			albumService: &albumServiceStub{
				listAlbumsByUserFn: func(ctx context.Context, userID int32, limit, offset int32) ([]*domain.AlbumListItem, error) {
					return []*domain.AlbumListItem{
						{
							Album: domain.Album{
								ID:    1,
								Title: "Album 1",
							},
							CoverPhotoKey: "test-key-1",
							AlbumCoverImage: domain.ResponsiveImage{
								ID:          1,
								Alt:         "Photo 1",
								OriginalSrc: "",
								Sources:     []domain.ImageSource{},
							},
						},
						{
							Album: domain.Album{
								ID:    2,
								Title: "Album 2",
							},
							CoverPhotoKey: "test-key-2",
							AlbumCoverImage: domain.ResponsiveImage{
								ID:          2,
								Alt:         "Photo 2",
								OriginalSrc: "",
								Sources:     []domain.ImageSource{},
							},
						},
					}, nil
				},
			},
			req:           httptest.NewRequest(http.MethodGet, "/albums", nil),
			authenticated: true,
			wantStatus:    http.StatusOK,
			validateBody:  []string{"Album 1", "Album 2"},
		},
		{
			name: "album not found",
			albumService: &albumServiceStub{
				getAlbumPageViewFn: func(ctx context.Context, userID, albumID, limit, offset int32) (domain.AlbumPageView, error) {
					return domain.AlbumPageView{}, domain.ErrAlbumNotFound
				},
			},
			req:           httptest.NewRequest(http.MethodGet, "/albums?id=1", nil),
			authenticated: true,
			wantStatus:    http.StatusSeeOther,
			wantLocation:  "/albums",
			wantFlash:     &Flash{Message: "Album not found.", Level: flashErr},
		},
		{
			name: "error getting album",
			albumService: &albumServiceStub{
				getAlbumPageViewFn: func(ctx context.Context, userID, albumID, limit, offset int32) (domain.AlbumPageView, error) {
					return domain.AlbumPageView{}, errors.New("internal server error")
				},
			},
			req:           httptest.NewRequest(http.MethodGet, "/albums?id=1", nil),
			authenticated: true,
			wantStatus:    http.StatusSeeOther,
			wantLocation:  "/albums",
			wantFlash:     &Flash{Message: "Internal server error.", Level: flashErr},
		},
	}

	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			app := &application{
				sessionManager: scs.New(),
				Logger:         logging.NewLogger(io.Discard, false),
				albumService:   tc.albumService,
				config:         &config.Config{},
			}

			req := tc.req
			if tc.authenticated {
				req = req.WithContext(context.WithValue(req.Context(), AuthenticatedUserContextKey, &domain.UserPresentation{ID: 1}))
			}
			rr := httptest.NewRecorder()

			app.sessionManager.LoadAndSave(http.HandlerFunc(app.getAlbumHandler)).ServeHTTP(rr, req)

			resp := rr.Result()
			defer resp.Body.Close()

			if resp.StatusCode != tc.wantStatus {
				t.Fatalf("expected status %d, got %d", tc.wantStatus, resp.StatusCode)
			}

			if tc.wantLocation != "" {
				if got := resp.Header.Get("Location"); got != tc.wantLocation {
					t.Fatalf("expected redirect to %q, got %q", tc.wantLocation, got)
				}
			}

			if tc.wantFlash != nil {
				validateFlashInSession(t, app, req, resp, tc.wantFlash.Message, tc.wantFlash.Level)
			}
			if len(tc.validateBody) > 0 {
				bodyBytes, err := io.ReadAll(resp.Body)
				if err != nil {
					t.Fatalf("failed to read response body: %v", err)
				}
				body := string(bodyBytes)
				for _, substr := range tc.validateBody {
					if !strings.Contains(body, substr) {
						t.Fatalf("expected response body to contain %q, got %q", substr, body)
					}
				}
			}
		})
	}
}

func Test_createAlbumHandler(t *testing.T) {
	tcases := []struct {
		name          string
		albumService  *albumServiceStub
		albumTitle    string
		authenticated bool
		wantStatus    int
		wantLocation  string
		wantFlash     *Flash
	}{
		{
			name: "successfully creates album",
			albumService: &albumServiceStub{
				createAlbumFn: func(ctx context.Context, userID int32, title string) (domain.Album, error) {
					return domain.Album{ID: 1, Title: title}, nil
				},
			},
			albumTitle:    "New Album",
			authenticated: true,
			wantStatus:    http.StatusSeeOther,
			wantLocation:  "/albums?id=1",
			wantFlash:     &Flash{Message: fmt.Sprintf("Successfully created album %q!", "New Album"), Level: flashInfo},
		},
		{
			name:          "redirects to login when user is missing from context",
			albumTitle:    "New Album",
			authenticated: false,
			wantStatus:    http.StatusSeeOther,
			wantLocation:  "/login",
			wantFlash:     &Flash{Message: "User not found.", Level: flashErr},
		},
		{
			name:          "title missing in form",
			authenticated: true,
			wantStatus:    http.StatusSeeOther,
			wantLocation:  "/albums",
			wantFlash:     &Flash{Message: "Album title cannot be empty.", Level: flashErr},
		},
		{
			name: "error creating album",
			albumService: &albumServiceStub{
				createAlbumFn: func(ctx context.Context, userID int32, title string) (domain.Album, error) {
					return domain.Album{}, errors.New("failed to create album")
				},
			},
			albumTitle:    "New Album",
			authenticated: true,
			wantStatus:    http.StatusSeeOther,
			wantLocation:  "/albums",
			wantFlash:     &Flash{Message: "Error creating album.", Level: flashErr},
		},
	}

	for _, tt := range tcases {
		t.Run(tt.name, func(t *testing.T) {
			app := &application{
				sessionManager: scs.New(),
				Logger:         logging.NewLogger(io.Discard, false),
				albumService:   tt.albumService,
				config:         &config.Config{},
			}

			form := url.Values{}
			form.Set("title", tt.albumTitle)
			req := httptest.NewRequest(http.MethodPost, "/albums", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			if tt.authenticated {
				req = withAuthenticatedUser(req, &domain.UserPresentation{ID: 1})
			}

			rr := httptest.NewRecorder()
			app.sessionManager.LoadAndSave(http.HandlerFunc(app.createAlbumHandler)).ServeHTTP(rr, req)

			resp := rr.Result()
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				t.Fatalf("expected status %d, got %d", tt.wantStatus, resp.StatusCode)
			}

			if tt.wantLocation != "" {
				if got := resp.Header.Get("Location"); got != tt.wantLocation {
					t.Fatalf("expected redirect to %q, got %q", tt.wantLocation, got)
				}
			}

			if tt.wantFlash != nil {
				validateFlashInSession(t, app, req, resp, tt.wantFlash.Message, tt.wantFlash.Level)
			}
		})
	}
}

func Test_updateAlbumHandler(t *testing.T) {
	tcases := []struct {
		name          string
		albumService  *albumServiceStub
		updatedTitle  string
		authenticated bool
		wantStatus    int
		wantLocation  string
		wantFlash     *Flash
	}{
		{
			name: "successfully updates album",
			albumService: &albumServiceStub{
				updateAlbumFn: func(ctx context.Context, userID, albumID int32, title string) (domain.Album, error) {
					return domain.Album{ID: albumID, Title: title}, nil
				},
			},
			updatedTitle:  "Updated Album Title",
			authenticated: true,
			wantStatus:    http.StatusSeeOther,
			wantLocation:  "/albums?id=1",
			wantFlash:     &Flash{Message: "Album successfully updated.", Level: flashInfo},
		},
		{
			name:          "redirects to login when user is missing from context",
			updatedTitle:  "Updated Album Title",
			authenticated: false,
			wantStatus:    http.StatusSeeOther,
			wantLocation:  "/login",
			wantFlash:     &Flash{Message: "User not found.", Level: flashErr},
		},
		{
			name: "album not found",
			albumService: &albumServiceStub{
				updateAlbumFn: func(ctx context.Context, userID, albumID int32, title string) (domain.Album, error) {
					return domain.Album{}, domain.ErrAlbumNotFound
				},
			},
			updatedTitle:  "Updated Album Title",
			authenticated: true,
			wantStatus:    http.StatusSeeOther,
			wantLocation:  "/albums",
			wantFlash:     &Flash{Message: "Album not found.", Level: flashErr},
		},
		{
			name: "error updating album",
			albumService: &albumServiceStub{
				updateAlbumFn: func(ctx context.Context, userID, albumID int32, title string) (domain.Album, error) {
					return domain.Album{}, errors.New("internal server error")
				},
			},
			updatedTitle:  "Updated Album Title",
			authenticated: true,
			wantStatus:    http.StatusSeeOther,
			wantLocation:  "/albums",
			wantFlash:     &Flash{Message: "Error updating album.", Level: flashErr},
		},
	}

	for _, tt := range tcases {
		t.Run(tt.name, func(t *testing.T) {
			app := &application{
				sessionManager: scs.New(),
				Logger:         logging.NewLogger(io.Discard, false),
				albumService:   tt.albumService,
			}

			form := url.Values{}
			form.Set("title", tt.updatedTitle)
			req := httptest.NewRequest(http.MethodPost, "/albums?id=1", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			if tt.authenticated {
				req = withAuthenticatedUser(req, &domain.UserPresentation{ID: 1})
			}

			rr := httptest.NewRecorder()
			app.sessionManager.LoadAndSave(http.HandlerFunc(app.updateAlbumHandler)).ServeHTTP(rr, req)

			resp := rr.Result()
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				t.Fatalf("expected status %d, got %d", tt.wantStatus, resp.StatusCode)
			}
			if got := resp.Header.Get("Location"); got != tt.wantLocation {
				t.Fatalf("expected redirect to %q, got %q", tt.wantLocation, got)
			}

			validateFlashInSession(t, app, req, resp, tt.wantFlash.Message, tt.wantFlash.Level)
		})
	}
}

func Test_deleteAlbumHandler(t *testing.T) {
	tcases := []struct {
		name          string
		albumService  *albumServiceStub
		req           *http.Request
		authenticated bool
		wantStatus    int
		wantLocation  string
		wantFlash     *Flash
	}{
		{
			name: "successfully deletes album",
			albumService: &albumServiceStub{
				deleteAlbumFn: func(ctx context.Context, userID, albumID int32) error {
					return nil
				},
			},
			req:           httptest.NewRequest(http.MethodPost, "/albums/delete?id=1", nil),
			authenticated: true,
			wantStatus:    http.StatusSeeOther,
			wantLocation:  "/albums",
			wantFlash:     &Flash{Message: "Successfully deleted album with ID 1.", Level: flashInfo},
		},
		{
			name:          "redirects to login when user is missing from context",
			req:           httptest.NewRequest(http.MethodPost, "/albums/delete?id=1", nil),
			authenticated: false,
			wantStatus:    http.StatusSeeOther,
			wantLocation:  "/login",
			wantFlash:     &Flash{Message: "User not found.", Level: flashErr},
		},
		{
			name:          "id parameter missing",
			req:           httptest.NewRequest(http.MethodPost, "/albums/delete", nil),
			authenticated: true,
			wantStatus:    http.StatusSeeOther,
			wantLocation:  "/albums",
			wantFlash:     &Flash{Message: "Album ID is required.", Level: flashErr},
		},
		{
			name:          "invalid album id",
			req:           httptest.NewRequest(http.MethodPost, "/albums/delete?id=bad-id", nil),
			authenticated: true,
			wantStatus:    http.StatusSeeOther,
			wantLocation:  "/albums",
			wantFlash:     &Flash{Message: "Invalid album ID.", Level: flashErr},
		},
		{
			name: "album not found",
			albumService: &albumServiceStub{
				deleteAlbumFn: func(ctx context.Context, userID, albumID int32) error {
					return domain.ErrAlbumNotFound
				},
			},
			req:           httptest.NewRequest(http.MethodPost, "/albums/delete?id=1", nil),
			authenticated: true,
			wantStatus:    http.StatusSeeOther,
			wantLocation:  "/albums",
			wantFlash:     &Flash{Message: "Album not found.", Level: flashErr},
		},
		{
			name: "error deleting album",
			albumService: &albumServiceStub{
				deleteAlbumFn: func(ctx context.Context, userID, albumID int32) error {
					return errors.New("internal server error")
				},
			},
			req:           httptest.NewRequest(http.MethodPost, "/albums/delete?id=1", nil),
			authenticated: true,
			wantStatus:    http.StatusSeeOther,
			wantLocation:  "/albums",
			wantFlash:     &Flash{Message: "Error deleting album.", Level: flashErr},
		},
	}

	for _, tt := range tcases {
		t.Run(tt.name, func(t *testing.T) {
			app := &application{
				sessionManager: scs.New(),
				Logger:         logging.NewLogger(io.Discard, false),
				albumService:   tt.albumService,
			}

			if tt.authenticated {
				tt.req = withAuthenticatedUser(tt.req, &domain.UserPresentation{ID: 1})
			}
			rr := httptest.NewRecorder()

			app.sessionManager.LoadAndSave(http.HandlerFunc(app.deleteAlbumHandler)).ServeHTTP(rr, tt.req)
			resp := rr.Result()
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				t.Fatalf("expected status %d, got %d", tt.wantStatus, resp.StatusCode)
			}
			if got := resp.Header.Get("Location"); got != tt.wantLocation {
				t.Fatalf("expected redirect to %q, got %q", tt.wantLocation, got)
			}
			if tt.wantFlash != nil {
				validateFlashInSession(t, app, tt.req, resp, tt.wantFlash.Message, tt.wantFlash.Level)
			}
		})
	}
}

func Test_aboutHandler(t *testing.T) {
	app := &application{
		sessionManager: scs.New(),
		Logger:         logging.NewLogger(os.Stdout, false),
		config:         &config.Config{},
	}

	req := httptest.NewRequest(http.MethodGet, "/about", nil)
	rr := httptest.NewRecorder()
	req = withAuthenticatedUser(req, &domain.UserPresentation{ID: 1})

	app.sessionManager.LoadAndSave(http.HandlerFunc(app.aboutHandler)).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func Test_uploadPhotoHandler(t *testing.T) {
	tcases := []struct {
		name          string
		photoService  *photoServiceStub
		authenticated bool
		url           string
		wantStatus    int
		wantErr       string
		wantLocation  string
	}{
		{
			name: "successfully uploads photo",
			photoService: &photoServiceStub{
				createAlbumPhotoWithOriginalMetadataFn: func(ctx context.Context, f multipart.File, fh *multipart.FileHeader, userID, albumID int32) (domain.Photo, error) {
					return domain.Photo{ID: 1, UserID: &userID}, nil
				},
			},
			authenticated: true,
			url:           "/photos?type=album&id=1",
			wantStatus:    http.StatusCreated,
		},
		{
			name:          "throws error when request is unauthenticated",
			authenticated: false,
			url:           "/photos?type=album&id=1",
			wantStatus:    http.StatusUnauthorized,
			wantErr:       "user not authenticated",
		},
	}

	for _, tt := range tcases {
		t.Run(tt.name, func(t *testing.T) {
			app := &application{
				sessionManager: scs.New(),
				Logger:         logging.NewLogger(io.Discard, false),
				config:         &config.Config{},
				photoService:   tt.photoService,
			}

			form := &bytes.Buffer{}
			writer := multipart.NewWriter(form)
			part, err := writer.CreateFormFile(FormFileName, "test.jpg")
			if err != nil {
				t.Fatalf("error creating form file: %v", err)
			}
			_, err = part.Write([]byte("dummy photo data"))
			if err != nil {
				t.Fatalf("error writing to form file: %v", err)
			}
			writer.Close()

			req := httptest.NewRequest(http.MethodPost, tt.url, form)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			rr := httptest.NewRecorder()

			if tt.authenticated {
				req = withAuthenticatedUser(req, &domain.UserPresentation{ID: 1})
			}

			app.sessionManager.LoadAndSave(http.HandlerFunc(app.uploadPhotoHandler)).ServeHTTP(rr, req)

			resp := rr.Result()
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				t.Fatalf("expected status %d, got %d", tt.wantStatus, resp.StatusCode)
			}

			if tt.wantErr != "" {
				var jsonResp map[string]string
				err = json.NewDecoder(resp.Body).Decode(&jsonResp)
				if err != nil {
					t.Fatalf("error decoding JSON response: %v", err)
				}
				if jsonResp["error"] != tt.wantErr {
					t.Fatalf("expected error message %q, got %q", tt.wantErr, jsonResp["error"])
				}
				return
			}

			jsonResp := make(map[string]int32)
			err = json.NewDecoder(resp.Body).Decode(&jsonResp)
			if err != nil {
				t.Fatalf("error decoding JSON response: %v", err)
			}
			if _, ok := jsonResp["id"]; !ok {
				t.Fatalf("expected JSON response to contain 'id' field")
			}
		})
	}
}

func Test_photoStatusHandler(t *testing.T) {
	tcases := []struct {
		name, url, wantStatus string
		userID                int32
		photoServiceStub      *photoServiceStub
		wantErr               string
		wantStatusCode        int
	}{
		{
			name:       "photo is processing",
			url:        "/photos/status?id=1",
			wantStatus: string(domain.PhotoStatusProcessing),
			userID:     1,
			photoServiceStub: &photoServiceStub{
				getPhotoFn: func(ctx context.Context, id int32) (domain.Photo, error) {
					return domain.Photo{ID: id, Status: domain.PhotoStatusProcessing, UserID: utils.PtrInt32(1)}, nil
				},
			},
			wantStatusCode: http.StatusOK,
		},
		{
			name:       "photo is ready",
			url:        "/photos/status?id=2",
			wantStatus: string(domain.PhotoStatusProcessed),
			userID:     1,
			photoServiceStub: &photoServiceStub{
				getPhotoFn: func(ctx context.Context, id int32) (domain.Photo, error) {
					return domain.Photo{ID: id, Status: domain.PhotoStatusProcessed, UserID: utils.PtrInt32(1)}, nil
				},
			},
			wantStatusCode: http.StatusOK,
		},
		{
			name:       "photo not found",
			url:        "/photos/status?id=3",
			wantStatus: "error",
			userID:     1,
			photoServiceStub: &photoServiceStub{
				getPhotoFn: func(ctx context.Context, id int32) (domain.Photo, error) {
					return domain.Photo{}, domain.ErrPhotoNotFound
				},
			},
			wantErr:        "photo not found",
			wantStatusCode: http.StatusNotFound,
		},
		{
			name:       "unauthenticated user",
			url:        "/photos/status?id=4",
			wantStatus: "error",
			userID:     0,
			photoServiceStub: &photoServiceStub{
				getPhotoFn: func(ctx context.Context, id int32) (domain.Photo, error) {
					return domain.Photo{ID: id, Status: domain.PhotoStatusProcessed, UserID: utils.PtrInt32(1)}, nil
				},
			},
			wantErr:        "user not authenticated",
			wantStatusCode: http.StatusUnauthorized,
		},
		{
			name:   "missing photo ID parameter",
			url:    "/photos/status",
			userID: 1,
			photoServiceStub: &photoServiceStub{
				getPhotoFn: func(ctx context.Context, id int32) (domain.Photo, error) {
					return domain.Photo{}, nil
				},
			},
			wantErr:        "missing \"id\" query parameter",
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name:   "invalid photo ID parameter",
			url:    "/photos/status?id=invalid",
			userID: 1,
			photoServiceStub: &photoServiceStub{
				getPhotoFn: func(ctx context.Context, id int32) (domain.Photo, error) {
					return domain.Photo{}, nil
				},
			},
			wantErr:        "invalid \"id\" query parameter",
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name:   "photo not found",
			url:    "/photos/status?id=3",
			userID: 1,
			photoServiceStub: &photoServiceStub{
				getPhotoFn: func(ctx context.Context, id int32) (domain.Photo, error) {
					return domain.Photo{}, domain.ErrPhotoNotFound
				},
			},
			wantErr:        "photo not found",
			wantStatusCode: http.StatusNotFound,
		},
		{
			name:   "photo belongs to another user",
			url:    "/photos/status?id=5",
			userID: 1,
			photoServiceStub: &photoServiceStub{
				getPhotoFn: func(ctx context.Context, id int32) (domain.Photo, error) {
					return domain.Photo{ID: id, Status: domain.PhotoStatusProcessed, UserID: utils.PtrInt32(2)}, nil
				},
			},
			wantErr:        "photo not found",
			wantStatusCode: http.StatusNotFound,
		},
	}

	for _, tt := range tcases {
		t.Run(tt.name, func(t *testing.T) {
			app := &application{
				sessionManager: scs.New(),
				Logger:         logging.NewLogger(io.Discard, false),
				config:         &config.Config{},
				photoService:   tt.photoServiceStub,
			}

			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			if tt.userID != 0 {
				req = withAuthenticatedUser(req, &domain.UserPresentation{ID: tt.userID})
			}
			rr := httptest.NewRecorder()

			app.sessionManager.LoadAndSave(http.HandlerFunc(app.photoStatusHandler)).ServeHTTP(rr, req)

			resp := rr.Result()
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantStatusCode {
				t.Fatalf("expected status %d, got %d", tt.wantStatusCode, resp.StatusCode)
			}

			if tt.wantErr != "" {
				var jsonResp map[string]string
				err := json.NewDecoder(resp.Body).Decode(&jsonResp)
				if err != nil {
					t.Fatalf("error decoding JSON response: %v", err)
				}
				if jsonResp["error"] != tt.wantErr {
					t.Fatalf("expected error message %q, got %q", tt.wantErr, jsonResp["error"])
				}
				return
			}

			respBody := make(map[string]string)
			err := json.NewDecoder(resp.Body).Decode(&respBody)
			if err != nil {
				t.Fatalf("error decoding JSON response: %v", err)
			}

			if status, ok := respBody["status"]; !ok || status != tt.wantStatus || !strings.EqualFold(status, tt.wantStatus) {
				t.Fatalf("expected JSON response to contain 'status' field with value %q, got %q", tt.wantStatus, status)
			}
		})
	}
}

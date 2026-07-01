package web

import (
	"context"
	"encoding/gob"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alexedwards/scs/v2"
	"github.com/npezzotti/gophoto/internal/config"
	"github.com/npezzotti/gophoto/internal/domain"
	"github.com/npezzotti/gophoto/pkg/logging"
)

func init() {
	gob.Register(Flash{})
}

func Test_application_getAlbumHandler(t *testing.T) {
	t.Run("successful request", func(t *testing.T) {
		app := &application{
			sessionManager: scs.New(),
			Logger:         logging.NewLogger(io.Discard, false),
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
			config: &config.Config{},
		}

		req := httptest.NewRequest(http.MethodGet, "/albums?id=1", nil)
		req = req.WithContext(context.WithValue(req.Context(), AuthenticatedUserContextKey, &domain.UserPresentation{ID: 1}))
		rr := httptest.NewRecorder()

		app.sessionManager.LoadAndSave(http.HandlerFunc(app.getAlbumHandler)).ServeHTTP(rr, req)

		resp := rr.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
		}

		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read response body: %v", err)
		}
		if !strings.Contains(string(bodyBytes), "Test Album") || !strings.Contains(string(bodyBytes), "Photo 1") || !strings.Contains(string(bodyBytes), "Photo 2") {
			t.Fatalf("expected response body to contain album and photo titles, got %q", string(bodyBytes))
		}
	})
	t.Run("redirects to login when user is missing from context", func(t *testing.T) {
		app := &application{
			sessionManager: scs.New(),
			Logger:         logging.NewLogger(io.Discard, false),
		}

		req := httptest.NewRequest(http.MethodGet, "/albums", nil)
		rr := httptest.NewRecorder()

		app.sessionManager.LoadAndSave(http.HandlerFunc(app.getAlbumHandler)).ServeHTTP(rr, req)

		resp := rr.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusSeeOther {
			t.Fatalf("expected status %d, got %d", http.StatusSeeOther, resp.StatusCode)
		}

		if got := resp.Header.Get("Location"); got != "/login" {
			t.Fatalf("expected redirect to /login, got %q", got)
		}
	})

	t.Run("redirects to /albums for invalid album id", func(t *testing.T) {
		app := &application{
			sessionManager: scs.New(),
			Logger:         logging.NewLogger(io.Discard, false),
		}

		req := httptest.NewRequest(http.MethodGet, "/albums?id=bad-id", nil)
		req = req.WithContext(context.WithValue(req.Context(), AuthenticatedUserContextKey, &domain.UserPresentation{ID: 1}))
		rr := httptest.NewRecorder()

		app.sessionManager.LoadAndSave(http.HandlerFunc(app.getAlbumHandler)).ServeHTTP(rr, req)

		resp := rr.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusSeeOther {
			t.Fatalf("expected status %d, got %d", http.StatusSeeOther, resp.StatusCode)
		}

		if got := resp.Header.Get("Location"); got != "/albums" {
			t.Fatalf("expected redirect to /albums, got %q", got)
		}
	})

	t.Run("returns user's albums when no id parameter", func(t *testing.T) {
		app := &application{
			sessionManager: scs.New(),
			Logger:         logging.NewLogger(io.Discard, false),
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
			config: &config.Config{},
		}

		req := httptest.NewRequest(http.MethodGet, "/albums", nil)
		req = req.WithContext(context.WithValue(req.Context(), AuthenticatedUserContextKey, &domain.UserPresentation{ID: 1}))
		rr := httptest.NewRecorder()

		app.sessionManager.LoadAndSave(http.HandlerFunc(app.getAlbumHandler)).ServeHTTP(rr, req)

		resp := rr.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
		}

		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read response body: %v", err)
		}
		if !strings.Contains(string(bodyBytes), "Album 1") || !strings.Contains(string(bodyBytes), "Album 2") {
			t.Fatalf("expected response body to contain album titles, got %q", string(bodyBytes))
		}
	})

	t.Run("returns method not allowed for unsupported methods", func(t *testing.T) {
		app := &application{
			sessionManager: scs.New(),
			Logger:         logging.NewLogger(io.Discard, false),
		}

		req := httptest.NewRequest(http.MethodPost, "/albums", nil)
		rr := httptest.NewRecorder()

		app.getAlbumHandler(rr, req)

		resp := rr.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, resp.StatusCode)
		}
	})
}

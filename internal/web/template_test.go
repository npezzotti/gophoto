package web

import (
	"bytes"
	"context"
	htmlTemplate "html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alexedwards/scs/v2"
	"github.com/justinas/nosurf"
	"github.com/npezzotti/gophoto/internal/config"
	"github.com/npezzotti/gophoto/internal/domain"
	"github.com/npezzotti/gophoto/pkg/logging"
	"github.com/npezzotti/gophoto/pkg/template"
)

func Test_application_generateTemplateData(t *testing.T) {
	t.Run("should set default template data", func(t *testing.T) {
		a := application{
			sessionManager: scs.New(),
		}

		req := httptest.NewRequest(http.MethodPost, "/test", nil)

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			td := a.generateTemplateData(r)

			if td == nil {
				t.Error("expected template data to be set, got nil")
			}
		})

		rr := httptest.NewRecorder()
		a.sessionManager.LoadAndSave(handler).ServeHTTP(rr, req)
	})

	t.Run("should generate CSRF token", func(t *testing.T) {
		a := application{
			sessionManager: scs.New(),
		}

		req := httptest.NewRequest(http.MethodPost, "/test", nil)

		handler := nosurf.New(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			td := a.generateTemplateData(r)
			t.Logf("Generated CSRF token: %s\n", td.CSRFToken)

			if td.CSRFToken == "" {
				t.Error("expected CSRF token to be generated, got empty string")
			}
		}))

		rr := httptest.NewRecorder()
		a.sessionManager.LoadAndSave(handler).ServeHTTP(rr, req)
	})

	t.Run("should include flash message from session", func(t *testing.T) {
		a := application{
			sessionManager: scs.New(),
		}

		req := httptest.NewRequest(http.MethodPost, "/test", nil)

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			a.sessionManager.Put(r.Context(), sessionKeyFlash, Flash{
				Message: "Test flash message",
				Level:   flashInfo,
			})

			td := a.generateTemplateData(r)

			if td.Flash == nil {
				t.Error("expected flash message to be included in template data, got nil")
				return
			}

			expected := &Flash{
				Message: "Test flash message",
				Level:   flashInfo,
			}
			if *td.Flash != *expected {
				t.Errorf("got %v, expected %v", td.Flash, expected)
			}
		})

		rr := httptest.NewRecorder()
		a.sessionManager.LoadAndSave(handler).ServeHTTP(rr, req)
	})

	t.Run("should set default template data with authenticated user", func(t *testing.T) {
		a := application{
			sessionManager: scs.New(),
		}

		req := httptest.NewRequest(http.MethodPost, "/test", nil)

		// Add user to request context
		ctx := context.Background()
		ctx = context.WithValue(ctx, AuthenticatedUserContextKey, &domain.UserPresentation{ID: 123})
		req = req.WithContext(ctx)

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			td := a.generateTemplateData(r)

			if td == nil {
				t.Error("expected template data to be set, got nil")
			}

			if td.User == nil {
				t.Error("expected authenticated user to be included in template data, got nil")
				return
			}
		})

		rr := httptest.NewRecorder()
		a.sessionManager.LoadAndSave(handler).ServeHTTP(rr, req)
	})
}

func Test_application_writeTemplateResponse_successWithCachedTemplate(t *testing.T) {
	t.Run("successfully renders template with cached template", func(t *testing.T) {
		parsedTemplate, err := htmlTemplate.New("ok.html").Parse(`{{define "base"}}Hello, {{.User.ID}} {{.CSRFToken}}{{end}}`)
		if err != nil {
			t.Fatalf("failed to parse test template: %v", err)
		}

		var logOutput bytes.Buffer
		a := application{
			config: &config.Config{UseTemplateCache: true},
			templateCache: template.TemplateCache{
				"ok.html": parsedTemplate,
			},
			Logger: logging.NewLogger(&logOutput, false),
		}

		rr := httptest.NewRecorder()
		td := &templateData{User: &domain.UserPresentation{ID: 123}, CSRFToken: "abcd1234"}

		a.writeTemplateResponse(rr, td, http.StatusTeapot, "ok.html")

		if rr.Code != http.StatusTeapot {
			t.Fatalf("expected status %d, got %d", http.StatusTeapot, rr.Code)
		}

		if got := rr.Header().Get("Content-Type"); got != "text/html; charset=UTF-8" {
			t.Fatalf("expected Content-Type %q, got %q", "text/html; charset=UTF-8", got)
		}

		if body := rr.Body.String(); !strings.Contains(body, "Hello, 123 abcd1234") {
			t.Fatalf("expected response body to contain rendered content, got %q", body)
		}

		if strings.Contains(logOutput.String(), "error rendering template") ||
			strings.Contains(logOutput.String(), "error writing template") {
			t.Fatalf("expected no rendering errors in logs, got %q", logOutput.String())
		}
	})

	t.Run("returns 500 when template is missing", func(t *testing.T) {
		var logOutput bytes.Buffer
		a := application{
			config:        &config.Config{UseTemplateCache: true},
			templateCache: template.TemplateCache{},
			Logger:        logging.NewLogger(&logOutput, false),
		}
		rr := httptest.NewRecorder()

		a.writeTemplateResponse(rr, &templateData{}, http.StatusOK, "missing.html")
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
		if !strings.Contains(logOutput.String(), "error rendering template") {
			t.Fatalf("expected rendering error in logs, got %q", logOutput.String())
		}
	})

	t.Run("returns 500 when template rendering fails", func(t *testing.T) {
		parsedTemplate, err := htmlTemplate.New("error.html").Parse(`{{define "base"}}{{.UndefinedField}}{{end}}`)
		if err != nil {
			t.Fatalf("failed to parse test template: %v", err)
		}
		logOutput := bytes.Buffer{}
		a := application{
			config:        &config.Config{UseTemplateCache: true},
			templateCache: template.TemplateCache{"error.html": parsedTemplate},
			Logger:        logging.NewLogger(&logOutput, false),
		}
		rr := httptest.NewRecorder()

		a.writeTemplateResponse(rr, &templateData{}, http.StatusOK, "error.html")
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
		if !strings.Contains(logOutput.String(), "error rendering template") {
			t.Fatalf("expected rendering error in logs, got %q", logOutput.String())
		}
	})
}

func Test_application_renderTemplate(t *testing.T) {
	t.Run("renders template with status 200", func(t *testing.T) {
		parsedTemplate, err := htmlTemplate.New("ok.html").Parse(`{{define "base"}}Hello, {{.User.ID}}{{end}}`)
		if err != nil {
			t.Fatalf("failed to parse test template: %v", err)
		}

		var logOutput bytes.Buffer
		a := application{
			config: &config.Config{UseTemplateCache: true},
			Logger: logging.NewLogger(&logOutput, false),
			templateCache: template.TemplateCache{
				"ok.html": parsedTemplate,
			},
		}

		rr := httptest.NewRecorder()
		td := &templateData{User: &domain.UserPresentation{ID: 123}}

		a.renderTemplate(rr, td, "ok.html")

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		if got := rr.Header().Get("Content-Type"); got != "text/html; charset=UTF-8" {
			t.Fatalf("expected Content-Type %q, got %q", "text/html; charset=UTF-8", got)
		}

		if body := rr.Body.String(); !strings.Contains(body, "Hello, 123") {
			t.Fatalf("expected response body to contain rendered content, got %q", body)
		}

		if strings.Contains(logOutput.String(), "error rendering template") ||
			strings.Contains(logOutput.String(), "error writing template") {
			t.Fatalf("expected no rendering errors in logs, got %q", logOutput.String())
		}
	})
}

func Test_application_renderTemplateWithStatus(t *testing.T) {
	t.Run("renders template with specified status code", func(t *testing.T) {
		parsedTemplate, err := htmlTemplate.New("forbidden.html").Parse(`{{define "base"}}Hello, {{.User.ID}}{{end}}`)
		if err != nil {
			t.Fatalf("failed to parse test template: %v", err)
		}

		var logOutput bytes.Buffer
		a := application{
			config: &config.Config{UseTemplateCache: true},
			Logger: logging.NewLogger(&logOutput, false),
			templateCache: template.TemplateCache{
				"forbidden.html": parsedTemplate,
			},
		}

		rr := httptest.NewRecorder()
		td := &templateData{User: &domain.UserPresentation{ID: 123}}

		a.renderTemplateWithStatus(rr, td, http.StatusForbidden, "forbidden.html")

		if rr.Code != http.StatusForbidden {
			t.Fatalf("expected status %d, got %d", http.StatusForbidden, rr.Code)
		}

		if got := rr.Header().Get("Content-Type"); got != "text/html; charset=UTF-8" {
			t.Fatalf("expected Content-Type %q, got %q", "text/html; charset=UTF-8", got)
		}

		if body := rr.Body.String(); !strings.Contains(body, "Hello, 123") {
			t.Fatalf("expected response body to contain rendered content, got %q", body)
		}

		if strings.Contains(logOutput.String(), "error rendering template") ||
			strings.Contains(logOutput.String(), "error writing template") {
			t.Fatalf("expected no rendering errors in logs, got %q", logOutput.String())
		}
	})
}

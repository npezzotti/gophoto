package web

import (
	"bytes"
	"net/http"

	"github.com/justinas/nosurf"
	"github.com/npezzotti/gophoto/internal/domain"
	"github.com/npezzotti/gophoto/pkg/forms"
	"github.com/npezzotti/gophoto/pkg/pagination"
	"github.com/npezzotti/gophoto/pkg/template"
	templatesfs "github.com/npezzotti/gophoto/templates"
)

const (
	PagesGlob    = "pages/*.html"
	PartialsGlob = "partials/*.html"
	BaseTemplate = "base.html"
)

type templateData struct {
	Form                 forms.Form
	Flash                *Flash
	User                 *domain.UserPresentation
	Albums               []*domain.AlbumListItem
	Album                domain.Album
	Images               []domain.ResponsiveImage
	Paginator            *pagination.Pagination
	CSRFToken            string
	AddPhotoUploadAction string
}

func (a *application) generateTemplateData(r *http.Request) *templateData {
	td := &templateData{
		CSRFToken: nosurf.Token(r),
	}

	ctx := r.Context()
	if user, ok := extractUserFromContext(ctx); ok {
		td.User = user
	}

	flash, ok := a.sessionManager.Pop(ctx, sessionKeyFlash).(Flash)
	if ok {
		td.Flash = &flash
	}

	return td
}

func (a *application) renderTemplateWithStatus(w http.ResponseWriter, data *templateData, status int, tmpl string) {
	a.writeTemplateResponse(w, data, status, tmpl)
}

func (a *application) renderTemplate(w http.ResponseWriter, data *templateData, tmpl string) {
	a.writeTemplateResponse(w, data, http.StatusOK, tmpl)
}

func (a *application) writeTemplateResponse(w http.ResponseWriter, data *templateData, status int, tmpl string) {
	var tc template.TemplateCache
	if a.config.UseTemplateCache {
		tc = a.templateCache
	} else {
		newTemplateCache, err := template.NewTemplateCacheFromFS(templatesfs.FS, PagesGlob, PartialsGlob, BaseTemplate)
		if err != nil {
			a.Logger.Error("error creating template cache: %v", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		tc = newTemplateCache
	}

	var buf bytes.Buffer
	if err := tc.RenderTemplate(&buf, tmpl, data); err != nil {
		a.Logger.Error("error rendering template %s: %v", tmpl, err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	w.WriteHeader(status)

	if _, err := buf.WriteTo(w); err != nil {
		a.Logger.Error("error writing template %s: %v", tmpl, err)
	}
}

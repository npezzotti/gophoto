package web

import (
	"fmt"
	"net/http"

	"github.com/justinas/nosurf"
	"github.com/npezzotti/gophoto/internal/db"
	"github.com/npezzotti/gophoto/pkg/forms"
	"github.com/npezzotti/gophoto/pkg/pagination"
	"github.com/npezzotti/gophoto/pkg/template"
)

const (
	PagesGlob    = "./templates/pages/*.html"
	PartialsGlob = "./templates/partials/*.html"
	BaseTemplate = "./templates/base.html"
)

type templateData struct {
	Form                 forms.Form
	Flash                *Flash
	User                 *UserResponse
	Albums               []*AlbumResponse
	Album                db.GetAlbumRow
	Images               []*UserImageResponse
	Paginator            *pagination.Pagination
	CSRFToken            string
	AddPhotoUploadAction string
}

func (a *application) generateTemplateData(r *http.Request) *templateData {
	td := &templateData{
		CSRFToken: nosurf.Token(r),
	}

	user := a.getUserFromRequest(r)
	if user != nil {
		td.User = a.newUserResponse(r.Context(), user)
	}

	flash, ok := a.sessionManager.Pop(r.Context(), SessionKeyFlash).(Flash)
	if ok {
		td.Flash = &flash
	}

	return td
}

func (a *application) renderTemplate(w http.ResponseWriter, data *templateData, tmpl string) error {
	var tc template.TemplateCache

	if a.config.UseTemplateCache {
		tc = a.templateCache
	} else {
		tc, _ = template.NewTemplateCache(PagesGlob, PartialsGlob, BaseTemplate)
	}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")

	if err := tc.RenderTemplate(w, tmpl, data); err != nil {
		return fmt.Errorf("error rendering template %s: %w", tmpl, err)
	}
	return nil
}

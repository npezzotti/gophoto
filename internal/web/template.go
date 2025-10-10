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

	td.User = a.newUserResponse(r.Context(), a.getUserFromRequest(r))

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
		tc, _ = template.NewTemplateCache()
	}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")

	if err := tc.RenderTemplate(w, tmpl, data); err != nil {
		return fmt.Errorf("error rendering template %s: %w", tmpl, err)
	}
	return nil
}

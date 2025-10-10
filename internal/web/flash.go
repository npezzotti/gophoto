package web

import "net/http"

type flashClass string

const (
	flashInfo = flashClass("info")
	flashErr  = flashClass("danger")
)

type Flash struct {
	Message string
	Level   flashClass
}

func (a *application) flash(r *http.Request, msg string, level flashClass) {
	flash := Flash{
		Message: msg,
		Level:   level,
	}
	a.sessionManager.Put(r.Context(), SessionKeyFlash, flash)
}

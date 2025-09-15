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

func (a *application) Flash(r *http.Request, msg string, level flashClass) {
	flash := Flash{
		Message: msg,
		Level:   level,
	}
	a.sessionManager.Put(r.Context(), "__flash", flash)
}

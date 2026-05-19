package web

import "context"

type flashClass string

const (
	flashInfo = flashClass("info")
	flashErr  = flashClass("danger")
)

type Flash struct {
	Message string
	Level   flashClass
}

func (a *application) flash(ctx context.Context, msg string, level flashClass) {
	flash := Flash{
		Message: msg,
		Level:   level,
	}
	a.sessionManager.Put(ctx, SessionKeyFlash, flash)
}

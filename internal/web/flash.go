package web

import "context"

type flashClass string

const (
	flashInfo = flashClass("info")
	flashErr  = flashClass("danger")

	SessionKeyFlash = "__flash"
)

type Flash struct {
	Message string
	Level   flashClass
}

func (a *application) flash(ctx context.Context, msg string, level flashClass) {
	f := Flash{
		Message: msg,
		Level:   level,
	}
	a.sessionManager.Put(ctx, SessionKeyFlash, f)
}

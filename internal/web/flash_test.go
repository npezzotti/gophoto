package web

import (
	"context"
	"testing"

	"github.com/alexedwards/scs/v2"
)

func TestFlash__flash(t *testing.T) {
	a := application{
		sessionManager: scs.New(),
	}

	// Set up a test session context with a test session token
	ctx, err := a.sessionManager.Load(context.Background(), "test-session-token")
	if err != nil {
		t.Fatalf("failed to load session context: %v", err)
	}

	a.flash(ctx, "hello", flashInfo)
	val := a.sessionManager.Get(ctx, sessionKeyFlash)
	flash, ok := val.(Flash)
	if !ok {
		t.Fatalf("expected value to be of type \"Flash\", got %T", val)
	}

	expected := Flash{
		Message: "hello",
		Level:   flashInfo,
	}
	if flash != expected {
		t.Fatalf("got %v, expected %v", flash, expected)
	}
}

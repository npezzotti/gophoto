package web

import (
	"context"
	"testing"

	"github.com/alexedwards/scs/v2"
)

func TestFlash__flash(t *testing.T) {
	tcases := []struct {
		name       string
		flashLevel flashClass
	}{
		{
			name:       "info flash is stored in session",
			flashLevel: flashInfo,
		},
		{
			name:       "error flash is stored in session",
			flashLevel: flashErr,
		},
	}

	for _, tt := range tcases {
		t.Run(tt.name, func(t *testing.T) {
			const sessionToken = "test-session-token"

			a := application{
				sessionManager: scs.New(),
			}

			// Set up a test session context with a test session token
			ctx, err := a.sessionManager.Load(context.Background(), sessionToken)
			if err != nil {
				t.Fatalf("failed to load session context: %v", err)
			}

			a.flash(ctx, "hello", tt.flashLevel)
			val := a.sessionManager.Get(ctx, sessionKeyFlash)
			flash, ok := val.(Flash)
			if !ok {
				t.Fatalf("expected value to be of type \"Flash\", got %T", val)
			}

			expected := Flash{
				Message: "hello",
				Level:   tt.flashLevel,
			}
			if flash != expected {
				t.Fatalf("got %v, expected %v", flash, expected)
			}
		})
	}
}

package web

import (
	"context"
	"net/http"
	"testing"

	"github.com/npezzotti/gophoto/internal/domain"
)

func Test__isAuthenticated(t *testing.T) {
	r := &http.Request{}
	r = r.WithContext(context.Background())
	r = r.WithContext(context.WithValue(r.Context(), IsAuthenticatedContextKey, true))

	if !isAuthenticated(r) {
		t.Fatal("expected isAuthenticated to return true for request with no context, but got false")
	}
}

func Test__extractUserFromContext(t *testing.T) {
	ctx := context.Background()
	ctxWithUserID := context.WithValue(ctx, AuthenticatedUserContextKey, &domain.UserPresentation{ID: 123})
	user, ok := extractUserFromContext(ctxWithUserID)
	if !ok {
		t.Fatal("expected to find user in context, but did not")
	}
	if user.ID != 123 {
		t.Fatalf("expected user ID to be 123, got %d", user.ID)
	}
}

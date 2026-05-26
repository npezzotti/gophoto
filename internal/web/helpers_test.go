package web

import (
	"context"
	"net/http"
	"testing"

	"github.com/npezzotti/gophoto/internal/domain"
)

func Test__isAuthenticated(t *testing.T) {
	tcases := []struct {
		name     string
		request  *http.Request
		expected bool
	}{
		{
			name:     "request with no context returns false",
			request:  &http.Request{},
			expected: false,
		},
		{
			name: "request with context but no IsAuthenticatedContextKey returns false",
			request: func() *http.Request {
				r := &http.Request{}
				r = r.WithContext(context.Background())
				return r
			}(),
			expected: false,
		},
		{
			name: "request with context and IsAuthenticatedContextKey set to true returns true",
			request: func() *http.Request {
				r := &http.Request{}
				r = r.WithContext(context.Background())
				r = r.WithContext(context.WithValue(r.Context(), IsAuthenticatedContextKey, true))
				return r
			}(),
			expected: true,
		},
	}

	for _, tt := range tcases {
		t.Run(tt.name, func(t *testing.T) {
			got := isAuthenticated(tt.request)
			if got != tt.expected {
				t.Fatalf("expected isAuthenticated to return %v, got %v", tt.expected, got)
			}
		})
	}
}

func Test__extractUserFromContext(t *testing.T) {
	tcases := []struct {
		name string
		ctx  context.Context
		want *domain.UserPresentation
		ok   bool
	}{
		{
			name: "context with no user returns nil and false",
			ctx:  context.Background(),
			want: nil,
			ok:   false,
		},
		{
			name: "context with user returns user and true",
			ctx: func() context.Context {
				ctx := context.Background()
				ctx = context.WithValue(ctx, AuthenticatedUserContextKey, &domain.UserPresentation{ID: 123})
				return ctx
			}(),
			want: &domain.UserPresentation{ID: 123},
			ok:   true,
		},
	}

	for _, tt := range tcases {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := extractUserFromContext(tt.ctx)
			if ok != tt.ok {
				t.Fatalf("expected ok to be %v, got %v", tt.ok, ok)
			}

			if tt.want == nil {
				if got != nil {
					t.Fatalf("expected got to be nil, got %v", got)
				}
				return
			}

			if tt.want != nil && got == nil {
				t.Fatalf("expected got to be non-nil, got nil")
			}

			if tt.want != nil && got != nil && got.ID != tt.want.ID {
				t.Fatalf("expected user ID to be %v, got %v", tt.want.ID, got.ID)
			}
		})
	}
}

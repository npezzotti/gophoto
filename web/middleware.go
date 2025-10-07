package web

import (
	"context"
	"net/http"
	"strings"

	"github.com/justinas/nosurf"
)

type middleware func(http.Handler) http.Handler

func setupMiddleware(handler http.Handler, mw ...middleware) http.Handler {
	for i := len(mw) - 1; i >= 0; i-- {
		h := mw[i]
		if h != nil {
			handler = h(handler)
		}
	}

	return handler
}

func (a *application) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userId := a.sessionManager.GetInt32(r.Context(), SessionKeyUserID)
		if userId == 0 {
			next.ServeHTTP(w, r)
			return
		}

		exists, err := a.database.UserExists(r.Context(), userId)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}

		if exists {
			ctxIsAuth := context.WithValue(r.Context(), isAuthenticatedContextKey, true)
			ctx := context.WithValue(ctxIsAuth, authenticatedUserId, userId)

			r = r.WithContext(ctx)
		}

		next.ServeHTTP(w, r)
	})
}

func (a *application) protected(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isAuthenticated(r) {
			a.sessionManager.Put(r.Context(), SessionKeyFlash, Flash{
				Message: "You must be logged in to access this.",
				Level:   "danger",
			})

			a.sessionManager.Put(r.Context(), SessionKeyRedirectPath, r.URL.Path)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func noSurf(next http.Handler) http.Handler {
	csrfHandler := nosurf.New(next)
	csrfHandler.SetBaseCookie(http.Cookie{
		HttpOnly: true,
	})

	return csrfHandler
}

// checkFileOwnership is a middleware that checks if the authenticated user owns the file they are trying to access.
func (a *application) checkFileOwnership(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := a.getUserFromRequest(r)

		// URL format is /<baseDir>/<shard1>/<shard2>/<key>/<filename>
		// We need to extract the <key> part to look up the photo in the database.
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) < 5 {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}
		key := parts[len(parts)-2]

		photo, err := a.database.GetPhotoByKey(r.Context(), key)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}
		if photo.UserID != user.ID {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

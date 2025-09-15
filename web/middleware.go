package web

import (
	"context"
	"net/http"

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
		userId := a.sessionManager.GetInt32(r.Context(), "userId")
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
			a.sessionManager.Put(r.Context(), "__flash", Flash{
				Message: "You must be logged in to access this.",
				Level:   "danger",
			})

			a.sessionManager.Put(r.Context(), "redirectPath", r.URL.Path)
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

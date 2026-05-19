package web

import (
	"context"
	"encoding/base64"
	"net/http"
	"strconv"
	"time"

	"github.com/justinas/nosurf"
	"github.com/npezzotti/gophoto/pkg/store"
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

		exists, err := a.userService.UserExists(r.Context(), userId)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}

		if exists {
			ctxIsAuth := context.WithValue(r.Context(), IsAuthenticatedContextKey, true)
			ctx := context.WithValue(ctxIsAuth, AuthenticatedUserId, userId)

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

		// Disable caching for authenticated routes to prevent
		// users from seeing cached content after logging out
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")

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

// validatePresignedURL is a middleware that validates the presigned URL for accessing uploads.
func (a *application) validatePresignedURL(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !a.validUrl(r) {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (a *application) validUrl(r *http.Request) bool {
	queryParams := r.URL.Query()
	expiryStr := queryParams.Get("expires")
	b64signature := queryParams.Get("signature")

	if expiryStr == "" || b64signature == "" {
		return false
	}

	expiry, err := strconv.ParseInt(expiryStr, 10, 64)
	if err != nil {
		return false
	}

	if time.Now().Unix() > expiry {
		return false
	}

	receivedSig, err := base64.URLEncoding.DecodeString(b64signature)
	if err != nil {
		return false
	}

	message := store.CreateMessage(r.URL.Path, expiry)

	return store.VerifySignature(message, receivedSig, a.config.SigningKey)
}

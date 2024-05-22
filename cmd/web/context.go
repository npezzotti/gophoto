package main

type contextKey string

const (
	authenticatedUserId       = contextKey("authenticatedUserId")
	isAuthenticatedContextKey = contextKey("isAuthenticated")
)

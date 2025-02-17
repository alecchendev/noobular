package server

import (
	"net/http"
)

func handleSignupPage(w http.ResponseWriter, r *http.Request, ctx requestContext) error {
	return ctx.renderer.RenderSignupPage(w)
}

func handleSigninPage(w http.ResponseWriter, r *http.Request, ctx requestContext) error {
	return ctx.renderer.RenderSigninPage(w)
}


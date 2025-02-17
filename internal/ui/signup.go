package ui

import (
	"net/http"
)

type SignupPageArgs struct {
	Signin bool
}

func (r *Renderer) RenderSignupPage(w http.ResponseWriter) error {
	return r.templates["signup.html"].ExecuteTemplate(w, "page.html", newPageArgs(true, false, SignupPageArgs{false}))
}

func (r *Renderer) RenderSigninPage(w http.ResponseWriter) error {
	return r.templates["signup.html"].ExecuteTemplate(w, "page.html", newPageArgs(true, false, SignupPageArgs{true}))
}


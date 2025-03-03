package server

import (
	"net/http"
	"time"
)

func handleLogout(w http.ResponseWriter, r *http.Request, ctx requestContext, authCtx authContext) error {
	cookie, err := CreateAuthCookie(authCtx.jwtSecret, 0, authCtx.env == Production)
	if err != nil {
		return err
	}
	cookie.Expires = time.Unix(0, 0)
	http.SetCookie(w, &cookie)
	http.Redirect(w, r, "/", http.StatusSeeOther)
	return nil
}

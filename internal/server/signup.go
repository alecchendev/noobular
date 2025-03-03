package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/go-webauthn/webauthn/webauthn"
)

func handleSignupPage(w http.ResponseWriter, r *http.Request, ctx requestContext) error {
	return ctx.renderer.RenderSignupPage(w)
}

const maxUsernameLength = 64

func handleSignupBegin(w http.ResponseWriter, r *http.Request, ctx requestContext, authCtx authContext) error {
	username := r.URL.Query().Get("username")
	if username == "" {
		return fmt.Errorf("empty username")
	}
	if len(username) > maxUsernameLength {
		return fmt.Errorf("username too long, max %d characters", maxUsernameLength)
	}
	// TODO: delete user if they don't finish signup
	user, err := ctx.dbClient.CreateUser(username)
	if err != nil {
		return fmt.Errorf("Error creating user: %w", err)
	}
	webAuthnUser := NewWebAuthnUser(user)

	options, session, err := authCtx.webAuthn.BeginRegistration(&webAuthnUser)
	if err != nil {
		return fmt.Errorf("Error beginning registration: %v", err)
	}

	sessionBlob, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("Error marshalling session: %v", err)
	}
	err = ctx.dbClient.InsertSession(webAuthnUser.User.Id, sessionBlob)
	if err != nil {
		return fmt.Errorf("Error inserting session: %v", err)
	}

	optionsBlob, err := json.Marshal(options)
	if err != nil {
		return fmt.Errorf("Error marshalling options: %v", err)
	}
	w.Header().Add("Content-Type", "application/json")
	_, err = w.Write(optionsBlob)
	return err
}

func handleSignupFinish(w http.ResponseWriter, r *http.Request, ctx requestContext, authCtx authContext) error {
	username := r.URL.Query().Get("username")
	if username == "" {
		return fmt.Errorf("empty username")
	}
	user, ok, err := ctx.dbClient.GetUserByUsername(username)
	if err != nil {
		return fmt.Errorf("Error getting user: %w", err)
	}
	if !ok {
		return fmt.Errorf("User not found")
	}
	webAuthnUser := NewWebAuthnUser(user)

	sessionData, err := ctx.dbClient.GetSession(webAuthnUser.User.Id)
	if err != nil {
		return fmt.Errorf("Error getting session: %v", err)
	}
	var session webauthn.SessionData
	err = json.Unmarshal(sessionData, &session)
	if err != nil {
		return fmt.Errorf("Error unmarshalling session: %v", err)
	}

	webAuthnCredential, err := authCtx.webAuthn.FinishRegistration(&webAuthnUser, session, r)
	if err != nil {
		return fmt.Errorf("Error finishing registration: %v", err)
	}
	credential, err := NewCredential(webAuthnUser.User.Id, *webAuthnCredential)
	if err != nil {
		return fmt.Errorf("Error converting credential: %v", err)
	}
	err = ctx.dbClient.InsertCredential(credential)
	if err != nil {
		return fmt.Errorf("Error inserting credential: %v", err)
	}
	log.Printf("User %s registered with credentials", username)

	// TODO: add credentials to cookie and verify in auth middleware
	// This would mean even if attacker gets our server's jwt secret
	// they'd need to also compromise the user's webauthn device to forge a token
	cookie, err := CreateAuthCookie(authCtx.jwtSecret, webAuthnUser.User.Id, authCtx.env == Production)
	http.SetCookie(w, &cookie)
	return err
}

package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"noobular/internal/db"

	"github.com/go-webauthn/webauthn/webauthn"
)

func handleSigninPage(w http.ResponseWriter, r *http.Request, ctx requestContext) error {
	return ctx.renderer.RenderSigninPage(w)
}

func handleSigninBegin(w http.ResponseWriter, r *http.Request, ctx requestContext, authCtx authContext) error {
	username := r.URL.Query().Get("username")
	if username == "" {
		return fmt.Errorf("empty username")
	}

	webAuthnUser, err := getWebAuthnUser(ctx.dbClient, username)
	if err != nil {
		return fmt.Errorf("Error getting user: %v", err)
	}

	options, session, err := authCtx.webAuthn.BeginLogin(&webAuthnUser)
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

func handleSigninFinish(w http.ResponseWriter, r *http.Request, ctx requestContext, authCtx authContext) error {
	username := r.URL.Query().Get("username")
	if username == "" {
		return fmt.Errorf("empty username")
	}

	webAuthnUser, err := getWebAuthnUser(ctx.dbClient, username)
	if err != nil {
		return fmt.Errorf("Error getting user: %v", err)
	}

	sessionData, err := ctx.dbClient.GetSession(webAuthnUser.User.Id)
	if err != nil {
		return fmt.Errorf("Error getting session: %v", err)
	}
	var session webauthn.SessionData
	err = json.Unmarshal(sessionData, &session)
	if err != nil {
		return fmt.Errorf("Error unmarshalling session: %v", err)
	}

	webAuthnCredential, err := authCtx.webAuthn.FinishLogin(&webAuthnUser, session, r)
	if err != nil {
		return fmt.Errorf("Error finishing registration: %v", err)
	}

	// Prevent replay attacks by checking the sign count has been incremented
	if webAuthnCredential.Authenticator.CloneWarning {
		return fmt.Errorf("Sign count not incremented, key may have been cloned!")
	}

	credential, err := NewCredential(webAuthnUser.User.Id, *webAuthnCredential)
	if err != nil {
		return fmt.Errorf("Error converting credential: %v", err)
	}
	err = ctx.dbClient.UpdateCredential(credential)
	if err != nil {
		return fmt.Errorf("Error updating credential: %v", err)
	}
	log.Printf("User %s logged in with credentials", username)

	cookie, err := CreateAuthCookie(authCtx.jwtSecret, webAuthnUser.User.Id, authCtx.env == Production)
	if err != nil {
		return fmt.Errorf("Error creating auth cookie: %v", err)
	}
	http.SetCookie(w, &cookie)
	return nil
}

func getWebAuthnUser(dbClient db.DbClient, username string) (WebAuthnUser, error) {
	user, ok, err := dbClient.GetUserByUsername(username)
	if err != nil {
		return WebAuthnUser{}, fmt.Errorf("Error getting user: %v", err)
	}
	if !ok {
		return WebAuthnUser{}, fmt.Errorf("User not found")
	}
	credential, ok, err := dbClient.GetCredentialByUserId(user.Id)
	if err != nil {
		return WebAuthnUser{}, fmt.Errorf("Error getting credential: %v", err)
	}
	if !ok {
		return WebAuthnUser{}, fmt.Errorf("User has no credential")
	}
	webAuthnCredential, err := NewWebAuthnCredential(&credential)
	if err != nil {
		return WebAuthnUser{}, fmt.Errorf("Error converting credential: %v", err)
	}
	return NewWebAuthnUserWithCred(user, webAuthnCredential), nil
}

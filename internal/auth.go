package internal

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"

	"noobular/internal/db"
)

// Auth middleware used on various routes

func checkCookie(r *http.Request, jwtSecret []byte) (int64, error) {
	tokenCookie, err := r.Cookie("session_token")
	if err != nil {
		log.Println("No session token")
		return 0, err
	}
	userId, err := ValidateJwt(jwtSecret, tokenCookie.Value)
	if err != nil {
		log.Println("Invalid session token:", err)
		return 0, err
	}
	return userId, nil
}

func authRequiredHandler(handler UserHandler) HandlerMapHandler {
	return func(w http.ResponseWriter, r *http.Request, ctx HandlerContext) error {
		userId, err := checkCookie(r, ctx.jwtSecret)
		if err != nil {
			http.Redirect(w, r, "/signin", http.StatusSeeOther)
			return nil
		}
		user, err := ctx.dbClient.GetUser(userId)
		if err == sql.ErrNoRows {
			log.Println("User not found")
			http.Redirect(w, r, "/signup", http.StatusSeeOther)
			return nil
		}
		if err != nil {
			log.Println("Error getting user:", err)
			http.Redirect(w, r, "/signin", http.StatusSeeOther)
			return nil
		}
		return handler(w, r, ctx, user)
	}
}

func authOptionalHandler(handler OptionalUserHandler) HandlerMapHandler {
	return func(w http.ResponseWriter, r *http.Request, ctx HandlerContext) error {
		userId, err := checkCookie(r, ctx.jwtSecret)
		loggedIn := err == nil
		if loggedIn {
			user, err := ctx.dbClient.GetUser(userId)
			if err != nil {
				log.Println("Error getting user:", err)
				return handler(w, r, ctx, nil)
			} else {
				return handler(w, r, ctx, &user)
			}
		} else {
			return handler(w, r, ctx, nil)
		}
	}
}

func authRejectedHandler(handler HandlerMapHandler) HandlerMapHandler {
	return func(w http.ResponseWriter, r *http.Request, ctx HandlerContext) error {
		_, err := checkCookie(r, ctx.jwtSecret)
		if err == nil {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return nil
		}
		return handler(w, r, ctx)
	}
}

type WebAuthnHandler func(http.ResponseWriter, *http.Request, HandlerContext, *webauthn.WebAuthn) error

func withWebAuthn(webAuthn *webauthn.WebAuthn, handler WebAuthnHandler) HandlerMapHandler {
	return func(w http.ResponseWriter, r *http.Request, ctx HandlerContext) error {
		return handler(w, r, ctx, webAuthn)
	}
}

// Sign up page

func handleSignupPage(w http.ResponseWriter, r *http.Request, ctx HandlerContext) error {
	return ctx.renderer.RenderSignupPage(w)
}

// Sign in page

func handleSigninPage(w http.ResponseWriter, r *http.Request, ctx HandlerContext) error {
	return ctx.renderer.RenderSigninPage(w)
}

// Log out

func handleLogout(w http.ResponseWriter, r *http.Request, ctx HandlerContext, _ *db.User) error {
	cookie, err := CreateAuthCookie(ctx.jwtSecret, 0, ctx.env == Production)
	if err != nil {
		return err
	}
	cookie.Expires = time.Unix(0, 0)
	http.SetCookie(w, &cookie)
	http.Redirect(w, r, "/", http.StatusSeeOther)
	return nil
}

// Helpers

func GetWebAuthnUser(dbClient *db.DbClient, username string, create bool, failExistingCredential bool) (WebAuthnUser, error) {
	var user db.User
	user, err := dbClient.GetUserByUsername(username)
	if err == sql.ErrNoRows && create {
		user, err = dbClient.CreateUser(username)
		if err != nil {
			return WebAuthnUser{}, err
		}
	} else if err != nil {
		return WebAuthnUser{}, fmt.Errorf("Error getting user: %v", err)
	}
	credential, err := dbClient.GetCredentialByUserId(user.Id)
	if err == nil && failExistingCredential {
		return WebAuthnUser{}, fmt.Errorf("User already has a credential")
	} else if err == sql.ErrNoRows {
		return WebAuthnUser{user, []webauthn.Credential{}}, nil
	} else if err != nil {
		return WebAuthnUser{}, fmt.Errorf("Error getting credential: %v", err)
	}
	webAuthnCredential, err := NewWebAuthnCredential(&credential)
	if err != nil {
		return WebAuthnUser{}, fmt.Errorf("Error converting credential: %v", err)
	}
	return WebAuthnUser{user, []webauthn.Credential{webAuthnCredential}}, nil
}

func SaveSessionAndReturnOpts(w http.ResponseWriter, ctx HandlerContext, webAuthnUser WebAuthnUser, options interface{}, session *webauthn.SessionData) error {
	// Save session data for next request
	sessionBlob, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("Error marshalling session: %v", err)
	}
	err = ctx.dbClient.InsertSession(webAuthnUser.User.Id, sessionBlob)
	if err != nil {
		return fmt.Errorf("Error inserting session: %v", err)
	}
	// Write back option json to client
	optionsBlob, err := json.Marshal(options)
	if err != nil {
		return fmt.Errorf("Error marshalling options: %v", err)
	}
	w.Header().Add("Content-Type", "application/json")
	_, err = w.Write(optionsBlob)
	return err
}

func CreateAuthCookie(jwtSecret []byte, userId int64, httpsOnly bool) (http.Cookie, error) {
	expiry := time.Now().Add(2 * 7 * 24 * time.Hour) // 2 weeks
	token, err := CreateJwt(jwtSecret, userId, expiry)
	if err != nil {
		return http.Cookie{}, err
	}
	return http.Cookie{
		Name:     "session_token",
		Value:    token,
		Expires:  expiry,
		HttpOnly: true,                 // Not accessible to client side code
		SameSite: http.SameSiteLaxMode, // Cannot send cookie to other domains
		Secure:   httpsOnly,    // HTTPS only, need to disable locally
		Path:     "/",
	}, nil
}

// Webauthn sign up

func handleSignupBegin(w http.ResponseWriter, r *http.Request, ctx HandlerContext, webAuthn *webauthn.WebAuthn) error {
	username := r.URL.Query().Get("username")
	if username == "" {
		return fmt.Errorf("empty username")
	}
	webAuthnUser, err := GetWebAuthnUser(ctx.dbClient, username, true, true)
	if err != nil {
		return fmt.Errorf("Error getting webauthn user: %v", err)
	}

	// Begin registration
	options, session, err := webAuthn.BeginRegistration(&webAuthnUser)
	if err != nil {
		return fmt.Errorf("Error beginning registration: %v", err)
	}
	return SaveSessionAndReturnOpts(w, ctx, webAuthnUser, options, session)
}

func handleSignupFinish(w http.ResponseWriter, r *http.Request, ctx HandlerContext, webAuthn *webauthn.WebAuthn) error {
	username := r.URL.Query().Get("username")
	if username == "" {
		return fmt.Errorf("empty username")
	}
	webAuthnUser, err := GetWebAuthnUser(ctx.dbClient, username, false, false)
	if err != nil {
		return fmt.Errorf("Error getting webauthn user: %v", err)
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

	webAuthnCredential, err := webAuthn.FinishRegistration(&webAuthnUser, session, r)
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
	cookie, err := CreateAuthCookie(ctx.jwtSecret, webAuthnUser.User.Id, ctx.env == Production)
	http.SetCookie(w, &cookie)
	return nil
}

// Webauthn sign in

func handleSigninBegin(w http.ResponseWriter, r *http.Request, ctx HandlerContext, webAuthn *webauthn.WebAuthn) error {
	username := r.URL.Query().Get("username")
	if username == "" {
		return fmt.Errorf("empty username")
	}
	webAuthnUser, err := GetWebAuthnUser(ctx.dbClient, username, false, false)
	if err != nil {
		return fmt.Errorf("Error getting webauthn user: %v", err)
	}

	// Begin registration
	options, session, err := webAuthn.BeginLogin(&webAuthnUser)
	if err != nil {
		return fmt.Errorf("Error beginning registration: %v", err)
	}
	return SaveSessionAndReturnOpts(w, ctx, webAuthnUser, options, session)
}

func handleSigninFinish(w http.ResponseWriter, r *http.Request, ctx HandlerContext, webAuthn *webauthn.WebAuthn) error {
	username := r.URL.Query().Get("username")
	if username == "" {
		return fmt.Errorf("empty username")
	}
	webAuthnUser, err := GetWebAuthnUser(ctx.dbClient, username, false, false)
	if err != nil {
		return fmt.Errorf("Error getting webauthn user: %v", err)
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

	webAuthnCredential, err := webAuthn.FinishLogin(&webAuthnUser, session, r)
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

	cookie, err := CreateAuthCookie(ctx.jwtSecret, webAuthnUser.User.Id, ctx.env == Production)
	http.SetCookie(w, &cookie)
	return nil
}

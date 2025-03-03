package server

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"

	"noobular/internal/db"
	"noobular/internal/ui"

	"github.com/go-webauthn/webauthn/webauthn"
)

type Environment string

const (
	Local      Environment = "local"
	Production Environment = "production"
)

type ServerConfig struct {
	Env               Environment
	Port              int
	JwtSecret         []byte
	CertChainFilepath string
	PrivKeyFilepath   string
	WebAuthn          *webauthn.WebAuthn
}

func ParseServerConfig() ServerConfig {
	envStr := os.Getenv("ENVIRONMENT")
	env := Environment(envStr)
	if env != Local && env != Production {
		log.Println("No environment set: defaulting to local for server")
		env = Local
	}

	jwtSecretHex := os.Getenv("JWT_SECRET")
	if jwtSecretHex == "" {
		token := make([]byte, 32)
		_, err := rand.Read(token)
		if err != nil {
			log.Fatal(err)
		}
		log.Println("Example: set -x JWT_SECRET", hex.EncodeToString(token))
		log.Fatal("JWT_SECRET must be set")
	}
	jwtSecret, err := hex.DecodeString(jwtSecretHex)
	if err != nil {
		log.Fatal("JWT_SECRET must be a valid hex string")
	}

	urlStr := "http://localhost:8080"
	if env == Production {
		urlStr = "https://noobular.com"
	}
	urlUrl, err := url.Parse(urlStr)
	if err != nil {
		log.Fatal(err)
	}

	webAuthn, err := webauthn.New(&webauthn.Config{
		RPDisplayName: "Noobular",        // Display Name for your site
		RPID:          urlUrl.Hostname(), // Generally the domain name for your site
		RPOrigins:     []string{urlStr},  // The origin URL for WebAuthn requests
	})
	if err != nil {
		log.Fatal(err)
	}

	certChainFilepath := os.Getenv("CERT_PATH")
	privKeyFilepath := os.Getenv("PRIV_KEY_PATH")

	return ServerConfig{env, 8080, jwtSecret, certChainFilepath, privKeyFilepath, webAuthn}
}

func NewServer(db *sql.DB, renderer ui.Renderer, cfg ServerConfig) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	mux.Handle("/style/", http.StripPrefix("/style/", http.FileServer(http.Dir("style"))))

	newMethodHandlerMap := func() methodHandlerMap {
		return newMethodHandlerMap(db, renderer, cfg.Env)
	}

	authCtx := newAuthContext(cfg.Env, cfg.JwtSecret, cfg.WebAuthn)

	mux.Handle("/", newMethodHandlerMap().
		Get(authOptionalHandler(authCtx, handleHomePage)))

	mux.Handle("/signup", newMethodHandlerMap().
		Get(authRejectedHandler(authCtx, handleSignupPage)))
	mux.Handle("/signup/begin", newMethodHandlerMap().
		Get(authRejectedHandler(authCtx, withAuthCtx(authCtx, handleSignupBegin))))
	mux.Handle("/signup/finish", newMethodHandlerMap().
		Post(authRejectedHandler(authCtx, withAuthCtx(authCtx, handleSignupFinish))))

	mux.Handle("/signin", newMethodHandlerMap().
		Get(authRejectedHandler(authCtx, handleSigninPage)))
	mux.Handle("/signin/begin", newMethodHandlerMap().
		Get(authRejectedHandler(authCtx, withAuthCtx(authCtx, handleSigninBegin))))
	mux.Handle("/signin/finish", newMethodHandlerMap().
		Post(authRejectedHandler(authCtx, withAuthCtx(authCtx, handleSigninFinish))))

	mux.Handle("/logout", newMethodHandlerMap().
		Get(withAuthCtx(authCtx, handleLogout)))

	return &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: mux,
	}
}

var ErrPageNotFound = errors.New("Page not found")

func handleHomePage(w http.ResponseWriter, r *http.Request, ctx requestContext, user *db.User) error {
	if r.URL.Path != "/" {
		return ErrPageNotFound
	}
	return ctx.renderer.RenderHomePage(w, user != nil)
}

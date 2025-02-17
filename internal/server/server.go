package server

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"noobular/internal/db"
	"noobular/internal/ui"
	"os"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
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

func NewServer(dbClient db.DbClient, renderer ui.Renderer, cfg ServerConfig) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	mux.Handle("/style/", http.StripPrefix("/style/", http.FileServer(http.Dir("style"))))

	newMethodHandlerMap := func() methodHandlerMap {
		return newMethodHandlerMap(dbClient, renderer, cfg.Env)
	}

	mux.Handle("/", newMethodHandlerMap().
		Get(handleHomePage))
	mux.Handle("/signup", newMethodHandlerMap().
		Get(handleSignupPage))
	mux.Handle("/signin", newMethodHandlerMap().
		Get(handleSigninPage))


	return &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: mux,
	}
}

type requestContext struct {
	reqId    uuid.UUID
	dbClient db.DbClient
	renderer ui.Renderer
}

func newRequestContext(reqId uuid.UUID, dbClient db.DbClient, renderer ui.Renderer) requestContext {
	return requestContext{reqId: reqId, dbClient: dbClient, renderer: renderer}
}

type requestHandler func(
	w http.ResponseWriter,
	r *http.Request,
	ctx requestContext,
) error

type methodHandlerMap struct {
	dbClient db.DbClient
	renderer ui.Renderer
	handlers map[string]requestHandler
	env      Environment
}

func newMethodHandlerMap(dbClient db.DbClient, renderer ui.Renderer, env Environment) methodHandlerMap {
	return methodHandlerMap{
		dbClient: dbClient,
		renderer: renderer,
		handlers: make(map[string]requestHandler),
		env:      env,
	}
}

func (m methodHandlerMap) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var requestIdOpt uuid.NullUUID
	_ = requestIdOpt.UnmarshalText([]byte(r.Header.Get("X-Request-Id")))
	var requestId uuid.UUID
	if requestIdOpt.Valid {
		requestId = requestIdOpt.UUID
	} else {
		requestId = uuid.New()
	}
	err := r.ParseForm()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	log.Println(requestId, r.Method, r.URL.Path, r.Form)
	if m.env == Local {
		// Refresh templates so we don't need to restart
		// server to see changes.
		m.renderer.RefreshTemplates()
	}
	handler, ok := m.handlers[r.Method]
	if !ok {
		log.Printf("%s: Method %s not allowed for path %s\n", requestId, r.Method, r.URL.Path)
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	err = handler(w, r, newRequestContext(requestId, m.dbClient, m.renderer))
	switch {
	case errors.Is(err, ErrPageNotFound):
		http.Error(w, err.Error(), http.StatusNotFound)
	case err != nil:
		log.Printf("%s: %v\n", requestId, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (m methodHandlerMap) Get(handler requestHandler) methodHandlerMap {
	m.handlers["GET"] = handler
	return m
}

func (m methodHandlerMap) Post(handler requestHandler) methodHandlerMap {
	m.handlers["POST"] = handler
	return m
}

func (m methodHandlerMap) Put(handler requestHandler) methodHandlerMap {
	m.handlers["PUT"] = handler
	return m
}

func (m methodHandlerMap) Delete(handler requestHandler) methodHandlerMap {
	m.handlers["DELETE"] = handler
	return m
}

var ErrPageNotFound = errors.New("Page not found")

func handleHomePage(w http.ResponseWriter, r *http.Request, ctx requestContext) error {
	if r.URL.Path != "/" {
		return ErrPageNotFound
	}
	return ctx.renderer.RenderHomePage(w, false)
}

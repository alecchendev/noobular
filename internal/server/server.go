package server

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"noobular/internal/db"
	"noobular/internal/ui"

	"github.com/google/uuid"
)

type Environment string

const (
	Local      Environment = "local"
	Production Environment = "production"
)

func NewServer(port int, dbClient db.DbClient, renderer ui.Renderer, env Environment) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	mux.Handle("/style/", http.StripPrefix("/style/", http.FileServer(http.Dir("style"))))

	newMethodHandlerMap := func() methodHandlerMap {
		return newMethodHandlerMap(dbClient, renderer, env)
	}

	mux.Handle("/", newMethodHandlerMap().
		Get(handleHomePage))
	mux.Handle("/signup", newMethodHandlerMap().
		Get(handleSignupPage))
	mux.Handle("/signin", newMethodHandlerMap().
		Get(handleSigninPage))


	return &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
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

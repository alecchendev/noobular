package server

import (
	"database/sql"
	"errors"
	"log"
	"net/http"

	"noobular/internal/db"
	"noobular/internal/ui"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
)

type requestContext struct {
	reqId    uuid.UUID
	dbClient db.DbClient
	renderer ui.Renderer
}

func newRequestContext(reqId uuid.UUID, tx *sql.Tx, renderer ui.Renderer) requestContext {
	return requestContext{reqId: reqId, dbClient: db.NewDbClient(tx), renderer: renderer}
}

type requestHandler func(
	w http.ResponseWriter,
	r *http.Request,
	ctx requestContext,
) error

type methodHandlerMap struct {
	db       *sql.DB
	renderer ui.Renderer
	handlers map[string]requestHandler
	env      Environment
}

func newMethodHandlerMap(db *sql.DB, renderer ui.Renderer, env Environment) methodHandlerMap {
	return methodHandlerMap{
		db:       db,
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
	tx, err := m.db.Begin()
	if err != nil {
		log.Printf("%s: %v\n", requestId, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = handler(w, r, newRequestContext(requestId, tx, m.renderer))
	if err != nil {
		log.Printf("%s: %v\n", requestId, err)
		rollbackErr := tx.Rollback()
		if rollbackErr != nil {
			log.Printf("%s: rollback err: %v\n", requestId, rollbackErr)
		}
	} else {
		err = tx.Commit()
		if err != nil {
			log.Printf("%s: commit err: %v\n", requestId, err)
		}
	}
	switch {
	case errors.Is(err, ErrPageNotFound):
		http.Error(w, err.Error(), http.StatusNotFound)
	case err != nil:
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

type authContext struct {
	env       Environment
	jwtSecret []byte
	webAuthn  *webauthn.WebAuthn
}

func newAuthContext(env Environment, jwtSecret []byte, webAuthn *webauthn.WebAuthn) authContext {
	return authContext{env, jwtSecret, webAuthn}
}

type authHandler func(w http.ResponseWriter, r *http.Request, ctx requestContext, authCtx authContext) error

func withAuthCtx(authCtx authContext, handler authHandler) requestHandler {
	return func(w http.ResponseWriter, r *http.Request, ctx requestContext) error {
		return handler(w, r, ctx, authCtx)
	}
}

package server

import (
	"fmt"
	"log"
	"net/http"
	"noobular/internal/db"

	"github.com/google/uuid"
)

func NewServer(port int, dbClient *db.DbClient) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	mux.Handle("/style/", http.StripPrefix("/style/", http.FileServer(http.Dir("style"))))

	newMethodHandlerMap := func() methodHandlerMap {
		return newMethodHandlerMap(dbClient)
	}

	mux.Handle("/", newMethodHandlerMap().
		Get(func(w http.ResponseWriter, r *http.Request, ctx requestContext) {
			fmt.Fprintf(w, "Hello, World!")
		}))

	return &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}
}

type requestContext struct {
	reqId uuid.UUID
	dbClient  *db.DbClient
}

func newRequestContext(reqId uuid.UUID, dbClient *db.DbClient) requestContext {
	return requestContext{ reqId: reqId, dbClient: dbClient }
}

type requestHandler func(w http.ResponseWriter, r *http.Request, ctx requestContext)

type methodHandlerMap struct {
	dbClient *db.DbClient
	handlers map[string]requestHandler
}

func newMethodHandlerMap(dbClient *db.DbClient) methodHandlerMap {
	return methodHandlerMap{ dbClient: dbClient, handlers: make(map[string]requestHandler) }
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
	handler, ok := m.handlers[r.Method]
	if !ok {
		log.Printf("%s: Method %s not allowed for path %s\n", requestId, r.Method, r.URL.Path)
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	handler(w, r, newRequestContext(requestId, m.dbClient))
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

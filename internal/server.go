package internal

import (
	"fmt"
	"log"
	"net/http"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"

	"noobular/internal/db"
)

func NewServer(dbClient *db.DbClient, renderer Renderer, webAuthn *webauthn.WebAuthn, jwtSecret []byte, port int) *http.Server {
	router := initRouter(dbClient, renderer, webAuthn, jwtSecret)
	return &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: router,
	}
}

func initRouter(dbClient *db.DbClient, renderer Renderer, webAuthn *webauthn.WebAuthn, jwtSecret []byte) *http.ServeMux {
	newHandlerMap := func() HandlerMap {
		return NewHandlerMap(dbClient, renderer, jwtSecret)
	}
	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	mux.Handle("/style/", http.StripPrefix("/style/", http.FileServer(http.Dir("style"))))

	mux.Handle("/", newHandlerMap().Get(authOptionalHandler(handleHomePage)))
	mux.Handle("/browse", newHandlerMap().Get(authOptionalHandler(handleBrowsePage)))

	mux.Handle("/signup", newHandlerMap().Get(authRejectedHandler(handleSignupPage)))
	mux.Handle("/signin", newHandlerMap().Get(authRejectedHandler(handleSigninPage)))
	mux.Handle("/signup/begin", newHandlerMap().Get(authRejectedHandler(withWebAuthn(webAuthn, handleSignupBegin))))
	mux.Handle("/signup/finish", newHandlerMap().Post(authRejectedHandler(withWebAuthn(webAuthn, handleSignupFinish))))
	mux.Handle("/signin/begin", newHandlerMap().Get(authRejectedHandler(withWebAuthn(webAuthn, handleSigninBegin))))
	mux.Handle("/signin/finish", newHandlerMap().Post(authRejectedHandler(withWebAuthn(webAuthn, handleSigninFinish))))
	mux.Handle("/logout", newHandlerMap().Get(authOptionalHandler(handleLogout)))

	mux.Handle("/student", newHandlerMap().Get(authRequiredHandler(handleStudentPage)))
	mux.Handle("/student/course/{courseId}", newHandlerMap().Get(authRequiredHandler(handleStudentCoursePage)).Post(authRequiredHandler(handleTakeCourse)))
	mux.Handle("/student/course/{courseId}/module/{moduleId}", newHandlerMap().Get(authRequiredHandler(handleTakeModulePage)))
	mux.Handle("/student/course/{courseId}/module/{moduleId}/block/{blockIdx}/piece", newHandlerMap().Get(authRequiredHandler(handleTakeModule)))
	mux.Handle("/student/course/{courseId}/module/{moduleId}/block/{blockIdx}/answer", newHandlerMap().Post(authRequiredHandler(handleAnswerQuestion)))
	mux.Handle("/student/course/{courseId}/module/{moduleId}/complete", newHandlerMap().Put(authRequiredHandler(handleCompleteModule)))

	mux.Handle("/teacher", newHandlerMap().Get(authRequiredHandler(handleTeacherCoursesPage)))
	mux.Handle("/teacher/course/create", newHandlerMap().Get(authRequiredHandler(handleCreateCoursePage)).Post(authRequiredHandler(handleCreateCourse)))
	mux.Handle("/teacher/course/{courseId}/edit", newHandlerMap().Get(authRequiredHandler(handleEditCoursePage)))
	mux.Handle("/teacher/course/{courseId}", newHandlerMap().Put(authRequiredHandler(handleEditCourse)).Delete(authRequiredHandler(handleDeleteCourse)))
	mux.Handle("/teacher/course/{courseId}/module/{moduleId}", newHandlerMap().Put(authRequiredHandler(handleEditModule)).Delete(authRequiredHandler(handleDeleteModule)))
	mux.Handle("/teacher/course/{courseId}/module/{moduleId}/edit", newHandlerMap().Get(authRequiredHandler(handleEditModulePage)))
	mux.Handle("/teacher/course/{courseId}/module/{moduleId}/preview", newHandlerMap().Get(authRequiredHandler(handlePreviewModulePage)))

	mux.Handle("/ui/{questionIdx}/choice", newHandlerMap().Get(handleAddChoice))
	mux.Handle("/ui/{element}", newHandlerMap().Get(handleAddElement).Delete(handleDeleteElement))

	return mux
}

// Things that all handlers should have access to
type HandlerContext struct {
	dbClient     *db.DbClient
	renderer     Renderer
	jwtSecret    []byte
}

func NewHandlerContext(dbClient *db.DbClient, renderer Renderer, jwtSecret []byte) HandlerContext {
	return HandlerContext{dbClient, renderer, jwtSecret}
}

// Basically an http.Handle but returns an error
type HandlerMapHandler func(http.ResponseWriter, *http.Request, HandlerContext) error

type HandlerMap struct {
	handlers        map[string]HandlerMapHandler
	ctx             HandlerContext
	reloadTemplates bool
}

func (hm HandlerMap) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var requestIdOpt uuid.NullUUID
	requestIdOpt.UnmarshalText([]byte(r.Header.Get("X-Request-Id")))
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
	if hm.reloadTemplates {
		// Reload templates so we don't have to restart the server
		// to see changes
		hm.ctx.renderer.refreshTemplates()
	}
	handler, ok := hm.handlers[r.Method]
	if !ok {
		log.Printf("%s Method %s not allowed for path %s", requestId, r.Method, r.URL.Path)
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	}
	err = handler(w, r, hm.ctx)
	if err != nil {
		log.Println(requestId, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func NewHandlerMap(dbClient *db.DbClient, renderer Renderer, jwtSecret []byte) HandlerMap {
	return HandlerMap{
		handlers:        make(map[string]HandlerMapHandler),
		ctx:             NewHandlerContext(dbClient, renderer, jwtSecret),
		reloadTemplates: true,
	}
}

func (hm HandlerMap) Get(handler HandlerMapHandler) HandlerMap {
	hm.handlers["GET"] = handler
	return hm
}

func (hm HandlerMap) Post(handler HandlerMapHandler) HandlerMap {
	hm.handlers["POST"] = handler
	return hm
}

func (hm HandlerMap) Put(handler HandlerMapHandler) HandlerMap {
	hm.handlers["PUT"] = handler
	return hm
}

func (hm HandlerMap) Delete(handler HandlerMapHandler) HandlerMap {
	hm.handlers["DELETE"] = handler
	return hm
}

type UserHandler func(http.ResponseWriter, *http.Request, HandlerContext, db.User) error

type AnyoneHandler func(http.ResponseWriter, *http.Request, HandlerContext, bool) error

// Home page

func handleHomePage(w http.ResponseWriter, r *http.Request, ctx HandlerContext, loggedIn bool) error {
	if r.URL.Path != "/" {
		// TODO: We should return 404 here
		return fmt.Errorf("Not found")
	}
	return ctx.renderer.RenderHomePage(w, loggedIn)
}

// Browse page

func handleBrowsePage(w http.ResponseWriter, r *http.Request, ctx HandlerContext, loggedIn bool) error {
	courses, err := ctx.dbClient.GetCourses()
	if err != nil {
		return err
	}
	uiCourses := make([]UiCourse, len(courses))
	for i, course := range courses {
		modules, err := ctx.dbClient.GetModules(course.Id)
		if err != nil {
			return err
		}
		uiModules := make([]UiModule, 0)
		for _, module := range modules {
			moduleVersion, err := ctx.dbClient.GetLatestModuleVersion(module.Id)
			if err != nil {
				return err
			}
			blockCount, err := ctx.dbClient.GetBlockCount(moduleVersion.Id)
			if err != nil {
				return err
			}
			if blockCount == 0 {
				continue
			}
			uiModules = append(uiModules, NewUiModule(course.Id, moduleVersion))
		}
		uiCourses[i] = NewUiCourse(course, uiModules)
	}
	return ctx.renderer.RenderBrowsePage(w, uiCourses, loggedIn)
}

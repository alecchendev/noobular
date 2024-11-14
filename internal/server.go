package internal

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
)

func NewServer(dbClient *DbClient, port int) *http.Server {
	router := initRouter(dbClient)
	return &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: router,
	}
}

func initRouter(dbClient *DbClient) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("/course/create", NewHandlerMap(dbClient).Get(handleCreateCoursePage).Post(handleCreateCourse))
	mux.Handle("/course/{courseId}/edit", NewHandlerMap(dbClient).Get(handleEditCoursePage).Put(handleEditCourse))
	mux.Handle("/ui/{element}", NewHandlerMap(dbClient).Get(handleAddElement).Delete(handleDeleteElement))
	// This is kinda a weird place to put the deleteModuleHandler because it's on a different page
	// (the edit course page) but it's fine for now.
	mux.Handle("/course/{courseId}/module/{moduleId}/edit", NewHandlerMap(dbClient).Get(handleEditModulePage).Put(handleEditModule).Delete(handleDeleteModule))
	mux.Handle("/course", NewHandlerMap(dbClient).Get(handleCoursesPage))
	mux.Handle("/student/course", NewHandlerMap(dbClient).Get(handleStudentCoursesPage))
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	mux.Handle("/style/", http.StripPrefix("/style/", http.FileServer(http.Dir("style"))))
	mux.Handle("/", NewHandlerMap(dbClient).Get(handleHomePage))
	return mux
}

// Things that all handlers should have access to
type HandlerContext struct {
	dbClient *DbClient
	renderer Renderer
}

func NewHandlerContext(dbClient *DbClient, renderer Renderer) HandlerContext {
	return HandlerContext{dbClient, renderer}
}

// Basically an http.Handle but returns an error
type HandlerMapHandler func(http.ResponseWriter, *http.Request, HandlerContext) error

type HandlerMap struct {
	handlers        map[string]HandlerMapHandler
	ctx		HandlerContext
	reloadTemplates bool
}

func (hm HandlerMap) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	log.Println(r.Method, r.URL.Path, r.Form)
	if hm.reloadTemplates {
		// Reload templates so we don't have to restart the server
		// to see changes
		hm.ctx.renderer.refreshTemplates()
	}
	if handler, ok := hm.handlers[r.Method]; ok {
		err := handler(w, r, hm.ctx)
		if err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
}

func NewHandlerMap(dbClient *DbClient) HandlerMap {
	return HandlerMap{
		handlers:        make(map[string]HandlerMapHandler),
		ctx:             NewHandlerContext(dbClient, NewRenderer()),
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

// Courses page

func handleCoursesPage(w http.ResponseWriter, r *http.Request, ctx HandlerContext) error {
	newCourseId, err := strconv.Atoi(r.URL.Query().Get("newCourse"))
	if err != nil {
		newCourseId = -1
	}
	courses, err := ctx.dbClient.GetCourses()
	if err != nil {
		return err
	}
	return ctx.renderer.RenderTeacherCoursePage(w, courses, newCourseId)
}

func handleStudentCoursesPage(w http.ResponseWriter, r *http.Request, ctx HandlerContext) error {
	courses, err := ctx.dbClient.GetCourses()
	if err != nil {
		return err
	}
	return ctx.renderer.RenderStudentCoursePage(w, courses)
}

// Create course page

func handleCreateCoursePage(w http.ResponseWriter, r *http.Request, ctx HandlerContext) error {
	return ctx.renderer.RenderCreateCoursePage(w)
}

type createCourseRequest struct {
	title              string
	description        string
	moduleTitles       []string
	moduleDescriptions []string
}

func parseCreateCourseRequest(r *http.Request) (createCourseRequest, error) {
	err := r.ParseForm()
	if err != nil {
		return createCourseRequest{}, err
	}
	title := r.Form.Get("title")
	description := r.Form.Get("description")
	moduleTitles := r.Form["module-title[]"]
	moduleDescriptions := r.Form["module-description[]"]
	return createCourseRequest{title, description, moduleTitles, moduleDescriptions}, nil
}


func handleCreateCourse(w http.ResponseWriter, r *http.Request, ctx HandlerContext) error {
	req, err := parseCreateCourseRequest(r)
	if err != nil {
		return err
	}
	course, _, err := ctx.dbClient.CreateCourse(req.title, req.description, req.moduleTitles, req.moduleDescriptions)
	if err != nil {
		return err
	}
	w.Header().Add("HX-Redirect", fmt.Sprintf("/course?newCourse=%d#course-%d", course.Id, course.Id))
	return nil
}

// Edit course page

func handleEditCoursePage(w http.ResponseWriter, r *http.Request, ctx HandlerContext) error {
	courseId, err := strconv.Atoi(r.PathValue("courseId"))
	if err != nil {
		return err
	}
	course, err := ctx.dbClient.GetCourse(courseId)
	if err != nil {
		return err
	}
	return ctx.renderer.RenderEditCoursePage(w, course)
}

type editCourseRequest struct {
	courseId           int
	title              string
	description        string
	moduleIds          []int
	moduleTitles       []string
	moduleDescriptions []string
}

func parseEditCourseRequest(r *http.Request) (editCourseRequest, error) {
	courseId, err := strconv.Atoi(r.PathValue("courseId"))
	if err != nil {
		return editCourseRequest{}, err
	}
	err = r.ParseForm()
	if err != nil {
		return editCourseRequest{}, err
	}
	title := r.Form.Get("title")
	description := r.Form.Get("description")
	moduleIdStrs := r.Form["module-id[]"]
	moduleIds := make([]int, len(moduleIdStrs))
	for i, moduleIdStr := range moduleIdStrs {
		moduleIdInt, err := strconv.Atoi(moduleIdStr)
		if err != nil {
			return editCourseRequest{}, err
		}
		moduleIds[i] = moduleIdInt
	}
	moduleTitles := r.Form["module-title[]"]
	moduleDescriptions := r.Form["module-description[]"]
	return editCourseRequest{courseId, title, description, moduleIds, moduleTitles, moduleDescriptions}, nil
}

func handleEditCourse(w http.ResponseWriter, r *http.Request, ctx HandlerContext) error {
	req, err := parseEditCourseRequest(r)
	if err != nil {
		return err
	}
	_, _, err = ctx.dbClient.EditCourse(req.courseId, req.title, req.description, req.moduleIds, req.moduleTitles, req.moduleDescriptions)
	if err != nil {
		return err
	}
	return ctx.renderer.RenderCourseCreated(w)
}

// Add element generic template
// These are used in multiple pages, for adding modules, questions, choices.

// These two handlers seem kinda dumb, i.e. they could just be done in javascript,
// but I'm just going to do things the pure HTMX way for now to see how it goes.

// Simply returns another small chunk of HTML to add new elements
func handleAddElement(w http.ResponseWriter, r *http.Request, ctx HandlerContext) error {
	element := r.PathValue("element")
	var err error
	if element == "module" {
		err = ctx.renderer.RenderNewModule(w, EmptyModule())
	} else if element == "question" {
		err = ctx.renderer.RenderNewQuestion(w, EmptyQuestion())
	} else if element == "choice" {
		err = ctx.renderer.RenderNewChoice(w, EmptyChoice())
	} else {
		err = fmt.Errorf("Unknown element: %s", element)
	}
	return err
}

func handleDeleteElement(w http.ResponseWriter, r *http.Request, ctx HandlerContext) error {
	// No op
	return nil
}

func handleDeleteModule(w http.ResponseWriter, r *http.Request, ctx HandlerContext) error {
	moduleId, err := strconv.Atoi(r.PathValue("moduleId"))
	if err != nil {
		return err
	}
	err = ctx.dbClient.DeleteModule(moduleId)
	if err != nil {
		return err
	}
	// Nothing to render
	return nil
}

// Edit module page

func handleEditModulePage(w http.ResponseWriter, r *http.Request, ctx HandlerContext) error {
	// Get courseId and moduleId from "/course/:courseId/module/:moduleId/edit"
	courseId, err := strconv.Atoi(r.PathValue("courseId"))
	if err != nil {
		return err
	}
	moduleId, err := strconv.Atoi(r.PathValue("moduleId"))
	if err != nil {
		return err
	}
	uiEditModule, err := ctx.dbClient.GetEditModule(courseId, moduleId)
	if err != nil {
		return err
	}
	return ctx.renderer.RenderEditModulePage(w, uiEditModule)
}

type editModuleRequest struct {
	moduleId int
	title    string
	description string
	questions []string
	choicesByQuestion [][]string
}

func parseEditModuleRequest(r *http.Request) (editModuleRequest, error) {
	moduleId, err := strconv.Atoi(r.PathValue("moduleId"))
	if err != nil {
		return editModuleRequest{}, err
	}
	err = r.ParseForm()
	if err != nil {
		return editModuleRequest{}, err
	}
	title := r.Form.Get("title")
	description := r.Form.Get("description")
	questions := r.Form["question-title[]"]
	choices := r.Form["choice-title[]"]
	// Choices are separated by "end-choice" in the form
	// i.e. we expect r.Form["choice-title[]"] to look something like:
	// ["choice1", "choice2", "end-choice", "choice3", "choice4", "end-choice"]
	uiQuestions := make([]string, len(questions))
	uiChoicesByQuestion := make([][]string, len(questions))
	choiceIdx := 0
	for i, question := range questions {
		uiChoices := make([]string, 0)
		for ; choiceIdx < len(choices); choiceIdx++ {
			choice := choices[choiceIdx]
			if choice == "end-choice" {
				choiceIdx++
				break
			}
			uiChoices = append(uiChoices, choice)
		}
		uiQuestions[i] = question
		uiChoicesByQuestion[i] = uiChoices
	}
	for i, question := range uiQuestions {
		if question == "" {
			return editModuleRequest{}, fmt.Errorf("Questions  cannot be empty")
		}
		if len(uiChoicesByQuestion[i]) == 0 {
			return editModuleRequest{}, fmt.Errorf("Questions must have at least one choice")
		}
		for _, choice := range uiChoicesByQuestion[i] {
			if choice == "" {
				return editModuleRequest{}, fmt.Errorf("Choices cannot be empty")
			}
		}
	}
	return editModuleRequest{moduleId, title, description, uiQuestions, uiChoicesByQuestion}, nil
}

func handleEditModule(w http.ResponseWriter, r *http.Request, ctx HandlerContext) error {
	req, err := parseEditModuleRequest(r)
	if err != nil {
		return err
	}
	err = ctx.dbClient.EditModule(req.moduleId, req.title, req.description, req.questions, req.choicesByQuestion)
	if err != nil {
		return err
	}
	return ctx.renderer.RenderModuleEdited(w)
}

// Home page

func handleHomePage(w http.ResponseWriter, r *http.Request, ctx HandlerContext) error {
	if r.URL.Path != "/" {
		// TODO: We should return 404 here
		return fmt.Errorf("Not found")
	}
	return ctx.renderer.RenderHomePage(w)
}

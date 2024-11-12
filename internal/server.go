package internal

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
)

func NewServer(dbClient *DbClient, port int) *http.Server {
	router := initRouter(dbClient)
	return &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: router,
	}
}


func initTemplates() map[string]*template.Template {
	funcMap := template.FuncMap{
		"TitleCase": func(s string) string {
			return strings.Title(strings.ToLower(s))
		},
		"EmptyModule": func () UiModule {
			return EmptyModule()
		},
		"EmptyQuestion": func () UiQuestion {
			return EmptyQuestion()
		},
		"EmptyChoice": func () UiChoice {
			return EmptyChoice()
		},
	}
	return map[string]*template.Template{
		// Pages
		"index.html":   template.Must(template.ParseFiles("template/page.html", "template/index.html")),
		"courses.html": template.Must(template.ParseFiles("template/page.html", "template/courses.html")),
		"create_course.html": template.Must(template.New("").Funcs(funcMap).ParseFiles(
			"template/page.html", "template/create_course.html",
			"template/add_element.html", "template/created_course_response.html",
			"template/edited_course_response.html")),
		"edit_module.html": template.Must(template.New("").Funcs(funcMap).ParseFiles(
			"template/page.html", "template/edit_module.html", "template/add_element.html",
			"template/edited_module_response.html")),
		// Standalone partials
		"add_element.html": template.Must(template.New("").Funcs(funcMap).ParseFiles("template/add_element.html")),
	}
}

var templates = initTemplates()

type HandlerMap struct {
	handlers        map[string]func(http.ResponseWriter, *http.Request)
	reloadTemplates bool
}

func (hm HandlerMap) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if hm.reloadTemplates {
		// Reload templates so we don't have to restart the server
		// to see changes
		templates = initTemplates()
	}
	if handler, ok := hm.handlers[r.Method]; ok {
		handler(w, r)
		return
	}
	http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
}

func NewHandlerMap() HandlerMap {
	return HandlerMap{
		handlers:        make(map[string]func(http.ResponseWriter, *http.Request)),
		reloadTemplates: true,
	}
}

func (hm HandlerMap) Get(handler func(http.ResponseWriter, *http.Request)) HandlerMap {
	hm.handlers["GET"] = handler
	return hm
}

func (hm HandlerMap) Post(handler func(http.ResponseWriter, *http.Request)) HandlerMap {
	hm.handlers["POST"] = handler
	return hm
}

func (hm HandlerMap) Put(handler func(http.ResponseWriter, *http.Request)) HandlerMap {
	hm.handlers["PUT"] = handler
	return hm
}

func (hm HandlerMap) Delete(handler func(http.ResponseWriter, *http.Request)) HandlerMap {
	hm.handlers["DELETE"] = handler
	return hm
}

func withDbClient(dbClient *DbClient, handler func(http.ResponseWriter, *http.Request, *DbClient)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		handler(w, r, dbClient)
	}
}

func coursePageHandler(w http.ResponseWriter, r *http.Request, dbClient *DbClient) {
	courses, err := dbClient.GetCourses()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	templates["courses.html"].ExecuteTemplate(w, "page.html", courses)
}

func createCourseHandler(w http.ResponseWriter, r *http.Request, dbClient *DbClient) {
	fmt.Println("Creating course")
	err := r.ParseForm()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	title := r.Form.Get("title")
	description := r.Form.Get("description")
	moduleTitles := r.Form["module-title[]"]
	moduleDescriptions := r.Form["module-description[]"]
	course, modules, err := dbClient.CreateCourse(title, description, moduleTitles, moduleDescriptions)
	if err != nil {
		fmt.Println("Error creating course:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Println("Created course:", course)
	fmt.Println("Created modules:", modules)
	templates["create_course.html"].ExecuteTemplate(w, "created_course_response.html", nil)
}

func editCourseHandler(w http.ResponseWriter, r *http.Request, dbClient *DbClient) {
	fmt.Println("Editing course")
	courseId, err := strconv.Atoi(r.PathValue("courseId"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = r.ParseForm()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	title := r.Form.Get("title")
	description := r.Form.Get("description")
	moduleIdStrs := r.Form["module-id[]"]
	moduleIds := make([]int, len(moduleIdStrs))
	for i, moduleIdStr := range moduleIdStrs {
		moduleIdInt, err := strconv.Atoi(moduleIdStr)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		moduleIds[i] = moduleIdInt
	}
	moduleTitles := r.Form["module-title[]"]
	moduleDescriptions := r.Form["module-description[]"]
	course, modules, err := dbClient.EditCourse(courseId, title, description, moduleIds, moduleTitles, moduleDescriptions)
	if err != nil {
		fmt.Println("Error editing course:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Println("Edited course:", course)
	fmt.Println("Edited modules:", modules)
	templates["create_course.html"].ExecuteTemplate(w, "edited_course_response.html", nil)
}

func createCoursePageHandler(w http.ResponseWriter, r *http.Request) {
	templates["create_course.html"].ExecuteTemplate(w, "page.html", EmptyCourse())
}

func editCoursePageHandler(w http.ResponseWriter, r *http.Request, dbClient *DbClient) {
	courseId, err := strconv.Atoi(r.PathValue("courseId"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	course, err := dbClient.GetCourse(courseId)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Println("Editing course:", course)
	fmt.Println("Course modules:", course.Modules)
	templates["create_course.html"].ExecuteTemplate(w, "page.html", course)
}

// These two handlers seem kinda dumb, i.e. they could just be done in javascript,
// but I'm just going to do things the pure HTMX way for now to see how it goes.

// Simply returns another small chunk of HTML to add new elements
func addElementHandler(w http.ResponseWriter, r *http.Request) {
	var data interface{}
	switch r.PathValue("element") {
		case "module":
			data = EmptyModule()
			break
		case "question":
			data = EmptyQuestion()
			break
		case "choice":
			data = EmptyChoice()
			break
	}
	templates["add_element.html"].ExecuteTemplate(w, "add_element.html", data)

}

func deleteElementHandler(w http.ResponseWriter, r *http.Request) {
	// No op
}

func deleteModuleHandler(w http.ResponseWriter, r *http.Request, dbClient *DbClient) {
	moduleId, err := strconv.Atoi(r.PathValue("moduleId"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = dbClient.DeleteModule(moduleId)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func editModulePageHandler(w http.ResponseWriter, r *http.Request, dbClient *DbClient) {
	// Get courseId and moduleId from "/course/:courseId/module/:moduleId/edit"

	courseId, err := strconv.Atoi(r.PathValue("courseId"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	moduleId, err := strconv.Atoi(r.PathValue("moduleId"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	uiModule, err := dbClient.GetEditModule(courseId, moduleId)

	err = templates["edit_module.html"].ExecuteTemplate(w, "page.html", uiModule)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func editModuleHandler(w http.ResponseWriter, r *http.Request, dbClient *DbClient) {
	fmt.Println("Editing module")
	moduleId, err := strconv.Atoi(r.PathValue("moduleId"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = r.ParseForm()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
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

	// Validation
	for i, question := range uiQuestions {
		if question == "" {
			http.Error(w, "Questions cannot be empty", http.StatusBadRequest)
			return
		}
		if len(uiChoicesByQuestion[i]) == 0 {
			http.Error(w, "Questions must have at least one choice", http.StatusBadRequest)
			return
		}
		for _, choice := range uiChoicesByQuestion[i] {
			if choice == "" {
				http.Error(w, "Choices cannot be empty", http.StatusBadRequest)
				return
			}
		}
	}

	err = dbClient.EditModule(moduleId, title, description, uiQuestions, uiChoicesByQuestion)
	if err != nil {
		fmt.Println("Error editing module:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Println("Updated module")
	templates["edit_module.html"].ExecuteTemplate(w, "edited_module_response.html", nil)
}

func homePageHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	templates["index.html"].ExecuteTemplate(w, "page.html", nil)
}

func initRouter(dbClient *DbClient) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("/course/create", NewHandlerMap().Get(createCoursePageHandler).Post(withDbClient(dbClient, createCourseHandler)))
	mux.Handle("/course/{courseId}/edit", NewHandlerMap().Get(withDbClient(dbClient, editCoursePageHandler)).Put(withDbClient(dbClient, editCourseHandler)))
	mux.Handle("/ui/{element}", NewHandlerMap().Get(addElementHandler).Delete(deleteElementHandler))
	// This is kinda a weird place to put the deleteModuleHandler because it's on a different page
	// (the edit course page) but it's fine for now.
	mux.Handle("/course/{courseId}/module/{moduleId}/edit", NewHandlerMap().Get(withDbClient(dbClient, editModulePageHandler)).Put(withDbClient(dbClient, editModuleHandler)).Delete(withDbClient(dbClient, deleteModuleHandler)))
	mux.Handle("/course", NewHandlerMap().Get(withDbClient(dbClient, coursePageHandler)))
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	mux.Handle("/style/", http.StripPrefix("/style/", http.FileServer(http.Dir("style"))))
	mux.Handle("/", NewHandlerMap().Get(homePageHandler))
	return mux
}

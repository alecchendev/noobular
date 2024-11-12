package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

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

type Course struct {
	Id          int
	Title       string
	Description string
}

type Module struct {
	Id          int
	CourseId    int
	Title       string
	Description string
}

type DbClient struct {
	db *sql.DB
}

func NewDbClient(db *sql.DB) *DbClient {
	return &DbClient{db}
}

func (c *DbClient) CreateCourse(title string, description string, moduleTitles []string, moduleDescriptions []string) (Course, []Module, error) {
	if len(moduleTitles) != len(moduleDescriptions) {
		return Course{}, []Module{}, fmt.Errorf("moduleTitles and moduleDescriptions must have the same length")
	}
	res, err := c.db.Exec("insert into courses(title, description) values(?, ?)", title, description)
	if err != nil {
		return Course{}, []Module{}, err
	}
	courseId, err := res.LastInsertId()
	if err != nil {
		return Course{}, []Module{}, err
	}
	course := Course{int(courseId), title, description}
	modules := make([]Module, len(moduleTitles))
	for i := 0; i < len(moduleTitles); i++ {
		moduleTitle := moduleTitles[i]
		moduleDescription := moduleDescriptions[i]
		res, err = c.db.Exec("insert into modules(course_id, title, description) values(?, ?, ?)", courseId, moduleTitle, moduleDescription)
		if err != nil {
			return Course{}, []Module{}, err
		}
		moduleId, err := res.LastInsertId()
		if err != nil {
			return Course{}, []Module{}, err
		}
		module := Module{int(moduleId), course.Id, moduleTitle, moduleDescription}
		modules[i] = module
	}
	return course, modules, nil
}

func (c *DbClient) EditCourse(courseId int, title string, description string, moduleIds []int, moduleTitles []string, moduleDescriptions []string) (Course, []Module, error) {
	if len(moduleTitles) != len(moduleDescriptions) || len(moduleTitles) != len(moduleIds) {
		return Course{}, []Module{}, fmt.Errorf("moduleTitles, moduleDescriptions, and moduleIds must have the same length, got titles: %d, descs: %d, ids: %d", len(moduleTitles), len(moduleDescriptions), len(moduleIds))
	}
	res, err := c.db.Exec("update courses set title = ?, description = ? where id = ?;", title, description, courseId)
	if err != nil {
		return Course{}, []Module{}, err
	}
	course := Course{courseId, title, description}
	modules := make([]Module, len(moduleTitles))
	for i := 0; i < len(moduleTitles); i++ {
		moduleId := moduleIds[i]
		moduleTitle := moduleTitles[i]
		moduleDescription := moduleDescriptions[i]
		if moduleId == -1 {
			res, err = c.db.Exec("insert into modules(course_id, title, description) values(?, ?, ?)", courseId, moduleTitle, moduleDescription)
			if err != nil {
				return Course{}, []Module{}, err
			}
			moduleIdInt64, err := res.LastInsertId()
			if err != nil {
				return Course{}, []Module{}, err
			}
			moduleId = int(moduleIdInt64)
		} else {
			_, err = c.db.Exec("update modules set title = ?, description = ? where id = ?;", moduleTitle, moduleDescription, moduleId)
			if err != nil {
				return Course{}, []Module{}, err
			}
		}
		module := Module{moduleId, course.Id, moduleTitle, moduleDescription}
		modules[i] = module
	}
	return course, modules, nil
}

// Course struct for feeding into a template to be rendered
type UiCourse struct {
	Id          int
	Title       string
	Description string
	Modules     []UiModule
}

func EmptyCourse() UiCourse {
	return UiCourse{ -1, "", "", []UiModule{} }
}

type UiModule struct {
	Id          int
	// Now that I added this field it's the same as Module......
	CourseId    int
	Title       string
	Description string
}

func EmptyModule() UiModule {
	return UiModule{ -1, -1, "", "" }
}

func (m UiModule) ElementType() string {
	return "module"
}

func (m UiModule) ElementText() string {
	return m.Title
}

type GetCourseRow struct {
	CourseId    int
	CourseTitle string
	CourseDesc  string
	ModuleId    sql.NullInt64
	ModuleTitle sql.NullString
	ModuleDesc  sql.NullString
}

func (row GetCourseRow) NewCourse() UiCourse {
	return UiCourse{
		Id:          row.CourseId,
		Title:       row.CourseTitle,
		Description: row.CourseDesc,
		Modules:     []UiModule{},
	}
}

func (row GetCourseRow) NewModule() UiModule {
	return UiModule{
		Id:          int(row.ModuleId.Int64),
		CourseId:    row.CourseId,
		Title:       row.ModuleTitle.String,
		Description: row.ModuleDesc.String,
	}
}

const getCoursesQuery = `
select c.id, c.title, c.description, m.id, m.title, m.description
from courses c
left join modules m on c.id = m.course_id
order by c.id, m.id;
`

func (c *DbClient) GetCourses() ([]UiCourse, error) {
	rows, err := c.db.Query(getCoursesQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var courses []UiCourse
	for rows.Next() {
		var row GetCourseRow
		err := rows.Scan(&row.CourseId, &row.CourseTitle, &row.CourseDesc, &row.ModuleId, &row.ModuleTitle, &row.ModuleDesc)
		if err != nil {
			return nil, err
		}
		if len(courses) == 0 || courses[len(courses)-1].Id != row.CourseId {
			courses = append(courses, row.NewCourse())
		}
		if row.ModuleId.Valid {
			courses[len(courses)-1].Modules = append(courses[len(courses)-1].Modules, row.NewModule())
		}

	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return courses, nil
}

const getCourseQuery = `
select c.id, c.title, c.description, m.id, m.title, m.description
from courses c
left join modules m on c.id = m.course_id
where c.id = ?
order by m.id;
`

func (c *DbClient) GetCourse(courseId int) (UiCourse, error) {
	rows, err := c.db.Query(getCourseQuery, courseId)
	if err != nil {
		return UiCourse{}, err
	}
	defer rows.Close()
	var course UiCourse = UiCourse{}
	uninitialized := true
	for rows.Next() {
		var row GetCourseRow
		err := rows.Scan(&row.CourseId, &row.CourseTitle, &row.CourseDesc, &row.ModuleId, &row.ModuleTitle, &row.ModuleDesc)
		if err != nil {
			return UiCourse{}, err
		}
		if uninitialized {
			course = row.NewCourse()
			uninitialized = false
		}
		if row.ModuleId.Valid {
			course.Modules = append(course.Modules, row.NewModule())
		}
	}
	if err := rows.Err(); err != nil {
		return UiCourse{}, err
	}
	return course, nil
}

const getEditModuleQuery = `
select c.id, c.title, m.id, m.title, m.description, q.id, q.question_text, ch.id, ch.choice_text
from courses c
left join modules m on c.id = m.course_id
left join questions q on m.id = q.module_id
left join choices ch on q.id = ch.question_id
where c.id = ? and m.id = ?
order by c.id, m.id, q.id, ch.id;
`

type UiEditModule struct {
	CourseId    int
	CourseTitle string
	ModuleId    int
	ModuleTitle string
	ModuleDesc  string
	Questions   []UiQuestion
}

type UiQuestion struct {
	Id           int
	QuestionText string
	Choices      []UiChoice
}

func EmptyQuestion() UiQuestion {
	return UiQuestion{ -1, "", []UiChoice{} }
}

func (q UiQuestion) ElementType() string {
	return "question"
}

func (q UiQuestion) ElementText() string {
	return q.QuestionText
}

type UiChoice struct {
	Id         int
	ChoiceText string
}

func EmptyChoice() UiChoice {
	return UiChoice{ -1, "" }
}

func (c UiChoice) ElementType() string {
	return "choice"
}

func (c UiChoice) ElementText() string {
	return c.ChoiceText
}

func (c *DbClient) GetEditModule(courseId int, moduleId int) (UiEditModule, error) {
	rows, err := c.db.Query(getEditModuleQuery, courseId, moduleId)
	if err != nil {
		return UiEditModule{}, err
	}
	defer rows.Close()

	var uiModule UiEditModule
	firstRow := true
	for rows.Next() {
		var row struct {
			CourseId     int
			CourseTitle  string
			ModuleId     int
			ModuleTitle  string
			ModuleDesc   string
			QuestionId   sql.NullInt64
			QuestionText sql.NullString
			ChoiceId     sql.NullInt64
			ChoiceText   sql.NullString
		}
		err := rows.Scan(&row.CourseId, &row.CourseTitle, &row.ModuleId, &row.ModuleTitle, &row.ModuleDesc, &row.QuestionId, &row.QuestionText, &row.ChoiceId, &row.ChoiceText)
		if err != nil {
			return UiEditModule{}, err
		}
		if firstRow {
			uiModule.CourseId = row.CourseId
			uiModule.CourseTitle = row.CourseTitle
			uiModule.ModuleId = row.ModuleId
			uiModule.ModuleTitle = row.ModuleTitle
			uiModule.ModuleDesc = row.ModuleDesc
			uiModule.Questions = []UiQuestion{}
			firstRow = false
		}
		if row.QuestionId.Valid && (len(uiModule.Questions) == 0 || uiModule.Questions[len(uiModule.Questions)-1].Id != int(row.QuestionId.Int64)) {
			uiModule.Questions = append(uiModule.Questions, UiQuestion{
				Id:           int(row.QuestionId.Int64),
				QuestionText: row.QuestionText.String,
				Choices:      []UiChoice{},
			})
		}
		if row.ChoiceId.Valid {
			uiModule.Questions[len(uiModule.Questions)-1].Choices = append(uiModule.Questions[len(uiModule.Questions)-1].Choices, UiChoice{
				Id:         int(row.ChoiceId.Int64),
				ChoiceText: row.ChoiceText.String,
			})
		}
	}
	if err := rows.Err(); err != nil {
		return UiEditModule{}, err
	}
	return uiModule, nil
}

func (c *DbClient) EditModule(moduleId int, title string, description string, questions []string, choices [][]string) error {
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	_, err = tx.Exec("update modules set title = ?, description = ? where id = ?;", title, description, moduleId)
	if err != nil {
		tx.Rollback()
		return err
	}
	// Delete all questions and choices for this module (deleting quesitons cascades to choices)
	_, err = tx.Exec("delete from questions where module_id = ?;", moduleId)
	if err != nil {
		tx.Rollback()
		return err
	}
	for i, question := range questions {
		res, err := tx.Exec("insert into questions(module_id, question_text) values(?, ?);", moduleId, question)
		if err != nil {
			tx.Rollback()
			return err
		}
		questionId, err := res.LastInsertId()
		if err != nil {
			tx.Rollback()
			return err
		}
		for _, choice := range choices[i] {
			_, err = tx.Exec("insert into choices(question_id, choice_text) values(?, ?);", questionId, choice)
			if err != nil {
				tx.Rollback()
				return err
			}
		}
	}
	err = tx.Commit()
	if err != nil {
		return err
	}
	return nil
}

func (c *DbClient) DeleteModule(moduleId int) error {
	_, err := c.db.Exec("delete from modules where id = ?;", moduleId)
	if err != nil {
		return err
	}
	return nil
}

const createCourseTable = `
create table if not exists courses (
	id integer primary key autoincrement,
	title text not null,
	description text not null
);
`

const createModuleTable = `
create table if not exists modules (
	id integer primary key autoincrement,
	course_id integer not null,
	title text not null,
	description text not null,
	foreign key (course_id) references courses(id) on delete cascade
);
`

const createQuestionTable = `
create table if not exists questions (
	id integer primary key autoincrement,
	module_id integer not null,
	question_text text not null,
	foreign key (module_id) references modules(id) on delete cascade
);
`

const createChoiceTable = `
create table if not exists choices (
	id integer primary key autoincrement,
	question_id integer not null,
	choice_text text not null,
	foreign key (question_id) references questions(id) on delete cascade
);
`

func initDb(db *sql.DB) {
	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
	}
	stmts := []string{createCourseTable, createModuleTable, createQuestionTable, createChoiceTable}
	for _, stmt := range stmts {
		_, err := tx.Exec(stmt)
		if err != nil {
			tx.Rollback()
			log.Fatal(err)
		}
	}
	tx.Commit()
}

func main() {
	db, err := sql.Open("sqlite3", "test.db?_foreign_keys=on")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	initDb(db)

	dbClient := NewDbClient(db)
	router := initRouter(dbClient)
	server := http.Server{
		Addr:    ":8080",
		Handler: router,
	}
	fmt.Println("Listening on port 8080")
	log.Fatal(server.ListenAndServe())
}

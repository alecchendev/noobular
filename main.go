package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net/http"

	_ "github.com/mattn/go-sqlite3"
)

func initTemplates() map[string]*template.Template {
	return map[string]*template.Template{
		"index.html":   template.Must(template.ParseFiles("template/page.html", "template/index.html")),
		"courses.html": template.Must(template.ParseFiles("template/page.html", "template/courses.html")),
		"create_course.html": template.Must(template.ParseFiles(
			"template/page.html", "template/create_course.html", "template/add_module.html",
			"template/created_course_response.html")),
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

func createCoursePageHandler(w http.ResponseWriter, r *http.Request) {
	templates["create_course.html"].ExecuteTemplate(w, "page.html", nil)
}

// These two handlers seem kinda dumb, i.e. they could just be done in javascript,
// but I'm just going to do things the pure HTMX way for now to see how it goes.

// Simply returns another small chunk of HTML to add new modules
func addModuleHandler(w http.ResponseWriter, r *http.Request) {
	templates["create_course.html"].ExecuteTemplate(w, "add_module.html", nil)
}

func deleteModuleHandler(w http.ResponseWriter, r *http.Request) {
	// No op
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
	mux.Handle("/course/create/module", NewHandlerMap().Get(addModuleHandler).Delete(deleteModuleHandler))
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
	res, err := c.db.Exec("insert into courses(title, description) values(?, ?)", title, description)
	if err != nil {
		return Course{}, []Module{}, err
	}
	courseId, err := res.LastInsertId()
	if err != nil {
		return Course{}, []Module{}, err
	}
	if len(moduleTitles) != len(moduleDescriptions) {
		return Course{}, []Module{}, fmt.Errorf("moduleTitles and moduleDescriptions must have the same length")
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

// Course struct for feeding into a template to be rendered
type UiCourse struct {
	Id          int
	Title       string
	Description string
	Modules     []UiModule
}

type UiModule struct {
	Id          int
	Title       string
	Description string
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
		var row struct {
			CourseId    int
			CourseTitle string
			CourseDesc  string
			ModuleId    sql.NullInt64
			ModuleTitle sql.NullString
			ModuleDesc  sql.NullString
		}
		err := rows.Scan(&row.CourseId, &row.CourseTitle, &row.CourseDesc, &row.ModuleId, &row.ModuleTitle, &row.ModuleDesc)
		if err != nil {
			return nil, err
		}
		if len(courses) == 0 || courses[len(courses)-1].Id != row.CourseId {
			courses = append(courses, UiCourse{
				Id:          row.CourseId,
				Title:       row.CourseTitle,
				Description: row.CourseDesc,
				Modules:     []UiModule{},
			})
		}
		if row.ModuleId.Valid {
			courses[len(courses)-1].Modules = append(courses[len(courses)-1].Modules, UiModule{
				Id:          int(row.ModuleId.Int64),
				Title:       row.ModuleTitle.String,
				Description: row.ModuleDesc.String,
			})
		}

	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return courses, nil
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
	db, err := sql.Open("sqlite3", "test.db")
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

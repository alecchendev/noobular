package main

import (
	// "fmt"
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"regexp"

	_ "github.com/mattn/go-sqlite3"
)

var resp_tmpl = `
<div id="response-message">
	<p>Course created successfully</p>
</div>
`

var templates = template.Must(template.Must(template.ParseFiles("edit.html", "view.html")).New("created_course_response.html").Parse(resp_tmpl))

var validPath = regexp.MustCompile("^/(edit|save|view)/([a-zA-Z0-9]+)$")

type Page struct {
	Title string
	Body  []byte
}

func (p *Page) save() error {
	filename := p.Title + ".txt"
	return os.WriteFile(filename, p.Body, 0600)
}

func loadPage(title string) (*Page, error) {
	filename := title + ".txt"
	body, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return &Page{Title: title, Body: body}, nil
}

func makeHandler(fn func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m := validPath.FindStringSubmatch(r.URL.Path)
		if m == nil {
			http.NotFound(w, r)
			return
		}
		fn(w, r, m[2])
	}
}

func renderTemplate(w http.ResponseWriter, tmpl string, p *Page) {
	err := templates.ExecuteTemplate(w, tmpl+".html", p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func viewHandler(w http.ResponseWriter, r *http.Request, title string) {
	p, err := loadPage(title)
	if err != nil {
		http.Redirect(w, r, "/edit/"+title, http.StatusFound)
		return
	}
	renderTemplate(w, "view", p)
}

func editHandler(w http.ResponseWriter, r *http.Request, title string) {
	p, err := loadPage(title)
	if err != nil {
		p = &Page{Title: title}
	}
	renderTemplate(w, "edit", p)
}

func saveHandler(w http.ResponseWriter, r *http.Request, title string) {
	body := r.FormValue("body")
	p := &Page{Title: title, Body: []byte(body)}
	err := p.save()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/view/"+title, http.StatusFound)
}

type HandlerMap map[string]func(http.ResponseWriter, *http.Request)

func (hm HandlerMap) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if handler, ok := hm[r.Method]; ok {
		handler(w, r)
		return
	}
	http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
}

func NewHandlerMap() HandlerMap {
	return HandlerMap{}
}

func (hm HandlerMap) Get(handler func(http.ResponseWriter, *http.Request)) HandlerMap {
	hm["GET"] = handler
	return hm
}

func (hm HandlerMap) Post(handler func(http.ResponseWriter, *http.Request)) HandlerMap {
	hm["POST"] = handler
	return hm
}

func withDbClient(dbClient *DbClient, handler func(http.ResponseWriter, *http.Request, *DbClient)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		handler(w, r, dbClient)
	}
}

func createCourseHandler(w http.ResponseWriter, r *http.Request, dbClient *DbClient) {
	fmt.Println("Creating course")
	var course Course
	err := json.NewDecoder(r.Body).Decode(&course)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	fmt.Println(course)
	dbClient.CreateCourse(course)
	templates.ExecuteTemplate(w, "created_course_response.html", nil)
}

func createCourseFormHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "create_course.html")
}

func homePageHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, "index.html")
}

func runServer(dbClient *DbClient) {
	http.HandleFunc("/view/", makeHandler(viewHandler))
	http.HandleFunc("/edit/", makeHandler(editHandler))
	http.HandleFunc("/save/", makeHandler(saveHandler))
	http.Handle("/course/create", NewHandlerMap().Get(createCourseFormHandler).Post(withDbClient(dbClient, createCourseHandler)))
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	http.Handle("/", NewHandlerMap().Get(homePageHandler))
	log.Fatal(http.ListenAndServe(":8080", nil))
}

type Course struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type DbClient struct {
	db *sql.DB
}

func NewDbClient(db *sql.DB) *DbClient {
	return &DbClient{db}
}

func (c *DbClient) CreateCourse(course Course) error {
	// exampleCourses := []Course{{"Cryptography", "Intro to cryptographic primitives"}, {"Abstract algebra", "Intro to abstract algebra"}}

	_, err := c.db.Exec("insert into courses(title, description) values(?, ?)", course.Title, course.Description)
	if err != nil {
		return err
	}
	return nil
}

func initDb(db *sql.DB) {
	sqlStmt := `
	create table courses (title text, description text);
	delete from courses;
	`
	_, err := db.Exec(sqlStmt)
	if err != nil {
		log.Printf("Error creating courses table: %q: %s\n", err, sqlStmt)
		return
	}
}

func main() {
	os.Remove("test.db")
	db, err := sql.Open("sqlite3", "test.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	initDb(db)

	dbClient := NewDbClient(db)
	runServer(dbClient)
}

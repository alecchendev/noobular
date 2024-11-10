package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"

	_ "github.com/mattn/go-sqlite3"
)

var templates = template.Must(template.ParseFiles("template/index.html", "template/courses.html", "template/create_course.html", "template/created_course_response.html"))

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

func coursePageHandler(w http.ResponseWriter, r *http.Request, dbClient *DbClient) {
	courses, err := dbClient.GetCourses()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	templates.ExecuteTemplate(w, "courses.html", courses)
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
	templates.ExecuteTemplate(w, "create_course.html", nil)
}

func homePageHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	templates.ExecuteTemplate(w, "index.html", nil)
}

func runServer(dbClient *DbClient) {
	http.Handle("/course/create", NewHandlerMap().Get(createCourseFormHandler).Post(withDbClient(dbClient, createCourseHandler)))
	http.Handle("/course", NewHandlerMap().Get(withDbClient(dbClient, coursePageHandler)))
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

func (c *DbClient) GetCourses() ([]Course, error) {
	rows, err := c.db.Query("select title, description from courses")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var courses []Course
	for rows.Next() {
		var course Course
		err := rows.Scan(&course.Title, &course.Description)
		if err != nil {
			return nil, err
		}
		courses = append(courses, course)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return courses, nil
}


func initDb(db *sql.DB) {
	sqlStmt := `
	create table if not exists courses (title text, description text);
	`
	_, err := db.Exec(sqlStmt)
	if err != nil {
		log.Printf("Error creating courses table: %q: %s\n", err, sqlStmt)
		return
	}
}

func main() {
	db, err := sql.Open("sqlite3", "test.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	initDb(db)

	dbClient := NewDbClient(db)
	runServer(dbClient)
}

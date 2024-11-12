package internal

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

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

func NewDbClient() *DbClient {
	db, err := sql.Open("sqlite3", "test.db?_foreign_keys=on")
	if err != nil {
		log.Fatal(err)
	}
	initDb(db)
	return &DbClient{db}
}

func (c *DbClient) Close() {
	c.db.Close()
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
	return UiCourse{-1, "", "", []UiModule{}}
}

type UiModule struct {
	Id int
	// Now that I added this field it's the same as Module......
	CourseId    int
	Title       string
	Description string
}

func EmptyModule() UiModule {
	return UiModule{-1, -1, "", ""}
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
	return UiQuestion{-1, "", []UiChoice{}}
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
	return UiChoice{-1, ""}
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

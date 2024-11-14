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

const insertCourseQuery = `
insert into courses(title, description)
values(?, ?);
`

const insertModuleQuery = `
insert into modules(course_id, title, description)
values(?, ?, ?);
`

func (c *DbClient) CreateCourse(title string, description string, moduleTitles []string, moduleDescriptions []string) (Course, []Module, error) {
	if len(moduleTitles) != len(moduleDescriptions) {
		return Course{}, []Module{}, fmt.Errorf("moduleTitles and moduleDescriptions must have the same length")
	}
	res, err := c.db.Exec(insertCourseQuery, title, description)
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
		res, err = c.db.Exec(insertModuleQuery, courseId, moduleTitle, moduleDescription)
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

const updateCourseQuery = `
update courses
set title = ?, description = ?
where id = ?;
`

func (c *DbClient) EditCourse(courseId int, title string, description string, moduleIds []int, moduleTitles []string, moduleDescriptions []string) (Course, []Module, error) {
	if len(moduleTitles) != len(moduleDescriptions) || len(moduleTitles) != len(moduleIds) {
		return Course{}, []Module{}, fmt.Errorf("moduleTitles, moduleDescriptions, and moduleIds must have the same length, got titles: %d, descs: %d, ids: %d", len(moduleTitles), len(moduleDescriptions), len(moduleIds))
	}
	res, err := c.db.Exec(updateCourseQuery, title, description, courseId)
	if err != nil {
		return Course{}, []Module{}, err
	}
	course := Course{courseId, title, description}
	modules := make([]Module, len(moduleTitles))
	for i := 0; i < len(moduleTitles); i++ {
		moduleId := moduleIds[i]
		moduleTitle := moduleTitles[i]
		moduleDescription := moduleDescriptions[i]
		// -1 means this is a new module
		if moduleId == -1 {
			res, err = c.db.Exec(insertModuleQuery, courseId, moduleTitle, moduleDescription)
			if err != nil {
				return Course{}, []Module{}, err
			}
			moduleIdInt64, err := res.LastInsertId()
			if err != nil {
				return Course{}, []Module{}, err
			}
			moduleId = int(moduleIdInt64)
		} else {
			_, err = c.db.Exec(updateModuleQuery, moduleTitle, moduleDescription, moduleId)
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

func (m UiModule) IsEmpty() bool {
	return m.Id == -1
}

const getCourseQuery = `
select c.id, c.title, c.description
from courses c
where c.id = ?;
`

const getCoursesQuery = `
select c.id, c.title, c.description
from courses c
order by c.id;
`

const getModulesQuery = `
select m.id, m.course_id, m.title, m.description
from modules m
where m.course_id = ?
order by m.id;
`

func (c *DbClient) GetCourses() ([]UiCourse, error) {
	courseRows, err := c.db.Query(getCoursesQuery)
	if err != nil {
		return nil, err
	}
	defer courseRows.Close()

	var courses []UiCourse
	for courseRows.Next() {
		var course UiCourse
		err := courseRows.Scan(&course.Id, &course.Title, &course.Description)
		if err != nil {
			return nil, err
		}

		moduleRows, err := c.db.Query(getModulesQuery, course.Id)
		if err != nil {
			return nil, err
		}
		defer moduleRows.Close()
		for moduleRows.Next() {
			var module UiModule
			err := moduleRows.Scan(&module.Id, &module.CourseId, &module.Title, &module.Description)
			if err != nil {
				return nil, err
			}
			course.Modules = append(course.Modules, module)
		}
		if err := moduleRows.Err(); err != nil {
			return nil, err
		}

		courses = append(courses, course)
	}
	if err := courseRows.Err(); err != nil {
		return nil, err
	}
	return courses, nil
}

func (c *DbClient) GetCourse(courseId int) (UiCourse, error) {
	row := c.db.QueryRow(getCourseQuery, courseId)
	var course UiCourse
	err := row.Scan(&course.Id, &course.Title, &course.Description)
	if err != nil {
		return UiCourse{}, err
	}
	rows, err := c.db.Query(getModulesQuery, courseId)
	if err != nil {
		return UiCourse{}, err
	}
	for rows.Next() {
		var module UiModule
		err := rows.Scan(&module.Id, &module.CourseId, &module.Title, &module.Description)
		if err != nil {
			return UiCourse{}, err
		}
		course.Modules = append(course.Modules, module)
	}
	if err := rows.Err(); err != nil {
		return UiCourse{}, err
	}
	return course, nil
}

const getModuleQuery = `
select m.id, m.course_id, m.title, m.description
from modules m
where m.id = ?
`

const getQuestionsQuery = `
select q.id, q.question_text
from questions q
where q.module_id = ?
order by q.id;
`

const getChoicesQuery = `
select ch.id, ch.choice_text
from choices ch
where ch.question_id = ?
order by ch.id;
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

func (q UiQuestion) IsEmpty() bool {
	return q.Id == -1
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

func (c UiChoice) IsEmpty() bool {
	return c.Id == -1
}

func (c *DbClient) GetEditModule(courseId int, moduleId int) (UiEditModule, error) {
	courseRow := c.db.QueryRow(getCourseQuery, courseId)
	var module UiEditModule
	var courseDescription string // stub
	err := courseRow.Scan(&module.CourseId, &module.CourseTitle, &courseDescription)
	if err != nil {
		return UiEditModule{}, err
	}
	moduleRow := c.db.QueryRow(getModuleQuery, moduleId)
	err = moduleRow.Scan(&module.ModuleId, &module.CourseId, &module.ModuleTitle, &module.ModuleDesc)
	if err != nil {
		return UiEditModule{}, err
	}

	questionRows, err := c.db.Query(getQuestionsQuery, moduleId)
	if err != nil {
		return UiEditModule{}, err
	}
	defer questionRows.Close()
	for questionRows.Next() {
		var question UiQuestion
		err := questionRows.Scan(&question.Id, &question.QuestionText)
		if err != nil {
			return UiEditModule{}, err
		}

		choiceRows, err := c.db.Query(getChoicesQuery, question.Id)
		if err != nil {
			return UiEditModule{}, err
		}
		defer choiceRows.Close()
		for choiceRows.Next() {
			var choice UiChoice
			err := choiceRows.Scan(&choice.Id, &choice.ChoiceText)
			if err != nil {
				return UiEditModule{}, err
			}
			question.Choices = append(question.Choices, choice)
		}
		if err := choiceRows.Err(); err != nil {
			return UiEditModule{}, err
		}
		module.Questions = append(module.Questions, question)
	}
	if err := questionRows.Err(); err != nil {
		return UiEditModule{}, err
	}
	return module, nil
}

const updateModuleQuery = `
update modules
set title = ?, description = ?
where id = ?;
`

const deleteQuestionsQuery = `
delete from questions
where module_id = ?;
`

const insertQuestionQuery = `
insert into questions(module_id, question_text)
values(?, ?);
`

const insertChoiceQuery = `
insert into choices(question_id, choice_text)
values(?, ?);
`

func (c *DbClient) EditModule(moduleId int, title string, description string, questions []string, choices [][]string) error {
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	_, err = tx.Exec(updateModuleQuery, title, description, moduleId)
	if err != nil {
		tx.Rollback()
		return err
	}
	// Delete all questions and choices for this module (deleting quesitons cascades to choices)
	_, err = tx.Exec(deleteQuestionsQuery, moduleId)
	if err != nil {
		tx.Rollback()
		return err
	}
	for i, question := range questions {
		res, err := tx.Exec(insertQuestionQuery, moduleId, question)
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
			_, err = tx.Exec(insertChoiceQuery, questionId, choice)
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

const getQuestionCountQuery = `
select count(*)
from questions q
where q.module_id = ?;
`

const getQuestionQuery = `
select q.id, q.question_text
from questions q
where q.module_id = ?
limit 1 offset ?;
`

func (c *DbClient) GetModuleQuestion(moduleId int, questionIdx int) (UiModule, UiQuestion, int, error) {
	moduleRow := c.db.QueryRow(getModuleQuery, moduleId);
	var module UiModule
	err := moduleRow.Scan(&module.Id, &module.CourseId, &module.Title, &module.Description)
	if err != nil {
		return UiModule{}, UiQuestion{}, 0, err
	}

	questionCountRow := c.db.QueryRow(getQuestionCountQuery, moduleId)
	var questionCount int
	err = questionCountRow.Scan(&questionCount)
	if err != nil {
		return UiModule{}, UiQuestion{}, 0, err
	}
	if questionIdx >= questionCount {
		return UiModule{}, UiQuestion{}, 0, fmt.Errorf("question index out of bounds")
	}

	questionRow := c.db.QueryRow(getQuestionQuery, moduleId, questionIdx)
	var question UiQuestion
	err = questionRow.Scan(&question.Id, &question.QuestionText)
	if err != nil {
		return UiModule{}, UiQuestion{}, 0, err
	}

	choiceRows, err := c.db.Query(getChoicesQuery, question.Id)
	if err != nil {
		return UiModule{}, UiQuestion{}, 0, err
	}
	defer choiceRows.Close()
	for choiceRows.Next() {
		var choice UiChoice
		err := choiceRows.Scan(&choice.Id, &choice.ChoiceText)
		if err != nil {
			return UiModule{}, UiQuestion{}, 0, err
		}
		question.Choices = append(question.Choices, choice)
	}
	if err := choiceRows.Err(); err != nil {
		return UiModule{}, UiQuestion{}, 0, err
	}

	return module, question, questionCount, nil
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

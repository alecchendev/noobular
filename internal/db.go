package internal

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand/v2"

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

type UiCourseStudent struct {
	Id          int
	Title       string
	Description string
	Modules     []UiModuleStudent
}

type UiModule struct {
	Id                        int
	CourseId                  int
	Title                     string
	Description               string
}

func NewUiModule(m Module) UiModule {
	return UiModule{m.Id, m.CourseId, m.Title, m.Description}
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

type UiModuleStudent struct {
	Id                        int
	CourseId                  int
	Title                     string
	Description               string
	QuestionCount             int
	NextUnansweredQuestionIdx int
}

const getModulesQuery = `
select m.id, m.course_id, m.title, m.description
from modules m
where m.course_id = ?
order by m.id;
`

const getModulesWithQuestionsQuery = `
select distinct m.id, m.course_id, m.title, m.description
from modules m
join questions q on m.id = q.module_id
where m.course_id = ?
order by m.id;
`

func (c *DbClient) GetModules(courseId int, requireHasQuestions bool) ([]Module, error) {
	var query string
	if requireHasQuestions {
		query = getModulesWithQuestionsQuery
	} else {
		query = getModulesQuery
	}
	moduleRows, err := c.db.Query(query, courseId)
	if err != nil {
		return nil, err
	}
	defer moduleRows.Close()
	modules := []Module{}
	for moduleRows.Next() {
		var module Module
		err := moduleRows.Scan(&module.Id, &module.CourseId, &module.Title, &module.Description)
		if err != nil {
			return nil, err
		}
		modules = append(modules, module)
	}
	if err := moduleRows.Err(); err != nil {
		return nil, err
	}
	return modules, nil
}

const getCoursesQuery = `
select c.id, c.title, c.description
from courses c
order by c.id;
`

const getCoursesWithModulesWithQuestionsQuery = `
select distinct c.id, c.title, c.description
from courses c
join modules m on c.id = m.course_id
join questions q on m.id = q.module_id
order by c.id;
`

func (c *DbClient) GetCourses(forStudent bool) ([]Course, error) {
	var query string
	if forStudent {
		query = getCoursesWithModulesWithQuestionsQuery
	} else {
		query = getCoursesQuery
	}
	courseRows, err := c.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer courseRows.Close()

	var courses []Course
	for courseRows.Next() {
		var course Course
		err := courseRows.Scan(&course.Id, &course.Title, &course.Description)
		if err != nil {
			return nil, err
		}
		courses = append(courses, course)
	}
	if err := courseRows.Err(); err != nil {
		return nil, err
	}
	return courses, nil
}

const getCourseQuery = `
select c.id, c.title, c.description
from courses c
where c.id = ?;
`

func (c *DbClient) GetCourse(courseId int) (Course, error) {
	row := c.db.QueryRow(getCourseQuery, courseId)
	var course Course
	err := row.Scan(&course.Id, &course.Title, &course.Description)
	if err != nil {
		return Course{}, err
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
select ch.id, ch.choice_text, ch.correct
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
	Id int
	// This is a random integer created to differentiate questions in the UI.
	Idx          int
	QuestionText string
	Choices      []UiChoice
	Explanation  string
}

func EmptyQuestion() UiQuestion {
	return UiQuestion{-1, rand.Int(), "", []UiChoice{}, ""}
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
	Id int
	// This is a random integer created to differentiate questions in the UI.
	QuestionIdx int
	/// A random idx just to differentiate choices in the UI
	/// so that label elements can be associated with certain choices.
	Idx        int
	ChoiceText string
	IsCorrect  bool
}

func EmptyChoice(questionId int) UiChoice {
	return UiChoice{-1, questionId, rand.Int(), "", false}
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
		question := EmptyQuestion()
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
			choice := EmptyChoice(question.Idx)
			err := choiceRows.Scan(&choice.Id, &choice.ChoiceText, &choice.IsCorrect)
			if err != nil {
				return UiEditModule{}, err
			}
			question.Choices = append(question.Choices, choice)
		}
		if err := choiceRows.Err(); err != nil {
			return UiEditModule{}, err
		}

		explanationRow := c.db.QueryRow(getExplanationContentQuery, question.Id)
		var contentId int64
		var content string
		err = explanationRow.Scan(&contentId, &content)
		if err != nil && err != sql.ErrNoRows {
			return UiEditModule{}, err
		}
		if err == nil {
			question.Explanation = content
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
insert into choices(question_id, choice_text, correct)
values(?, ?, ?);
`

const getExplanationContentQuery = `
select c.id, c.content
from explanations e
join content c on e.content_id = c.id
where e.question_id = ?;
`

const insertContentQuery = `
insert into content(content)
values(?);
`

const updateContentQuery = `
update content
set content = ?
where id = ?;
`

const insertExplanationQuery = `
insert into explanations(question_id, content_id)
values(?, ?);
`

func (c *DbClient) EditModule(moduleId int, title string, description string, questions []string, choices [][]string, correctChoiceIdxs []int, explanations []string) error {
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
		for choiceIdx, choice := range choices[i] {
			_, err = tx.Exec(insertChoiceQuery, questionId, choice, choiceIdx == correctChoiceIdxs[i])
			if err != nil {
				tx.Rollback()
				return err
			}
		}
		explanation := tx.QueryRow(getExplanationContentQuery, questionId)
		var contentId int64
		var content string
		err = explanation.Scan(&contentId, &content)
		if err != nil && err != sql.ErrNoRows {
			tx.Rollback()
			return err
		} else if err == sql.ErrNoRows && explanations[i] != "" {
			res, err := tx.Exec(insertContentQuery, explanations[i])
			if err != nil {
				log.Println("Error getting explanation:", err)
				tx.Rollback()
				return err
			}
			contentId, err = res.LastInsertId()
			if err != nil {
				log.Println("Error getting explanation2:", err)
				tx.Rollback()
				return err
			}
			log.Println("got here", contentId)
			_, err = tx.Exec(insertExplanationQuery, questionId, contentId)
			if err != nil {
				log.Println("Error getting explanation3:", err)
				tx.Rollback()
				return err
			}
		} else {
			log.Println("got here2")
			_, err = tx.Exec(updateContentQuery, explanations[i], contentId)
			if err != nil {
				log.Println("Error getting explanation3:", err)
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

const getQuestionQuery = `
select q.id, q.question_text
from questions q
where q.module_id = ?
limit 1 offset ?;
`

func (c *DbClient) GetModuleQuestion(moduleId int, questionIdx int) (UiModule, UiQuestion, error) {
	moduleRow := c.db.QueryRow(getModuleQuery, moduleId)
	var module UiModule
	err := moduleRow.Scan(&module.Id, &module.CourseId, &module.Title, &module.Description)
	if err != nil {
		return UiModule{}, UiQuestion{}, err
	}

	questionRow := c.db.QueryRow(getQuestionQuery, moduleId, questionIdx)
	question := EmptyQuestion()
	err = questionRow.Scan(&question.Id, &question.QuestionText)
	if err != nil {
		return UiModule{}, UiQuestion{}, err
	}

	choiceRows, err := c.db.Query(getChoicesQuery, question.Id)
	if err != nil {
		return UiModule{}, UiQuestion{}, err
	}
	defer choiceRows.Close()
	for choiceRows.Next() {
		choice := EmptyChoice(question.Idx)
		err := choiceRows.Scan(&choice.Id, &choice.ChoiceText, &choice.IsCorrect)
		if err != nil {
			return UiModule{}, UiQuestion{}, err
		}
		question.Choices = append(question.Choices, choice)
	}
	if err := choiceRows.Err(); err != nil {
		return UiModule{}, UiQuestion{}, err
	}

	explanationRow := c.db.QueryRow(getExplanationContentQuery, question.Id)
	var contentId int64
	var content string
	err = explanationRow.Scan(&contentId, &content)
	if err != nil && err != sql.ErrNoRows {
		return UiModule{}, UiQuestion{}, err
	}
	if err == nil {
		question.Explanation = content
	}

	return module, question, nil
}

func (c *DbClient) DeleteModule(moduleId int) error {
	_, err := c.db.Exec("delete from modules where id = ?;", moduleId)
	if err != nil {
		return err
	}
	return nil
}

const storeAnswerQuery = `
update answers
set choice_id = ?
where question_id = ?;

insert into answers(question_id, choice_id)
select ?, ?
where not exists (select 1 from answers where question_id = ?);
`

func (c *DbClient) StoreAnswer(questionId int, choiceId int) error {
	_, err := c.db.Exec(storeAnswerQuery, choiceId, questionId, questionId, choiceId, questionId)
	return err
}

const getAnswerQuery = `
select a.choice_id
from answers a
where a.question_id = ?;
`

// Returns the choice id of the answer for the question if it exists.
// Returns -1 if there is no answer for the question.
func (c *DbClient) GetAnswer(questionId int) (int, error) {
	row := c.db.QueryRow(getAnswerQuery, questionId)
	var choiceId int
	err := row.Scan(&choiceId)
	if err == sql.ErrNoRows {
		return -1, nil
	}
	if err != nil {
		return 0, err
	}
	return choiceId, nil
}

const getQuestionCountQuery = `
select count(*)
from questions q
where q.module_id = ?;
`

func (c *DbClient) GetQuestionCount(moduleId int) (int, error) {
	row := c.db.QueryRow(getQuestionCountQuery, moduleId)
	var questionCount int
	err := row.Scan(&questionCount)
	if err != nil {
		return 0, err
	}
	return questionCount, nil
}

func (c *DbClient) GetNextUnansweredQuestionIdx(moduleId int) (int, error) {
	rows, err := c.db.Query(getQuestionsQuery, moduleId)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	questionIdx := 0
	var questionId int      // not used
	var questionText string // not used
	for rows.Next() {
		err := rows.Scan(&questionId, &questionText)
		if err != nil {
			return 0, err
		}
		answer, err := c.GetAnswer(questionId)
		if err != nil {
			return 0, err
		}
		if answer == -1 {
			return questionIdx, nil
		}
		questionIdx += 1
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	return questionIdx, nil
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
	correct bool not null,
	foreign key (question_id) references questions(id) on delete cascade
);
`

const createAnswerTable = `
create table if not exists answers (
	id integer primary key autoincrement,
	question_id integer not null,
	choice_id integer not null,
	foreign key (question_id) references questions(id) on delete cascade
);
`

const createContentTable = `
create table if not exists content (
	id integer primary key autoincrement,
	content text not null
);
`

const createExplanationTable = `
create table if not exists explanations (
	id integer primary key autoincrement,
	question_id integer not null,
	content_id integer not null,
	foreign key (question_id) references questions(id) on delete cascade,
	foreign key (content_id) references content(id) on delete cascade
);
`

func initDb(db *sql.DB) {
	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
	}
	stmts := []string{
		createCourseTable,
		createModuleTable,
		createQuestionTable,
		createChoiceTable,
		createAnswerTable,
		createContentTable,
		createExplanationTable,
	}
	for _, stmt := range stmts {
		_, err := tx.Exec(stmt)
		if err != nil {
			tx.Rollback()
			log.Fatal(err)
		}
	}
	tx.Commit()
}

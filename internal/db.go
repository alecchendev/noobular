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

const insertUserQuery = `
insert into users(username)
values(?);
`

func (c *DbClient) CreateUser(username string) (int64, error) {
	res, err := c.db.Exec(insertUserQuery, username)
	if err != nil {
		return 0, err
	}
	userId, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return userId, nil
}

type User struct {
	Id       int
	Username string
}

func (c *DbClient) GetUser(userId int64) (User, error) {
	row := c.db.QueryRow("select id, username from users where id = ?;", userId)
	var user User
	err := row.Scan(&user.Id, &user.Username)
	if err != nil {
		return User{}, err
	}
	return user, nil
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

const getModulesQuery = `
select m.id, m.course_id, m.title, m.description
from modules m
where m.course_id = ?
order by m.id;
`

const getModulesWithBlocksQuery = `
select distinct m.id, m.course_id, m.title, m.description
from modules m
join blocks b on m.id = b.module_id
where m.course_id = ?
order by m.id;
`

func (c *DbClient) GetModules(courseId int, requireHasBlocks bool) ([]Module, error) {
	var query string
	if requireHasBlocks {
		query = getModulesWithBlocksQuery
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

const getCoursesWithModulesWithBlocksQuery = `
select distinct c.id, c.title, c.description
from courses c
join modules m on c.id = m.course_id
join blocks b on m.id = b.module_id
order by c.id;
`

func (c *DbClient) GetCourses(forStudent bool) ([]Course, error) {
	var query string
	if forStudent {
		query = getCoursesWithModulesWithBlocksQuery
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

const deleteCourseQuery = `
delete from courses
where id = ?;
`

func (c *DbClient) DeleteCourse(courseId int) error {
	tx, err := c.db.Begin()
	modules, err := c.GetModules(courseId, false)
	for _, module := range modules {
		_, err = tx.Exec(deleteContentForModuleQuery, module.Id)
		if err != nil {
			tx.Rollback()
			return err
		}
	}
	_, err = tx.Exec(deleteCourseQuery, courseId)
	if err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

const getModuleQuery = `
select m.id, m.course_id, m.title, m.description
from modules m
where m.id = ?
`

const getContentForBlockQuery = `
select c.id, c.content
from content c
join content_blocks cb on c.id = cb.content_id
where cb.block_id = ?;
`

const getQuestionsQuery = `
select q.id, q.question_text
from questions q
join blocks b on q.block_id = b.id
where b.module_id = ?
order by q.id;
`

const getBlocksQuery = `
select b.id, b.module_id, b.block_index, b.block_type
from blocks b
where b.module_id = ?
order by b.block_index;
`

func (c *DbClient) GetBlocks(moduleId int) ([]Block, error) {
	blockRows, err := c.db.Query(getBlocksQuery, moduleId)
	if err != nil {
		return nil, err
	}
	defer blockRows.Close()
	blocks := []Block{}
	for blockRows.Next() {
		var block Block
		err := blockRows.Scan(&block.Id, &block.ModuleId, &block.BlockIndex, &block.BlockType)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, block)
	}
	if err := blockRows.Err(); err != nil {
		return nil, err
	}
	return blocks, nil
}

const updateModuleQuery = `
update modules
set title = ?, description = ?
where id = ?;
`

const insertBlockQuery = `
insert into blocks(module_id, block_index, block_type)
values(?, ?, ?);
`

const deleteContentForModuleQuery = `
with module_block_ids as (
	select id from blocks where module_id = ?
)
delete from content
where id in (
    select content_id from content_blocks where block_id in module_block_ids
)
or id in (
    select content_id from explanations where question_id in (
        select id from questions where block_id in module_block_ids
    )
);
`

const deleteBlocksQuery = `
delete from blocks
where module_id = ?;
`

const insertQuestionQuery = `
insert into questions(block_id, question_text)
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

func UpdateModuleMetadata(tx *sql.Tx, moduleId int, title string, description string) error {
	_, err := tx.Exec(updateModuleQuery, title, description, moduleId)
	return err
}

func DeleteContentForModule(tx *sql.Tx, moduleId int) error {
	_, err := tx.Exec(deleteContentForModuleQuery, moduleId)
	if err != nil {
		return err
	}
	_, err = tx.Exec(deleteBlocksQuery, moduleId)
	return err
}

func (c *DbClient) EditModule(moduleId int, title string, description string, blockTypes []string, contents []string, questions []string, choices [][]string, correctChoiceIdxs []int, explanations []string) error {
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	err = UpdateModuleMetadata(tx, moduleId, title, description)
	if err != nil {
		tx.Rollback()
		return err
	}
	// Delete all content pieces, and questions and choices for this module (deleting questions cascades to choices)
	err = DeleteContentForModule(tx, moduleId)
	if err != nil {
		tx.Rollback()
		return err
	}
	questionIdx := 0
	contentIdx := 0
	for i, blockType := range blockTypes {
		res, err := tx.Exec(insertBlockQuery, moduleId, i, blockType)
		if err != nil {
			tx.Rollback()
			return err
		}
		blockId, err := res.LastInsertId()
		if blockType == string(ContentBlockType) {
			err = c.InsertContentBlock(tx, blockId, contents[contentIdx])
			if err != nil {
				tx.Rollback()
				return err
			}
			contentIdx += 1
		} else if blockType == string(QuestionBlockType) {
			err = c.InsertQuestion(tx, blockId, questions[questionIdx], choices[questionIdx], correctChoiceIdxs[questionIdx], explanations[questionIdx])
			if err != nil {
				tx.Rollback()
				return err
			}
			questionIdx += 1
		} else {
			return fmt.Errorf("invalid block type: %s", blockType)
		}
	}
	err = tx.Commit()
	if err != nil {
		return err
	}
	return nil
}

const insertContentBlockQuery = `
insert into content_blocks(block_id, content_id)
values(?, ?);
`

func (c *DbClient) InsertContentBlock(tx *sql.Tx, blockId int64, content string) error {
	res, err := tx.Exec(insertContentQuery, content)
	if err != nil {
		return err
	}
	contentId, err := res.LastInsertId()
	if err != nil {
		return err
	}
	_, err = tx.Exec(insertContentBlockQuery, blockId, contentId)
	if err != nil {
		return err
	}
	return nil
}

// Need to rollback tx upon error one level up the stack because this function will not do that.
func (c *DbClient) InsertQuestion(tx *sql.Tx, blockId int64, question string, choices []string, correctChoiceIdx int, explanation string) error {
	res, err := tx.Exec(insertQuestionQuery, blockId, question)
	if err != nil {
		return err
	}
	questionId, err := res.LastInsertId()
	if err != nil {
		return err
	}
	for choiceIdx, choice := range choices {
		_, err = tx.Exec(insertChoiceQuery, questionId, choice, choiceIdx == correctChoiceIdx)
		if err != nil {
			return err
		}
	}
	existingExplanation := tx.QueryRow(getExplanationContentQuery, questionId)
	var contentId int64
	var content string
	err = existingExplanation.Scan(&contentId, &content)
	if err != nil && err != sql.ErrNoRows {
		return err
	} else if err == sql.ErrNoRows && explanation != "" {
		res, err := tx.Exec(insertContentQuery, explanation)
		if err != nil {
			return err
		}
		contentId, err = res.LastInsertId()
		if err != nil {
			return err
		}
		_, err = tx.Exec(insertExplanationQuery, questionId, contentId)
		if err != nil {
			return err
		}
	} else {
		_, err = tx.Exec(updateContentQuery, explanation, contentId)
		if err != nil {
			return err
		}
	}
	return nil
}

const getBlockQuery = `
select b.id, b.module_id, b.block_index, b.block_type
from blocks b
where b.block_index = ?
and b.module_id = ?;
`

func (c *DbClient) GetModule(moduleId int) (Module, error) {
	moduleRow := c.db.QueryRow(getModuleQuery, moduleId)
	var module Module
	err := moduleRow.Scan(&module.Id, &module.CourseId, &module.Title, &module.Description)
	if err != nil {
		return Module{}, err
	}
	return module, nil
}

type BlockType string

const (
	QuestionBlockType BlockType = "question"
	ContentBlockType  BlockType = "content"
)

type Block struct {
	Id         int
	ModuleId   int
	BlockIndex int
	BlockType  BlockType
}

func (c *DbClient) GetBlock(moduleId int, blockIdx int) (Block, error) {
	blockRow := c.db.QueryRow(getBlockQuery, blockIdx, moduleId)
	block := Block{}
	err := blockRow.Scan(&block.Id, &block.ModuleId, &block.BlockIndex, &block.BlockType)
	if err != nil {
		return Block{}, err
	}
	return block, nil
}

const getQuestionQuery = `
select q.id, q.block_id, q.question_text
from questions q
where q.block_id = ?;
`

type Question struct {
	Id           int
	BlockId      int
	QuestionText string
}

func (c *DbClient) GetQuestionFromBlock(blockId int) (Question, error) {
	questionRow := c.db.QueryRow(getQuestionQuery, blockId)
	question := Question{}
	err := questionRow.Scan(&question.Id, &question.BlockId, &question.QuestionText)
	if err != nil {
		return Question{}, err
	}
	return question, nil
}

const getChoicesForQuestionQuery = `
select ch.id, ch.question_id, ch.choice_text, ch.correct
from choices ch
where ch.question_id = ?
order by ch.id;
`

type Choice struct {
	Id         int
	QuestionId int
	ChoiceText string
	Correct  bool
}

func (c *DbClient) GetChoicesForQuestion(questionId int) ([]Choice, error) {
	choiceRows, err := c.db.Query(getChoicesForQuestionQuery, questionId)
	if err != nil {
		return nil, err
	}
	defer choiceRows.Close()
	choices := []Choice{}
	for choiceRows.Next() {
		choice := Choice{}
		err := choiceRows.Scan(&choice.Id, &choice.QuestionId, &choice.ChoiceText, &choice.Correct)
		if err != nil {
			return nil, err
		}
		choices = append(choices, choice)
	}
	if err := choiceRows.Err(); err != nil {
		return nil, err
	}
	return choices, nil
}

func (c *DbClient) GetExplanationForQuestion(questionId int) (Content, error) {
	explanationRow := c.db.QueryRow(getExplanationContentQuery, questionId)
	content := Content{}
	err := explanationRow.Scan(&content.Id, &content.Content)
	if err != sql.ErrNoRows && err != nil {
		return Content{}, err
	} else if err == sql.ErrNoRows {
		return Content{ -1, "" }, nil
	} else {
		return content, nil
	}
}

type Content struct {
	Id      int
	Content string
}

func (c *DbClient) GetContentFromBlock(blockId int) (Content, error) {
	contentRow := c.db.QueryRow(getContentForBlockQuery, blockId)
	content := Content{}
	err := contentRow.Scan(&content.Id, &content.Content)
	if err != nil {
		return Content{}, err
	}
	return content, nil
}

func (c *DbClient) DeleteModule(moduleId int) error {
	tx, err := c.db.Begin()
	_, err = tx.Exec(deleteContentForModuleQuery, moduleId)
	if err != nil {
		tx.Rollback()
		return err
	}
	_, err = tx.Exec("delete from modules where id = ?;", moduleId)
	if err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
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

const getBlockCountQuery = `
select count(*)
from blocks b
where b.module_id = ?;
`

func (c *DbClient) GetBlockCount(moduleId int) (int, error) {
	row := c.db.QueryRow(getBlockCountQuery, moduleId)
	var blockCount int
	err := row.Scan(&blockCount)
	if err != nil {
		return 0, err
	}
	return blockCount, nil
}

const getQuestionCountQuery = `
select count(*)
from questions q
join blocks b on q.block_id = b.id
where b.module_id = ?;
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

// Blocks are pieces of a module, either a question or piece of content.
const createBlockTable = `
create table if not exists blocks (
	id integer primary key autoincrement,
	module_id integer not null,
	block_index integer not null,
	block_type text not null,
	foreign key (module_id) references modules(id) on delete cascade
);
`

const createQuestionTable = `
create table if not exists questions (
	id integer primary key autoincrement,
	block_id integer not null unique,
	question_text text not null,
	foreign key (block_id) references blocks(id) on delete cascade
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

const createContentBlockTable = `
create table if not exists content_blocks (
	id integer primary key autoincrement,
	block_id integer not null unique,
	content_id integer not null,
	foreign key (block_id) references blocks(id) on delete cascade,
	foreign key (content_id) references content(id) on delete cascade
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

// TODO: webauthn/passkeys
const createUserTable = `
create table if not exists users (
	id integer primary key autoincrement,
	username string not null unique
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
		createBlockTable,
		createQuestionTable,
		createChoiceTable,
		createAnswerTable,
		createContentBlockTable,
		createContentTable,
		createExplanationTable,
		createUserTable,
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

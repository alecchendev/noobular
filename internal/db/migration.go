package db

import (
	"database/sql"
	"strconv"
)

// Hey! So you're looking to make a DB migration.
// Some things to remember to not blow everything up:
// - Always make sure to make a backup of the DB in case things go wrong.
// - If you are adding a foreign key column (and so need to create a new
// table), make sure to not accidentally delete on cascade all the other
// tables that reference the one you're replacing, i.e. you will need
// to create new tables and migrate the existing data.
// - Adding a new table for the first time does not require a migration.
// - Migrations should include all raw sql standalone so that it isn't
// dependent on other code that may be changed in the future.

const incrementVersionNumber = `
update db_version
set version = version + 1;
`

func latestDbVersion() DbVersion {
	return DbVersion(len(migrations())) - 1
}

type DbMigration func(tx *sql.Tx) error

func migrateToVersionFunc(version DbVersion) DbMigration {
	return migrations()[version]
}

func migrations() []DbMigration {
	return []DbMigration{
		func(tx *sql.Tx) error { return nil }, // version 0
		addPublicColumnToCoursesTable,
		markdownQuestionChoiceMigration,
		knowledgePointQuestionMigration,
		multiKnowledgePointQuestionMigration,
	}
}

const addPublicColumnToCoursesTableQuery = `
alter table courses
add column
public integer not null default true;
`

func addPublicColumnToCoursesTable(tx *sql.Tx) error {
	_, err := tx.Exec(addPublicColumnToCoursesTableQuery)
	return err
}

const createNewQuestionChoiceTables = `
create table if not exists questions_new (
	id integer primary key autoincrement,
	block_id integer not null unique,
	content_id integer not null,
	foreign key (block_id) references blocks(id) on delete cascade,
	foreign key (content_id) references content(id) on delete cascade
);

create table if not exists choices_new (
	id integer primary key autoincrement,
	question_id integer not null,
	content_id integer not null,
	correct bool not null,
	foreign key (question_id) references questions_new(id) on delete cascade,
	foreign key (content_id) references content(id) on delete cascade
);

create table if not exists answers_new (
	id integer primary key autoincrement,
	user_id integer not null,
	question_id integer not null,
	choice_id integer not null,
	foreign key (user_id) references users(id) on delete cascade,
	foreign key (question_id) references questions_new(id) on delete cascade
);

create table if not exists explanations_new (
	id integer primary key autoincrement,
	question_id integer not null,
	content_id integer not null,
	foreign key (question_id) references questions_new(id) on delete cascade,
	foreign key (content_id) references content(id) on delete cascade
);
`

const getAllQuestions = `
select q.id, q.block_id, q.question_text
from questions q
`

const insertNewQuestionQuery = `
insert into questions_new(block_id, content_id)
values(?, ?);
`

const getChoicesForQuestionQueryOld = `
select ch.id, ch.question_id, ch.choice_text, ch.correct
from choices ch
where ch.question_id = ?
order by ch.id;
`

func GetChoicesForQuestionOld(tx *sql.Tx, questionId int) ([]Choice, error) {
	choiceRows, err := tx.Query(getChoicesForQuestionQueryOld, questionId)
	if err != nil {
		return nil, err
	}
	defer choiceRows.Close()
	return rowsToChoices(choiceRows)
}

const insertNewChoiceQuery = `
insert into choices_new(question_id, content_id, correct)
values(?, ?, ?);
`

const getAnswersForQuestionQueryOld = `
select a.id, a.user_id, a.choice_id
from answers a
where a.question_id = ?;
`

func GetAnswersForQuestionOld(tx *sql.Tx, questionId int) ([]Answer, error) {
	rows, err := tx.Query(getAnswersForQuestionQueryOld, questionId)
	defer rows.Close()
	if err != nil {
		return nil, err
	}
	answers := []Answer{}
	for rows.Next() {
		id := 0
		userId := 0
		choiceId := 0
		err := rows.Scan(&id, &userId, &choiceId)
		if err != nil {
			return nil, err
		}
		answers = append(answers, NewAnswer(id, userId, questionId, choiceId))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return answers, nil
}

const insertNewAnswerQuery = `
insert into answers_new(user_id, question_id, choice_id)
values(?, ?, ?);
`

const insertNewExplanationQuery = `
insert into explanations_new(question_id, content_id)
values(?, ?);
`

const dropAndRenameOldQuestionChoiceTables = `
drop table questions;
drop table choices;
drop table answers;
drop table explanations;

alter table questions_new
rename to questions;

alter table choices_new
rename to choices;

alter table answers_new
rename to answers;

alter table explanations_new
rename to explanations;
`

func markdownQuestionChoiceMigration(tx *sql.Tx) error {
	// // New tables with columns
	// _, err := tx.Exec(createNewQuestionChoiceTables)
	// if err != nil {
	// 	return err
	// }
	// // migrate data
	// questionRows, err := tx.Query(getAllQuestions)
	// if err != nil {
	// 	return err
	// }
	// defer questionRows.Close()
	// questions := []Question{}
	// for questionRows.Next() {
	// 	question := Question{}
	// 	err := questionRows.Scan(&question.Id, &question.BlockId, &question.QuestionText)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	questions = append(questions, question)
	// }
	// for _, question := range questions {
	// 	questionContentId, err := InsertContent(tx, question.QuestionText)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	res, err := tx.Exec(insertNewQuestionQuery, question.BlockId, questionContentId)
	// 	if err != nil {
	// 		return fmt.Errorf("error inserting new question: %v", err)
	// 	}
	// 	newQuestionId, err := res.LastInsertId()
	// 	if err != nil {
	// 		return err
	// 	}
	// 	answers, err := GetAnswersForQuestionOld(tx, question.Id)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	choices, err := GetChoicesForQuestionOld(tx, question.Id)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	newChoiceIds := map[int]int64{}
	// 	for _, choice := range choices {
	// 		// create content for choice
	// 		choiceContentId, err := InsertContent(tx, choice.ChoiceText)
	// 		if err != nil {
	// 			return fmt.Errorf("error inserting new choice content: %v", err)
	// 		}
	// 		// insert into new choices table
	// 		res, err = tx.Exec(insertNewChoiceQuery, newQuestionId, choiceContentId, choice.Correct)
	// 		if err != nil {
	// 			return fmt.Errorf("error inserting new choice: %v", err)
	// 		}
	// 		newChoiceId, err := res.LastInsertId()
	// 		if err != nil {
	// 			return err
	// 		}
	// 		newChoiceIds[choice.Id] = newChoiceId
	// 	}
	// 	for _, answer := range answers {
	// 		_, err = tx.Exec(insertNewAnswerQuery, answer.UserId, newQuestionId, newChoiceIds[answer.ChoiceId])
	// 		if err != nil {
	// 			return fmt.Errorf("error inserting new answer: %v", err)
	// 		}
	// 	}
	// 	explanationContent, err := GetExplanationForQuestion(tx, int64(question.Id))
	// 	if err != nil {
	// 		return err
	// 	}
	// 	if explanationContent.Id != -1 {
	// 		_, err = tx.Exec(insertNewExplanationQuery, newQuestionId, explanationContent.Id)
	// 		if err != nil {
	// 			return err
	// 		}
	// 	}
	// }
	//
	// // delete old, rename new
	// _, err = tx.Exec(dropAndRenameOldQuestionChoiceTables)
	// if err != nil {
	// 	return err
	// }
	return nil
}

const renameOldNonKnowledgePointQuestionTables = `
alter table questions
rename to questions_old;

alter table choices
rename to choices_old;

alter table answers
rename to answers_old;

alter table explanations
rename to explanations_old;
`

// Just the actual create table functions at the time
const createNewKnowledgePointQuestionTables = `
create table if not exists questions (
	id integer primary key autoincrement,
	knowledge_point_id integer not null unique,
	content_id integer not null,
	foreign key (knowledge_point_id) references knowledge_points(id) on delete cascade,
	foreign key (content_id) references content(id) on delete cascade
);

create table if not exists choices (
	id integer primary key autoincrement,
	question_id integer not null,
	content_id integer not null,
	correct bool not null,
	foreign key (question_id) references questions(id) on delete cascade,
	foreign key (content_id) references content(id) on delete cascade
);

create table if not exists answers (
	id integer primary key autoincrement,
	user_id integer not null,
	question_id integer not null,
	choice_id integer not null,
	foreign key (user_id) references users(id) on delete cascade,
	foreign key (question_id) references questions(id) on delete cascade
);

create table if not exists explanations (
	id integer primary key autoincrement,
	question_id integer not null,
	content_id integer not null,
	foreign key (question_id) references questions(id) on delete cascade,
	foreign key (content_id) references content(id) on delete cascade
);
`

func migrateOldQuestionsToKnowledgePointQuestions(tx *sql.Tx) error {
	// get old questions
	rows, err := tx.Query(`
		select mod.course_id, q.id, q.block_id, q.content_id
		from questions_old q
		join blocks b on q.block_id = b.id
		join module_versions m on b.module_version_id = m.id
		join modules mod on m.module_id = mod.id;
	`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		// Get question
		var courseId int64
		var questionId int64
		var blockId int64
		var contentId int64
		err := rows.Scan(&courseId, &questionId, &blockId, &contentId)
		if err != nil {
			return err
		}
		// Update block type
		_, err = tx.Exec("update blocks set block_type = 'knowledge_point' where id = ?;", blockId)
		if err != nil {
			return err
		}
		// Create knowledge point
		knowledgePointName := "knowledge point: " + strconv.Itoa(int(blockId))
		res, err := tx.Exec("insert into knowledge_points(course_id, name) values(?, ?);", courseId, knowledgePointName)
		if err != nil {
			return err
		}
		knowledgePointId, err := res.LastInsertId()
		if err != nil {
			return err
		}
		// Link knowledge point and block
		_, err = tx.Exec("insert into knowledge_point_blocks(block_id, knowledge_point_id) values(?, ?);", blockId, knowledgePointId)
		if err != nil {
			return err
		}
		// Create question in new table
		res, err = tx.Exec("insert into questions(knowledge_point_id, content_id) values(?, ?);", knowledgePointId, contentId)
		if err != nil {
			return err
		}
		newQuestionId, err := res.LastInsertId()
		if err != nil {
			return err
		}
		// Get old choices
		choiceRows, err := tx.Query("select id, content_id, correct from choices_old where question_id = ?;", questionId)
		if err != nil {
			return err
		}
		defer choiceRows.Close()
		newChoiceIds := map[int64]int64{}
		// Fill new choices
		for choiceRows.Next() {
			var id int64
			var contentId int64
			var correct bool
			err := choiceRows.Scan(&id, &contentId, &correct)
			if err != nil {
				return err
			}
			// Create choice in new table
			res, err = tx.Exec("insert into choices(question_id, content_id, correct) values(?, ?, ?);", newQuestionId, contentId, correct)
			if err != nil {
				return err
			}
			choiceId, err := res.LastInsertId()
			if err != nil {
				return err
			}
			newChoiceIds[id] = choiceId
		}

		// Get old answers
		answerRows, err := tx.Query("select id, user_id, choice_id from answers_old where question_id = ?;", questionId)
		if err != nil {
			return err
		}
		defer answerRows.Close()
		// Fill new answers
		for answerRows.Next() {
			var id int64
			var userId int64
			var choiceId int64
			err := answerRows.Scan(&id, &userId, &choiceId)
			if err != nil {
				return err
			}
			_, err = tx.Exec("insert into answers(user_id, question_id, choice_id) values(?, ?, ?);", userId, newQuestionId, newChoiceIds[choiceId])
			if err != nil {
				return err
			}
		}
		// Migrate explanation
		explainRow := tx.QueryRow("select id, content_id from explanations_old where question_id = ?;", questionId)
		var explanationId int64
		var explanationContentId int64
		err = explainRow.Scan(&explanationId, &explanationContentId)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		if err == nil {
			_, err = tx.Exec("insert into explanations(question_id, content_id) values(?, ?);", newQuestionId, explanationContentId)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

const deleteOldNonKnowledgePointQuestionTables = `
drop table questions_old;
drop table choices_old;
drop table answers_old;
drop table explanations_old;
`

func knowledgePointQuestionMigration(tx *sql.Tx) error {
	// rename old tables
	_, err := tx.Exec(renameOldNonKnowledgePointQuestionTables)
	if err != nil {
		return err
	}
	// create new tables
	_, err = tx.Exec(createNewKnowledgePointQuestionTables)
	if err != nil {
		return err
	}
	// migrate data
	err = migrateOldQuestionsToKnowledgePointQuestions(tx)
	if err != nil {
		return err
	}
	// delete old tables
	_, err = tx.Exec(deleteOldNonKnowledgePointQuestionTables)
	if err != nil {
		return err
	}
	return nil
}

// Remove the unique constraint on knowledge_point_id column on questions

const createMultiKnowledgePointQuestionTables = `
create table if not exists questions (
	id integer primary key autoincrement,
	knowledge_point_id integer not null,
	content_id integer not null,
	foreign key (knowledge_point_id) references knowledge_points(id) on delete cascade,
	foreign key (content_id) references content(id) on delete cascade
);

create table if not exists choices (
	id integer primary key autoincrement,
	question_id integer not null,
	content_id integer not null,
	correct bool not null,
	foreign key (question_id) references questions(id) on delete cascade,
	foreign key (content_id) references content(id) on delete cascade
);

create table if not exists answers (
	id integer primary key autoincrement,
	user_id integer not null,
	question_id integer not null,
	choice_id integer not null,
	foreign key (user_id) references users(id) on delete cascade,
	foreign key (question_id) references questions(id) on delete cascade
);

create table if not exists explanations (
	id integer primary key autoincrement,
	question_id integer not null,
	content_id integer not null,
	foreign key (question_id) references questions(id) on delete cascade,
	foreign key (content_id) references content(id) on delete cascade
);
`

func migrateSingleToMultiKnowledgePointQuestions(tx *sql.Tx) error {
	// get old questions
	rows, err := tx.Query(`
		select q.id, q.knowledge_point_id, q.content_id
		from questions_old q;
	`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		// Get question
		var questionId int64
		var knowledgePointId int64
		var contentId int64
		err := rows.Scan(&questionId, &knowledgePointId, &contentId)
		if err != nil {
			return err
		}
		// Create question in new table
		res, err := tx.Exec("insert into questions(knowledge_point_id, content_id) values(?, ?);", knowledgePointId, contentId)
		if err != nil {
			return err
		}
		newQuestionId, err := res.LastInsertId()
		if err != nil {
			return err
		}
		// Get old choices
		choiceRows, err := tx.Query("select id, content_id, correct from choices_old where question_id = ?;", questionId)
		if err != nil {
			return err
		}
		defer choiceRows.Close()
		newChoiceIds := map[int64]int64{}
		// Fill new choices
		for choiceRows.Next() {
			var id int64
			var contentId int64
			var correct bool
			err := choiceRows.Scan(&id, &contentId, &correct)
			if err != nil {
				return err
			}
			// Create choice in new table
			res, err = tx.Exec("insert into choices(question_id, content_id, correct) values(?, ?, ?);", newQuestionId, contentId, correct)
			if err != nil {
				return err
			}
			choiceId, err := res.LastInsertId()
			if err != nil {
				return err
			}
			newChoiceIds[id] = choiceId
		}

		// Get old answers
		answerRows, err := tx.Query("select id, user_id, choice_id from answers_old where question_id = ?;", questionId)
		if err != nil {
			return err
		}
		defer answerRows.Close()
		// Fill new answers
		for answerRows.Next() {
			var id int64
			var userId int64
			var choiceId int64
			err := answerRows.Scan(&id, &userId, &choiceId)
			if err != nil {
				return err
			}
			_, err = tx.Exec("insert into answers(user_id, question_id, choice_id) values(?, ?, ?);", userId, newQuestionId, newChoiceIds[choiceId])
			if err != nil {
				return err
			}
		}
		// Migrate explanation
		explainRow := tx.QueryRow("select id, content_id from explanations_old where question_id = ?;", questionId)
		var explanationId int64
		var explanationContentId int64
		err = explainRow.Scan(&explanationId, &explanationContentId)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		if err == nil {
			_, err = tx.Exec("insert into explanations(question_id, content_id) values(?, ?);", newQuestionId, explanationContentId)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func multiKnowledgePointQuestionMigration(tx *sql.Tx) error {
	// rename old tables
	_, err := tx.Exec(renameOldNonKnowledgePointQuestionTables)
	if err != nil {
		return err
	}
	// create new tables
	_, err = tx.Exec(createMultiKnowledgePointQuestionTables)
	if err != nil {
		return err
	}
	// migrate data
	err = migrateSingleToMultiKnowledgePointQuestions(tx)
	if err != nil {
		return err
	}
	// delete old tables
	_, err = tx.Exec(deleteOldNonKnowledgePointQuestionTables)
	if err != nil {
		return err
	}
	return nil
}

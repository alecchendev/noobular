package db

import (
	"database/sql"
	"fmt"
)

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
	// New tables with columns
	_, err := tx.Exec(createNewQuestionChoiceTables)
	if err != nil {
		return err
	}
	// migrate data
	questionRows, err := tx.Query(getAllQuestions)
	if err != nil {
		return err
	}
	defer questionRows.Close()
	questions := []Question{}
	for questionRows.Next() {
		question := Question{}
		err := questionRows.Scan(&question.Id, &question.BlockId, &question.QuestionText)
		if err != nil {
			return err
		}
		questions = append(questions, question)
	}
	for _, question := range questions {
		questionContentId, err := InsertContent(tx, question.QuestionText)
		if err != nil {
			return err
		}
		res, err := tx.Exec(insertNewQuestionQuery, question.BlockId, questionContentId)
		if err != nil {
			return fmt.Errorf("error inserting new question: %v", err)
		}
		newQuestionId, err := res.LastInsertId()
		if err != nil {
			return err
		}
		answers, err := GetAnswersForQuestionOld(tx, question.Id)
		if err != nil {
			return err
		}
		choices, err := GetChoicesForQuestionOld(tx, question.Id)
		if err != nil {
			return err
		}
		newChoiceIds := map[int]int64{}
		for _, choice := range choices {
			// create content for choice
			choiceContentId, err := InsertContent(tx, choice.ChoiceText)
			if err != nil {
				return fmt.Errorf("error inserting new choice content: %v", err)
			}
			// insert into new choices table
			res, err = tx.Exec(insertNewChoiceQuery, newQuestionId, choiceContentId, choice.Correct)
			if err != nil {
				return fmt.Errorf("error inserting new choice: %v", err)
			}
			newChoiceId, err := res.LastInsertId()
			if err != nil {
				return err
			}
			newChoiceIds[choice.Id] = newChoiceId
		}
		for _, answer := range answers {
			_, err = tx.Exec(insertNewAnswerQuery, answer.UserId, newQuestionId, newChoiceIds[answer.ChoiceId])
			if err != nil {
				return fmt.Errorf("error inserting new answer: %v", err)
			}
		}
		explanationContent, err := GetExplanationForQuestion(tx, int64(question.Id))
		if err != nil {
			return err
		}
		if explanationContent.Id != -1 {
			_, err = tx.Exec(insertNewExplanationQuery, newQuestionId, explanationContent.Id)
			if err != nil {
				return err
			}
		}
	}

	// delete old, rename new
	_, err = tx.Exec(dropAndRenameOldQuestionChoiceTables)
	if err != nil {
		return err
	}
	return nil
}

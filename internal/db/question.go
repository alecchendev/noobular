package db

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

const createQuestionTable = `
create table if not exists questions (
	id integer primary key autoincrement,
	block_id integer not null unique,
	question_text text not null,
	foreign key (block_id) references blocks(id) on delete cascade
);
`

type Question struct {
	Id           int
	BlockId      int
	QuestionText string
}

const insertQuestionQuery = `
insert into questions(block_id, question_text)
values(?, ?);
`

// Need to rollback tx upon error one level up the stack because this function will not do that.
func InsertQuestion(tx *sql.Tx, blockId int64, question string, choices []string, correctChoiceIdx int, explanation string) error {
	res, err := tx.Exec(insertQuestionQuery, blockId, question)
	if err != nil {
		return err
	}
	questionId, err := res.LastInsertId()
	if err != nil {
		return err
	}
	for choiceIdx, choice := range choices {
		_, err = InsertChoice(tx, questionId, choice, choiceIdx == correctChoiceIdx)
		if err != nil {
			return err
		}
	}
	content, err := GetExplanationForQuestion(tx, questionId)
	if err != nil {
		return err
	} else if content.Id == -1 && explanation != "" {
		contentId, err := InsertContent(tx, explanation)
		if err != nil {
			return err
		}
		err = InsertExplanation(tx, int(questionId), int(contentId))
		if err != nil {
			return err
		}
	} else {
		// TODO if explanation is empty, just delete the content row
		err = UpdateContent(tx, int64(content.Id), explanation)
		if err != nil {
			return err
		}
	}
	return nil
}

const getQuestionQuery = `
select q.id, q.block_id, q.question_text
from questions q
where q.block_id = ?;
`

func (c *DbClient) GetQuestionFromBlock(blockId int) (Question, error) {
	questionRow := c.db.QueryRow(getQuestionQuery, blockId)
	question := Question{}
	err := questionRow.Scan(&question.Id, &question.BlockId, &question.QuestionText)
	if err != nil {
		return Question{}, err
	}
	return question, nil
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

const getQuestionsQuery = `
select q.id, q.question_text
from questions q
join blocks b on q.block_id = b.id
where b.module_id = ?
order by q.id;
`

func (c *DbClient) GetNextUnansweredQuestionIdx(userId int64, moduleId int) (int, error) {
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
		answer, err := c.GetAnswer(userId, questionId)
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


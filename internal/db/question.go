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

func NewQuestion(id int, blockId int, question string) Question {
	return Question{id, blockId, question}
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

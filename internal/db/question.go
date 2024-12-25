package db

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

const createQuestionTable = `
create table if not exists questions (
	id integer primary key autoincrement,
	block_id integer not null unique,
	content_id integer not null,
	foreign key (block_id) references blocks(id) on delete cascade,
	foreign key (content_id) references content(id) on delete cascade
);
`

type Question struct {
	Id           int
	BlockId      int
	ContentId    int
}

func NewQuestion(id int, blockId int, contentId int) Question {
	return Question{id, blockId, contentId}
}

const insertQuestionQuery = `
insert into questions(block_id, content_id)
values(?, ?);
`

// Need to rollback tx upon error one level up the stack because this function will not do that.
func InsertQuestion(tx *sql.Tx, blockId int64, question string, choices []string, correctChoiceIdx int, explanation string) error {
	questionContentId, err := InsertContent(tx, question)
	if err != nil {
		return err
	}
	res, err := tx.Exec(insertQuestionQuery, blockId, questionContentId)
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
	explanationContentId, err := InsertContent(tx, explanation)
	if err != nil {
		return err
	}
	err = InsertExplanation(tx, int(questionId), int(explanationContentId))
	if err != nil {
		return err
	}
	return nil
}

const getQuestionQuery = `
select q.id, q.block_id, q.content_id
from questions q
join content c on q.content_id = c.id
where q.block_id = ?;
`

func (c *DbClient) GetQuestionFromBlock(blockId int) (Question, error) {
	questionRow := c.db.QueryRow(getQuestionQuery, blockId)
	id := 0
	contentId := 0
	err := questionRow.Scan(&id, &blockId, &contentId)
	if err != nil {
		return Question{}, err
	}
	return NewQuestion(id, blockId, contentId), nil
}

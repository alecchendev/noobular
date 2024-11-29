package db

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

const createAnswerTable = `
create table if not exists answers (
	id integer primary key autoincrement,
	user_id integer not null,
	question_id integer not null,
	choice_id integer not null,
	foreign key (user_id) references users(id) on delete cascade,
	foreign key (question_id) references questions(id) on delete cascade
);
`

const storeAnswerQuery = `
update answers
set choice_id = ?
where user_id = ? and question_id = ?;

insert into answers(user_id, question_id, choice_id)
select ?, ?, ?
where not exists (select 1 from answers where user_id = ? and question_id = ?);
`

func (c *DbClient) StoreAnswer(userId int64, questionId int, choiceId int) error {
	_, err := c.db.Exec(storeAnswerQuery, choiceId, userId, questionId, userId, questionId, choiceId, userId, questionId)
	return err
}

const getAnswerQuery = `
select a.choice_id
from answers a
where a.user_id = ? and a.question_id = ?;
`

// Returns the choice id of the answer for the question if it exists.
// Returns -1 if there is no answer for the question.
func (c *DbClient) GetAnswer(userId int64, questionId int) (int, error) {
	row := c.db.QueryRow(getAnswerQuery, userId, questionId)
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

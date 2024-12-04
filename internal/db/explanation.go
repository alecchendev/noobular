package db

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

const createExplanationTable = `
create table if not exists explanations (
	id integer primary key autoincrement,
	question_id integer not null,
	content_id integer not null,
	foreign key (question_id) references questions(id) on delete cascade,
	foreign key (content_id) references content(id) on delete cascade
);
`

const insertExplanationQuery = `
insert into explanations(question_id, content_id)
values(?, ?);
`

func InsertExplanation(tx *sql.Tx, questionId int, contentId int) error {
	_, err := tx.Exec(insertExplanationQuery, questionId, contentId)
	if err != nil {
		return err
	}
	return nil
}

const getExplanationContentQuery = `
select c.id, c.content
from explanations e
join content c on e.content_id = c.id
where e.question_id = ?;
`

func (c *DbClient) GetExplanationForQuestion(questionId int) (Content, error) {
	tx, err := c.db.Begin()
	defer tx.Rollback()
	if err != nil {
		return Content{}, err
	}
	content, err := GetExplanationForQuestion(tx, int64(questionId))
	if err != nil {
		return Content{}, err
	}
	err = tx.Commit()
	if err != nil {
		return Content{}, err
	}
	return content, nil
}

func GetExplanationForQuestion(tx *sql.Tx, questionId int64) (Content, error) {
	explanationRow := tx.QueryRow(getExplanationContentQuery, questionId)
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

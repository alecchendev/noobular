package db

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)


const insertExplanationQuery = `
insert into explanations(question_id, content_id)
values(?, ?);
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


package db

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

const createChoiceTable = `
create table if not exists choices (
	id integer primary key autoincrement,
	question_id integer not null,
	content_id integer not null,
	correct bool not null,
	foreign key (question_id) references questions(id) on delete cascade,
	foreign key (content_id) references content(id) on delete cascade
);
`

type Choice struct {
	Id         int
	QuestionId int
	ContentId  int
	Correct    bool
}

func NewChoice(id int, questionId int, contentId int, correct bool) Choice {
	return Choice{id, questionId, contentId, correct}
}

const insertChoiceQuery = `
insert into choices(question_id, content_id, correct)
values(?, ?, ?);
`

func InsertChoice(tx *sql.Tx, questionId int64, choiceText string, correct bool) (Choice, error) {
	choiceContentId, err := InsertContent(tx, choiceText)
	if err != nil {
		return Choice{}, err
	}
	res, err := tx.Exec(insertChoiceQuery, questionId, choiceContentId, correct)
	if err != nil {
		return Choice{}, err
	}
	choiceId, err := res.LastInsertId()
	if err != nil {
		return Choice{}, err
	}
	return NewChoice(int(choiceId), int(questionId), int(choiceContentId), correct), nil
}

const getChoiceQuery = `
select ch.id, ch.question_id, ch.content_id, ch.correct
from choices ch
where ch.id = ?;
`

func (c *DbClient) GetChoice(choiceId int) (Choice, error) {
	row := c.db.QueryRow(getChoiceQuery, choiceId)
	id := 0
	questionId := 0
	contentId := 0
	correct := false
	err := row.Scan(&id, &questionId, &contentId, &correct)
	if err != nil {
		return Choice{}, err
	}
	return NewChoice(id, questionId, contentId, correct), nil
}

func rowsToChoices(choiceRows *sql.Rows) ([]Choice, error) {
	choices := []Choice{}
	for choiceRows.Next() {
		id := 0
		questionId := 0
		contentId := 0
		correct := false
		err := choiceRows.Scan(&id, &questionId, &contentId, &correct)
		if err != nil {
			return nil, err
		}
		choices = append(choices, NewChoice(id, questionId, contentId, correct))
	}
	if err := choiceRows.Err(); err != nil {
		return nil, err
	}
	return choices, nil
}

const getChoicesForQuestionQuery = `
select ch.id, ch.question_id, ch.content_id, ch.correct
from choices ch
where ch.question_id = ?
order by ch.id;
`

func (c *DbClient) GetChoicesForQuestion(questionId int) ([]Choice, error) {
	choiceRows, err := c.db.Query(getChoicesForQuestionQuery, questionId)
	if err != nil {
		return nil, err
	}
	defer choiceRows.Close()
	return rowsToChoices(choiceRows)
}

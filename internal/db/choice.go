package db

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

const createChoiceTable = `
create table if not exists choices (
	id integer primary key autoincrement,
	question_id integer not null,
	choice_text text not null,
	correct bool not null,
	foreign key (question_id) references questions(id) on delete cascade
);
`

type Choice struct {
	Id         int
	QuestionId int
	ChoiceText string
	Correct    bool
}

func NewChoice(id int, questionId int, choiceText string, correct bool) Choice {
	return Choice{id, questionId, choiceText, correct}
}

const insertChoiceQuery = `
insert into choices(question_id, choice_text, correct)
values(?, ?, ?);
`

func InsertChoice(tx *sql.Tx, questionId int64, choiceText string, correct bool) (Choice, error) {
	res, err := tx.Exec(insertChoiceQuery, questionId, choiceText, correct)
	if err != nil {
		return Choice{}, err
	}
	choiceId, err := res.LastInsertId()
	if err != nil {
		return Choice{}, err
	}
	return Choice{int(choiceId), int(questionId), choiceText, correct}, nil
}

const getChoiceQuery = `
select ch.id, ch.question_id, ch.choice_text, ch.correct
from choices ch
where ch.id = ?;
`

func (c *DbClient) GetChoice(choiceId int) (Choice, error) {
	row := c.db.QueryRow(getChoiceQuery, choiceId)
	choice := Choice{}
	err := row.Scan(&choice.Id, &choice.QuestionId, &choice.ChoiceText, &choice.Correct)
	if err != nil {
		return Choice{}, err
	}
	return choice, nil
}

const getChoicesForQuestionQuery = `
select ch.id, ch.question_id, ch.choice_text, ch.correct
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

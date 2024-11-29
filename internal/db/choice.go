package db

import (
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

const insertChoiceQuery = `
insert into choices(question_id, choice_text, correct)
values(?, ?, ?);
`

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

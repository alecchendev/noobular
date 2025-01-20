package db

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

const createQuestionTable = `
create table if not exists questions (
	id integer primary key autoincrement,
	knowledge_point_id integer not null unique,
	content_id integer not null,
	foreign key (knowledge_point_id) references knowledge_points(id) on delete cascade,
	foreign key (content_id) references content(id) on delete cascade
);
`

type Question struct {
	Id             int
	KnowledgePoint int64
	ContentId      int
}

func NewQuestion(id int, knowledgePointId int64, contentId int) Question {
	return Question{id, knowledgePointId, contentId}
}

const insertQuestionQuery = `
insert into questions(knowledge_point_id, content_id)
values(?, ?);
`

// Need to rollback tx upon error one level up the stack because this function will not do that.
func InsertQuestion(tx *sql.Tx, knowledgePointId int64, question string, choices []string, correctChoiceIdx int, explanation string) error {
	questionContentId, err := InsertContent(tx, question)
	if err != nil {
		return err
	}
	res, err := tx.Exec(insertQuestionQuery, knowledgePointId, questionContentId)
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

const getQuestionsForKnowledgePointQuery = `
select q.id, q.knowledge_point_id, q.content_id
from questions q
join content c on q.content_id = c.id
where q.knowledge_point_id = ?;
`

func (c *DbClient) GetQuestionsForKnowledgePoint(knowledgePointId int64) ([]Question, error) {
	questionRows, err := c.db.Query(getQuestionsForKnowledgePointQuery, knowledgePointId)
	defer questionRows.Close()
	if err != nil {
		return nil, err
	}
	questions := []Question{}
	for questionRows.Next() {
		id := 0
		contentId := 0
		err := questionRows.Scan(&id, &knowledgePointId, &contentId)
		if err != nil {
			return nil, err
		}
		questions = append(questions, NewQuestion(id, knowledgePointId, contentId))
	}
	if err := questionRows.Err(); err != nil {
		return nil, err
	}
	return questions, nil
}

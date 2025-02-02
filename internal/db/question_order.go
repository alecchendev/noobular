package db

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

// A question_order holds an entry for what question a user saw first, second etc.
// for a given knowlegdge point.
const createQuestionOrderTable = `
create table if not exists question_orders (
	id integer primary key autoincrement,
	visit_id integer not null,
	knowledge_point_id integer not null,
	question_id integer not null,
	question_index integer not null,
	foreign key (visit_id) references visits(id) on delete cascade,
	foreign key (knowledge_point_id) references knowledge_points(id) on delete cascade,
	foreign key (question_id) references questions(id) on delete cascade,
	constraint question_order_unique unique(visit_id, knowledge_point_id, question_id) on conflict fail
);
`

type QuestionOrder struct {
	Id               int64
	VisitId          int64
	KnowledgePointId int64
	QuestionId       int64
	QuestionIndex    int
}

func NewQuestionOrder(id int64, visitId int64, knowledgePointId int64, questionId int64, questionIndex int) QuestionOrder {
	return QuestionOrder{id, visitId, knowledgePointId, questionId, questionIndex}
}

const insertQuestionOrderQuery = `
insert into question_orders(visit_id, knowledge_point_id, question_id, question_index)
values(?, ?, ?, ?);
`

func (c *DbClient) InsertQuestionOrder(visitId int64, knowledgePointId int64, questionId int64, questionIndex int) (QuestionOrder, error) {
	tx, err := c.db.Begin()
	if err != nil {
		return QuestionOrder{}, err
	}
	defer tx.Rollback()
	questionOrder, err := InsertQuestionOrder(tx, visitId, knowledgePointId, questionId, questionIndex)
	if err != nil {
		return QuestionOrder{}, err
	}
	err = tx.Commit()
	if err != nil {
		return QuestionOrder{}, err
	}
	return questionOrder, nil
}

func InsertQuestionOrder(tx *sql.Tx, visitId int64, knowledgePointId int64, questionId int64, questionIndex int) (QuestionOrder, error) {
	res, err := tx.Exec(insertQuestionOrderQuery, visitId, knowledgePointId, questionId, questionIndex)
	if err != nil {
		return QuestionOrder{}, err
	}
	id, error := res.LastInsertId()
	if error != nil {
		return QuestionOrder{}, error
	}
	return NewQuestionOrder(id, visitId, knowledgePointId, questionId, questionIndex), nil
}

const getQuestionOrderQuery = `
select qo.id, qo.visit_id, qo.knowledge_point_id, qo.question_id, qo.question_index
from question_orders qo
where qo.visit_id = ? and qo.knowledge_point_id = ?
order by qo.question_index desc;
`

func (c *DbClient) GetQuestionOrders(visitId int64, knowledgePointId int64) ([]QuestionOrder, error) {
	rows, err := c.db.Query(getQuestionOrderQuery, visitId, knowledgePointId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var questionOrders []QuestionOrder
	for rows.Next() {
		var id int64
		var visitId int64
		var knowledgePointId int64
		var questionId int64
		var questionIndex int
		err := rows.Scan(&id, &visitId, &knowledgePointId, &questionId, &questionIndex)
		if err != nil {
			return nil, err
		}
		questionOrders = append(questionOrders, NewQuestionOrder(id, visitId, knowledgePointId, questionId, questionIndex))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return questionOrders, nil
}

package db

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

// A knowledge point is a unit of learning. It has
// an associated pool of questions used for practice and
// testing whether a student has learned something.
const createKnowledgePointTable = `
create table if not exists knowledge_points (
	id integer primary key autoincrement,
	course_id integer not null,
	name string not null,
	foreign key (course_id) references courses(id) on delete cascade
);
`

type KnowledgePoint struct {
	Id       int64
	CourseId int64
	Name     string
}

func NewKnowledgePoint(id int64, courseId int64,  name string) KnowledgePoint {
	return KnowledgePoint{id, courseId, name}
}

const insertKnowledgePointQuery = `
insert into knowledge_points(course_id, name)
values(?, ?);
`

func InsertKnowledgePoint(tx *sql.Tx, courseId int64, name string) (KnowledgePoint, error) {
	res, err := tx.Exec(insertKnowledgePointQuery, courseId, name)
	if err != nil {
		return KnowledgePoint{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return KnowledgePoint{}, err
	}
	return NewKnowledgePoint(id, courseId, name), nil
}

const updateKnowledgePointQuery = `
update knowledge_points
set name = ?
where id = ? and course_id = ?;
`

func UpdateKnowledgePoint(tx *sql.Tx, kpId int64, courseId int64, name string) (KnowledgePoint, error) {
	_, err := tx.Exec(updateKnowledgePointQuery, name, kpId, courseId)
	if err != nil {
		return KnowledgePoint{}, err
	}
	return NewKnowledgePoint(kpId, courseId, name), nil
}

const getKnowledgePointsForCourseQuery = `
select k.id, k.course_id, k.name
from knowledge_points k
where k.course_id = ?;
`

func (c *DbClient) GetKnowledgePoints(courseId int64) ([]KnowledgePoint, error) {
	rows, err := c.db.Query(getKnowledgePointsForCourseQuery, courseId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	knowledgePoints := []KnowledgePoint{}
	for rows.Next() {
		var id int64
		var courseId int64
		var name string
		err := rows.Scan(&id, &courseId, &name)
		if err != nil {
			return nil, err
		}
		knowledgePoints = append(knowledgePoints, NewKnowledgePoint(id, courseId, name))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return knowledgePoints, nil
}

const deleteUnansweredQuestionsContentForKnowledgePointQuery = `
with knowledge_point_ids as (
	select id from knowledge_points where id = ?
),
answered_question_ids as (
	select question_id from answers
),
question_ids as (
	select id from questions where knowledge_point_id in knowledge_point_ids and id not in answered_question_ids
),
question_content_ids as (
	select content_id from questions where id in question_ids
),
choice_content_ids as (
	select content_id from choices where question_id in question_ids
),
explanation_content_ids as (
	select content_id from explanations where question_id in question_ids
),
referenced_content_ids as (
	select * from question_content_ids
	union
	select * from choice_content_ids
	union
	select * from explanation_content_ids
),
elsewhere_content_block_content_ids as (
	select content_id from content_blocks
),
elsewhere_knowledge_point_ids as (
	select knowledge_point_id from knowledge_point_blocks where knowledge_point_id not in knowledge_point_ids
),
elsewhere_question_ids as (
	select id from questions where knowledge_point_id in elsewhere_knowledge_point_ids
),
elsewhere_question_content_ids as (
	select content_id from questions where id in elsewhere_question_ids
),
elsewhere_choice_content_ids as (
	select content_id from choices where question_id in elsewhere_question_ids
),
elsewhere_explanation_content_ids as (
	select content_id from explanations where question_id in elsewhere_question_ids
),
referenced_elsewhere_content_ids as (
	select * from elsewhere_content_block_content_ids
	union
	select * from elsewhere_question_content_ids
	union
	select * from elsewhere_choice_content_ids
	union
	select * from elsewhere_explanation_content_ids
),
referenced_only_here_content_ids as (
	select * from referenced_content_ids
	except
	select * from referenced_elsewhere_content_ids
)
delete from content
where id in referenced_only_here_content_ids;
`

const deleteUnansweredQuestionsForKnowledgePointQuery = `
with answered_question_ids as (
	select question_id from answers
)
delete from questions
where knowledge_point_id = ? and id not in answered_question_ids;
`

func DeleteUnansweredQuestionsForKnowledgePoint(tx *sql.Tx, kpId int64) error {
	_, err := tx.Exec(deleteUnansweredQuestionsContentForKnowledgePointQuery, kpId)
	if err != nil {
		return err
	}
	_, err = tx.Exec(deleteUnansweredQuestionsForKnowledgePointQuery, kpId)
	return err
}

const markQuestionsOld = `
update questions
set latest = false
where knowledge_point_id = ?;
`

func MarkQuestionsOld(tx *sql.Tx, kpId int64) error {
	_, err := tx.Exec(markQuestionsOld, kpId)
	return err
}

// Knowledge point block

const createKnowledgePointBlockTable = `
create table if not exists knowledge_point_blocks (
	id integer primary key autoincrement,
	block_id integer not null unique,
	knowledge_point_id integer not null,
	foreign key (block_id) references blocks(id) on delete cascade,
	foreign key (knowledge_point_id) references knowledge_points(id) on delete cascade
);
`

const insertKnowledgePointBlockQuery = `
insert into knowledge_point_blocks(block_id, knowledge_point_id)
values(?, ?);
`

func InsertKnowledgePointBlock(tx *sql.Tx, blockId int64, knowledgePointId int64) error {
	_, err := tx.Exec(insertKnowledgePointBlockQuery, blockId, knowledgePointId)
	return err
}

const getKnowledgePointQuery = `
select id, course_id, name
from knowledge_points
where course_id = ? and id = ?;
`

func (c *DbClient) GetKnowledgePoint(kpCourseId int64, kpId int64) (KnowledgePoint, error) {
	row := c.db.QueryRow(getKnowledgePointQuery, kpCourseId, kpId)
	var id int64
	var courseId int64
	var name string
	err := row.Scan(&id, &kpCourseId, &name)
	if err != nil {
		return KnowledgePoint{}, err
	}
	return NewKnowledgePoint(id, courseId, name), nil
}
	
const getKnowledgePointFromBlockQuery = `
select k.id, k.course_id, k.name
from knowledge_points k
join knowledge_point_blocks kb on k.id = kb.knowledge_point_id
where kb.block_id = ?;
`

func (c *DbClient) GetKnowledgePointFromBlock(blockId int) (KnowledgePoint, error) {
	row := c.db.QueryRow(getKnowledgePointFromBlockQuery, blockId)
	var id int64
	var courseId int64
	var name string
	err := row.Scan(&id, &courseId, &name)
	if err != nil {
		return KnowledgePoint{}, err
	}
	return NewKnowledgePoint(id, courseId, name), nil
}

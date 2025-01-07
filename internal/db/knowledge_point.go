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

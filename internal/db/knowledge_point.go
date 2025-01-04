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

package db

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

// An enrollment is just a table to track what courses a student is enrolled in.
const createEnrollmentTable = `
create table if not exists enrollments (
	id integer primary key autoincrement,
	user_id integer not null,
	course_id integer not null,
	foreign key (user_id) references users(id) on delete cascade,
	foreign key (course_id) references courses(id) on delete cascade,
	constraint enrollment_ unique(user_id, course_id) on conflict fail
);
`

type Enrollment struct {
	Id      int64
	UserId  int64
	CourseId int
}

func NewEnrollment(id int64, userId int64, courseId int) Enrollment {
	return Enrollment{id, userId, courseId}
}

const insertEnrollmentQuery = `
insert into enrollments(user_id, course_id)
values(?, ?);
`

func (c *DbClient) InsertEnrollment(userId int64, courseId int) (Enrollment, error) {
	res, err := c.db.Exec(insertEnrollmentQuery, userId, courseId)
	if err != nil {
		return Enrollment{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Enrollment{}, err
	}
	return NewEnrollment(id, userId, courseId), nil
}

const getEnrollmentQuery = `
select e.id, e.user_id, e.course_id
from enrollments e
where e.user_id = ? and e.course_id = ?;
`

func (c *DbClient) GetEnrollment(userId int64, courseId int) (Enrollment, error) {
	row := c.db.QueryRow(getEnrollmentQuery, userId, courseId)
	var enrollment Enrollment
	err := row.Scan(&enrollment.Id, &enrollment.UserId, &enrollment.CourseId)
	if err != nil {
		return Enrollment{}, err
	}
	return enrollment, nil
}

const getEnrollmentCountQuery = `
select count(*)
from enrollments e
where e.course_id = ?;
`

func GetEnrollmentCount(tx *sql.Tx, courseId int) (int64, error) {
	row := tx.QueryRow(getEnrollmentCountQuery, courseId)
	var count int64
	err := row.Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (c *DbClient) GetEnrollmentCount(courseId int) (int64, error) {
	tx, err := c.Begin()
	defer tx.Rollback()
	if err != nil {
		return 0, err
	}
	row := tx.QueryRow(getEnrollmentCountQuery, courseId)
	var count int64
	err = row.Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

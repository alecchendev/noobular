package db

import (
	_ "github.com/mattn/go-sqlite3"
)

// An enrollment is just a table to track what courses a student is enrolled in.
const createEnrollmentTable = `
create table if not exists enrollments (
	id integer primary key autoincrement,
	user_id integer not null,
	course_id integer not null,
	foreign key (user_id) references users(id) on delete cascade,
	foreign key (course_id) references courses(id) on delete cascade
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

const getEnrollmentsQuery = `
select e.id, e.user_id, e.course_id
from enrollments e
where e.user_id = ?;
`

func (c *DbClient) GetEnrollments(userId int64) ([]Enrollment, error) {
	rows, err := c.db.Query(getEnrollmentsQuery, userId)
	if err != nil {
		return []Enrollment{}, err
	}
	defer rows.Close()
	enrollments := []Enrollment{}
	for rows.Next() {
		var enrollment Enrollment
		err := rows.Scan(&enrollment.Id, &enrollment.UserId, &enrollment.CourseId)
		if err != nil {
			return []Enrollment{}, err
		}
		enrollments = append(enrollments, enrollment)
	}
	if err := rows.Err(); err != nil {
		return []Enrollment{}, err
	}
	return enrollments, nil
}

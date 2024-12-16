package db

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

const createCourseTable = `
create table if not exists courses (
	id integer primary key autoincrement,
	user_id integer not null,
	title text not null,
	description text not null,
	public integer not null default true,
	foreign key (user_id) references users(id) on delete cascade
);
`

type Course struct {
	Id          int
	Title       string
	Description string
	Public      bool
}

func NewCourse(id int, title string, description string, public bool) Course {
	return Course{id, title, description, public}
}

const insertCourseQuery = `
insert into courses(user_id, title, description, public)
values(?, ?, ?, ?);
`

func (c *DbClient) CreateCourse(userId int64, title string, description string, public bool) (Course, error) {
	res, err := c.db.Exec(insertCourseQuery, userId, title, description, public)
	if err != nil {
		return Course{}, err
	}
	courseId, err := res.LastInsertId()
	if err != nil {
		return Course{}, err
	}
	return NewCourse(int(courseId), title, description, public), nil
}

const updateCourseQuery = `
update courses
set title = ?, description = ?, public = ?
where id = ? and user_id = ?;
`

func EditCourse(tx *sql.Tx, userId int64, courseId int, title string, description string, public bool) (Course, error) {
	_, err := tx.Exec(updateCourseQuery, title, description, public, courseId, userId)
	if err != nil {
		return Course{}, err
	}
	return NewCourse(courseId, title, description, public), nil
}

func rowToCourse(row *sql.Row) (Course, error) {
	var id int
	var title string
	var description string
	var public bool
	err := row.Scan(&id, &title, &description, &public)
	if err != nil {
		return Course{}, err
	}
	return NewCourse(id, title, description, public), nil
}

const getCourseQuery = `
select c.id, c.title, c.description, c.public
from courses c
where c.id = ?;
`

func (c *DbClient) GetCourse(courseId int) (Course, error) {
	row := c.db.QueryRow(getCourseQuery, courseId)
	return rowToCourse(row)
}

const getTeacherCourseQuery = `
select c.id, c.title, c.description, c.public
from courses c
where c.id = ? and c.user_id = ?;
`

func (c *DbClient) GetTeacherCourse(courseId int, userId int64) (Course, error) {
	row := c.db.QueryRow(getTeacherCourseQuery, courseId, userId)
	return rowToCourse(row)
}

const getTeacherCoursesQuery = `
select c.id, c.title, c.description, c.public
from courses c
where c.user_id = ?
order by c.id;
`

func (c *DbClient) GetTeacherCourses(userId int64) ([]Course, error) {
	courseRows, err := c.db.Query(getTeacherCoursesQuery, userId)
	if err != nil {
		return nil, err
	}
	defer courseRows.Close()
	return rowsToCourses(courseRows)
}

const getPublicCoursesQuery = `
select c.id, c.title, c.description, c.public
from courses c
where c.public = true
order by c.id;
`

func (c *DbClient) GetPublicCourses() ([]Course, error) {
	courseRows, err := c.db.Query(getPublicCoursesQuery)
	if err != nil {
		return nil, err
	}
	defer courseRows.Close()
	return rowsToCourses(courseRows)
}

func rowsToCourses(courseRows *sql.Rows) ([]Course, error) {
	var courses []Course
	for courseRows.Next() {
		var id int
		var title string
		var description string
		var public bool
		err := courseRows.Scan(&id, &title, &description, &public)
		if err != nil {
			return nil, err
		}
		courses = append(courses, NewCourse(id, title, description, public))
	}
	if err := courseRows.Err(); err != nil {
		return nil, err
	}
	return courses, nil
}

const getEditCourseQuery = `
select c.id, c.title, c.description, c.public
from courses c
where c.user_id = ? and c.id = ?;
`

func (c *DbClient) GetEditCourse(userId int64, courseId int) (Course, error) {
	row := c.db.QueryRow(getEditCourseQuery, userId, courseId)
	return rowToCourse(row)
}

const getModuleCourseQuery = `
select c.id, c.title, c.description, c.public
from modules m
join courses c on m.course_id = c.id
where c.user_id = ? and m.id = ?;
`

func (c *DbClient) GetModuleCourse(userId int64, moduleId int) (Course, error) {
	row := c.db.QueryRow(getModuleCourseQuery, userId, moduleId)
	return rowToCourse(row)
}

const deleteCourseQuery = `
delete from courses
where user_id = ? and id = ?;
`

func (c *DbClient) DeleteCourse(userId int64, courseId int) error {
	tx, err := c.db.Begin()
	defer tx.Rollback()
	modules, err := c.GetModules(courseId)
	for _, module := range modules {
		_, err = tx.Exec(deleteContentForModuleQuery, module.Id)
		if err != nil {
			return err
		}
	}
	_, err = tx.Exec(deleteCourseQuery, userId, courseId)
	if err != nil {
		return err
	}
	return tx.Commit()
}

const getEnrolledCoursesQuery = `
select c.id, c.title, c.description, c.public
from courses c
join enrollments e on c.id = e.course_id
where e.user_id = ?
order by c.id;
`

func (c *DbClient) GetEnrolledCourses(userId int64) ([]Course, error) {
	courseRows, err := c.db.Query(getEnrolledCoursesQuery, userId)
	if err != nil {
		return nil, err
	}
	defer courseRows.Close()
	return rowsToCourses(courseRows)
}

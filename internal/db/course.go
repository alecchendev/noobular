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
	foreign key (user_id) references users(id) on delete cascade
);
`

type Course struct {
	Id          int
	Title       string
	Description string
}

func NewCourse(id int, title string, description string) Course {
	return Course{id, title, description}
}

const insertCourseQuery = `
insert into courses(user_id, title, description)
values(?, ?, ?);
`

func (c *DbClient) CreateCourse(userId int64, title string, description string) (Course, error) {
	res, err := c.db.Exec(insertCourseQuery, userId, title, description)
	if err != nil {
		return Course{}, err
	}
	courseId, err := res.LastInsertId()
	if err != nil {
		return Course{}, err
	}
	return Course{int(courseId), title, description}, nil
}

const updateCourseQuery = `
update courses
set title = ?, description = ?
where id = ? and user_id = ?;
`

func EditCourse(tx *sql.Tx, userId int64, courseId int, title string, description string) (Course, error) {
	_, err := tx.Exec(updateCourseQuery, title, description, courseId, userId)
	if err != nil {
		return Course{}, err
	}
	return NewCourse(courseId, title, description), nil
}

const getCourseQuery = `
select c.id, c.title, c.description
from courses c
where c.id = ?;
`

func (c *DbClient) GetCourse(courseId int) (Course, error) {
	row := c.db.QueryRow(getCourseQuery, courseId)
	var course Course
	err := row.Scan(&course.Id, &course.Title, &course.Description)
	if err != nil {
		return Course{}, err
	}
	return course, nil
}

const getCoursesQuery = `
select c.id, c.title, c.description
from courses c
order by c.id;
`

const getTeacherCoursesQuery = `
select c.id, c.title, c.description
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

func (c *DbClient) GetCourses() ([]Course, error) {
	courseRows, err := c.db.Query(getCoursesQuery)
	if err != nil {
		return nil, err
	}
	defer courseRows.Close()
	return rowsToCourses(courseRows)
}

func rowsToCourses(courseRows *sql.Rows) ([]Course, error) {
	var courses []Course
	for courseRows.Next() {
		var course Course
		err := courseRows.Scan(&course.Id, &course.Title, &course.Description)
		if err != nil {
			return nil, err
		}
		courses = append(courses, course)
	}
	if err := courseRows.Err(); err != nil {
		return nil, err
	}
	return courses, nil
}

const getEditCourseQuery = `
select c.id, c.title, c.description
from courses c
where c.user_id = ? and c.id = ?;
`

func (c *DbClient) GetEditCourse(userId int64, courseId int) (Course, error) {
       row := c.db.QueryRow(getEditCourseQuery, userId, courseId)
       var course Course
       err := row.Scan(&course.Id, &course.Title, &course.Description)
       if err != nil {
               return Course{}, err
       }
       return course, nil
}

const getModuleCourseQuery = `
select c.id, c.title, c.description
from modules m
join courses c on m.course_id = c.id
where c.user_id = ? and m.id = ?;
`

func (c *DbClient) GetModuleCourse(userId int64, moduleId int) (Course, error) {
	row := c.db.QueryRow(getModuleCourseQuery, userId, moduleId)
	var course Course
	err := row.Scan(&course.Id, &course.Title, &course.Description)
	if err != nil {
		return Course{}, err
	}
	return course, nil
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
select c.id, c.title, c.description
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

package db

import (
	"database/sql"
	"fmt"

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

func (c *DbClient) CreateCourse(userId int64, title string, description string, moduleTitles []string, moduleDescriptions []string) (Course, []Module, error) {
	if len(moduleTitles) != len(moduleDescriptions) {
		return Course{}, []Module{}, fmt.Errorf("moduleTitles and moduleDescriptions must have the same length")
	}
	res, err := c.db.Exec(insertCourseQuery, userId, title, description)
	if err != nil {
		return Course{}, []Module{}, err
	}
	courseId, err := res.LastInsertId()
	if err != nil {
		return Course{}, []Module{}, err
	}
	course := Course{int(courseId), title, description}
	modules := make([]Module, len(moduleTitles))
	for i := 0; i < len(moduleTitles); i++ {
		moduleTitle := moduleTitles[i]
		moduleDescription := moduleDescriptions[i]
		module, err := c.CreateModule(int(courseId), moduleTitle, moduleDescription)
		if err != nil {
			return Course{}, []Module{}, err
		}
		modules[i] = module
	}
	return course, modules, nil
}

const updateCourseQuery = `
update courses
set title = ?, description = ?
where id = ?;
`

func (c *DbClient) EditCourse(courseId int, title string, description string, moduleIds []int, moduleTitles []string, moduleDescriptions []string) (Course, []Module, error) {
	if len(moduleTitles) != len(moduleDescriptions) || len(moduleTitles) != len(moduleIds) {
		return Course{}, []Module{}, fmt.Errorf("moduleTitles, moduleDescriptions, and moduleIds must have the same length, got titles: %d, descs: %d, ids: %d", len(moduleTitles), len(moduleDescriptions), len(moduleIds))
	}
	tx, err := c.db.Begin()
	_, err = tx.Exec(updateCourseQuery, title, description, courseId)
	if err != nil {
		tx.Rollback()
		return Course{}, []Module{}, err
	}
	course := Course{courseId, title, description}
	modules := make([]Module, len(moduleTitles))
	for i := 0; i < len(moduleTitles); i++ {
		moduleId := moduleIds[i]
		moduleTitle := moduleTitles[i]
		moduleDescription := moduleDescriptions[i]
		// -1 means this is a new module
		if moduleId == -1 {
			module, err := CreateModule(tx, courseId, moduleTitle, moduleDescription)
			if err != nil {
				tx.Rollback()
				return Course{}, []Module{}, err
			}
			moduleId = module.Id
		} else {
			// No need to instert new module version just to change the name.
			version, err := GetLatestModuleVersion(tx, moduleId)
			if err != nil {
				tx.Rollback()
				return Course{}, []Module{}, err
			}
			err = UpdateModuleVersionMetadata(tx, version.Id, moduleTitle, moduleDescription)
			if err != nil {
				tx.Rollback()
				return Course{}, []Module{}, err
			}
		}
		module := Module{moduleId, course.Id}
		modules[i] = module
	}
	err = tx.Commit()
	if err != nil {
		return Course{}, []Module{}, err
	}
	return course, modules, nil
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

const getUserCoursesQuery = `
select c.id, c.title, c.description
from courses c
where c.user_id = ?
order by c.id;
`

const getCoursesWithModulesWithBlocksQuery = `
select distinct c.id, c.title, c.description
from courses c
join modules m on c.id = m.course_id
join module_versions mv on m.id = mv.module_id
join blocks b on mv.id = b.module_version_id
order by c.id;
`

func (c *DbClient) GetCourses(userId int64) ([]Course, error) {
	var courseRows *sql.Rows
	var err error
	if userId != -1 {
		courseRows, err = c.db.Query(getUserCoursesQuery, userId)
	} else {
		courseRows, err = c.db.Query(getCoursesWithModulesWithBlocksQuery)
	}
	if err != nil {
		return nil, err
	}
	defer courseRows.Close()

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
	modules, err := c.GetModules(courseId, false)
	for _, module := range modules {
		_, err = tx.Exec(deleteContentForModuleQuery, module.Id)
		if err != nil {
			tx.Rollback()
			return err
		}
	}
	_, err = tx.Exec(deleteCourseQuery, userId, courseId)
	if err != nil {
		tx.Rollback()
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

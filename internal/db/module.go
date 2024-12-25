package db

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

const createModuleTable = `
create table if not exists modules (
	id integer primary key autoincrement,
	course_id integer not null,
	foreign key (course_id) references courses(id) on delete cascade
);
`

type Module struct {
	Id          int
	CourseId    int
}

func NewModule(id int, courseId int) Module {
	return Module{id, courseId}
}

const insertModuleQuery = `
insert into modules(course_id)
values(?);
`

func (c *DbClient) CreateModule(courseId int, moduleTitle string, moduleDescription string) (Module, error) {
	tx, err := c.db.Begin()
	defer tx.Rollback()
	if err != nil {
		return Module{}, err
	}
	module, err := CreateModule(tx, courseId, moduleTitle, moduleDescription)
	if err != nil {
		return Module{}, err
	}
	err = tx.Commit()
	if err != nil {
		return Module{}, err
	}
	return module, nil
}

func CreateModule(tx *sql.Tx, courseId int, moduleTitle string, moduleDescription string) (Module, error) {
	res, err := tx.Exec(insertModuleQuery, courseId)
	if err != nil {
		return Module{}, err
	}
	moduleId, err := res.LastInsertId()
	if err != nil {
		return Module{}, err
	}
	_, err = InsertModuleVersion(tx, int(moduleId), moduleTitle, moduleDescription)
	if err != nil {
		return Module{}, err
	}
	return Module{int(moduleId), courseId}, nil
}

const getModulesQuery = `
select m.id, m.course_id
from modules m
where m.course_id = ?
order by m.id;
`

func (c *DbClient) GetModules(courseId int) ([]Module, error) {
	moduleRows, err := c.db.Query(getModulesQuery, courseId)
	if err != nil {
		return nil, err
	}
	defer moduleRows.Close()
	modules := []Module{}
	for moduleRows.Next() {
		var module Module
		err := moduleRows.Scan(&module.Id, &module.CourseId)
		if err != nil {
			return nil, err
		}
		modules = append(modules, module)
	}
	if err := moduleRows.Err(); err != nil {
		return nil, err
	}
	return modules, nil
}

// Desired behavior: only delete content if it's only referenced by this module
const deleteContentForModuleQuery = `
with module_block_ids as (
	select b.id from blocks b
	join module_versions mv on b.module_version_id = mv.id
	where mv.module_id = ?
),
content_block_content_ids as (
	select content_id from content_blocks where block_id in module_block_ids
),
question_ids as (
	select id from questions where block_id in module_block_ids
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
	select * from content_block_content_ids
	union
	select * from question_content_ids
	union
	select * from choice_content_ids
	union
	select * from explanation_content_ids
),
elsewhere_content_block_content_ids as (
	select content_id from content_blocks where block_id not in module_block_ids
),
elsewhere_question_ids as (
	select id from questions where block_id not in module_block_ids
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

func DeleteContentForModule(tx *sql.Tx, moduleId int) error {
	_, err := tx.Exec(deleteContentForModuleQuery, moduleId)
	return err
}

const getModuleQuery = `
select m.id, m.course_id
from modules m
where m.id = ? and m.course_id = ?;
`

func GetModule(tx *sql.Tx, courseId, moduleId int) (Module, error) {
	moduleRow := tx.QueryRow(getModuleQuery, moduleId, courseId)
	var module Module
	err := moduleRow.Scan(&module.Id, &module.CourseId)
	if err != nil {
		return Module{}, err
	}
	return module, nil
}

func (c *DbClient) GetModule(courseId int, moduleId int) (Module, error) {
	tx, err := c.Begin()
	defer tx.Rollback()
	if err != nil {
		return Module{}, err
	}
	return GetModule(tx, courseId, moduleId)
}

func (c *DbClient) DeleteModule(moduleId int) error {
	tx, err := c.db.Begin()
	defer tx.Rollback()
	err = DeleteContentForModule(tx, moduleId)
	if err != nil {
		return err
	}
	_, err = tx.Exec("delete from modules where id = ?;", moduleId)
	if err != nil {
		return err
	}
	return tx.Commit()
}


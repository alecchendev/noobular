package db

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

// TODO: delete title and description once module versions is working
const createModuleTable = `
create table if not exists modules (
	id integer primary key autoincrement,
	course_id integer not null,
	title text not null,
	description text not null,
	foreign key (course_id) references courses(id) on delete cascade
);
`

type Module struct {
	Id          int
	CourseId    int
	Title       string
	Description string
}

func NewModule(id int, courseId int, title string, description string) Module {
	return Module{id, courseId, title, description}
}

const insertModuleQuery = `
insert into modules(course_id, title, description)
values(?, ?, ?);
`

func (c *DbClient) CreateModule(courseId int, moduleTitle string, moduleDescription string) (Module, error) {
	tx, err := c.db.Begin()
	if err != nil {
		return Module{}, err
	}
	module, err := CreateModule(tx, courseId, moduleTitle, moduleDescription)
	if err != nil {
		tx.Rollback()
		return Module{}, err
	}
	err = tx.Commit()
	if err != nil {
		return Module{}, err
	}
	return module, nil
}

func CreateModule(tx *sql.Tx, courseId int, moduleTitle string, moduleDescription string) (Module, error) {
	res, err := tx.Exec(insertModuleQuery, courseId, moduleTitle, moduleDescription)
	if err != nil {
		return Module{}, err
	}
	moduleId, err := res.LastInsertId()
	if err != nil {
		return Module{}, err
	}
	return Module{int(moduleId), courseId, moduleTitle, moduleDescription}, nil
}

const getModulesQuery = `
select m.id, m.course_id, m.title, m.description
from modules m
where m.course_id = ?
order by m.id;
`

const getModulesWithBlocksQuery = `
select distinct m.id, m.course_id, m.title, m.description
from modules m
join module_versions mv on m.id = mv.module_id
join blocks b on mv.id = b.module_version_id
where m.course_id = ?
order by m.id;
`

func (c *DbClient) GetModules(courseId int, requireHasBlocks bool) ([]Module, error) {
	var query string
	if requireHasBlocks {
		query = getModulesWithBlocksQuery
	} else {
		query = getModulesQuery
	}
	moduleRows, err := c.db.Query(query, courseId)
	if err != nil {
		return nil, err
	}
	defer moduleRows.Close()
	modules := []Module{}
	for moduleRows.Next() {
		var module Module
		err := moduleRows.Scan(&module.Id, &module.CourseId, &module.Title, &module.Description)
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

const updateModuleQuery = `
update modules
set title = ?, description = ?
where id = ?;
`

func UpdateModuleMetadata(tx *sql.Tx, moduleId int, title string, description string) error {
	_, err := tx.Exec(updateModuleQuery, title, description, moduleId)
	return err
}

const deleteContentForModuleQuery = `
with module_block_ids as (
	select b.id from blocks b
	join module_versions mv on b.module_version_id = mv.id
	where mv.module_id = ?
)
delete from content
where id in (
    select content_id from content_blocks where block_id in module_block_ids
)
or id in (
    select content_id from explanations where question_id in (
        select id from questions where block_id in module_block_ids
    )
);
`

func DeleteContentForModule(tx *sql.Tx, moduleId int) error {
	_, err := tx.Exec(deleteContentForModuleQuery, moduleId)
	return err
}

const getModuleQuery = `
select m.id, m.course_id, m.title, m.description
from modules m
where m.id = ?
`

func (c *DbClient) GetModule(moduleId int) (Module, error) {
	moduleRow := c.db.QueryRow(getModuleQuery, moduleId)
	var module Module
	err := moduleRow.Scan(&module.Id, &module.CourseId, &module.Title, &module.Description)
	if err != nil {
		return Module{}, err
	}
	return module, nil
}

func (c *DbClient) DeleteModule(moduleId int) error {
	tx, err := c.db.Begin()
	err = DeleteContentForModule(tx, moduleId)
	if err != nil {
		tx.Rollback()
		return err
	}
	_, err = tx.Exec("delete from modules where id = ?;", moduleId)
	if err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}


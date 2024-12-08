package db

import (
	"database/sql"
)

const createPrereqTable = `
create table if not exists prereqs (
	id integer primary key autoincrement,
	module_id integer not null,
	prereq_module_id integer not null,
	foreign key (module_id) references modules(id) on delete cascade,
	foreign key (prereq_module_id) references modules(id) on delete cascade,
	constraint prereq_ unique(module_id, prereq_module_id) on conflict fail
);
`

type Prereq struct {
	Id            int
	ModuleId      int
	PrereqModuleId int
}

func NewPrereq(id int, moduleId int, prereqModuleId int) Prereq {
	return Prereq{id, moduleId, prereqModuleId}
}

const insertPrereqQuery = `
insert into prereqs(module_id, prereq_module_id)
values(?, ?);
`

func InsertPrereq(tx *sql.Tx, moduleId int, prereqModuleId int) (Prereq, error) {
	res, err := tx.Exec(insertPrereqQuery, moduleId, prereqModuleId)
	if err != nil {
		return Prereq{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Prereq{}, err
	}
	return NewPrereq(int(id), moduleId, prereqModuleId), nil
}

const getPrereqsQuery = `
select p.id, p.module_id, p.prereq_module_id
from prereqs p
where p.module_id = ?;
`

func (c *DbClient) GetPrereqs(moduleId int) ([]Prereq, error) {
	rows, err := c.db.Query(getPrereqsQuery, moduleId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var prereqs []Prereq
	for rows.Next() {
		var id int
		var moduleId int
		var prereqModuleId int
		err := rows.Scan(&id, &moduleId, &prereqModuleId)
		if err != nil {
			return nil, err
		}
		prereqs = append(prereqs, NewPrereq(id, moduleId, prereqModuleId))
	}
	return prereqs, nil
}

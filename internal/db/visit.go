package db

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

// A visit holds the latest block index that a user has visited in a module.
const createVisitTable = `
create table if not exists visits (
	id integer primary key autoincrement,
	user_id integer not null,
	module_id integer not null,
	block_index integer not null,
	foreign key (user_id) references users(id) on delete cascade
	foreign key (module_id) references modules(id) on delete cascade
);
`

type Visit struct {
	Id        int64
	UserId    int64
	ModuleId  int
	BlockIndex int
}

func NewVisit(id int64, userId int64, moduleId int, blockIdx int) Visit {
	return Visit{id, userId, moduleId, blockIdx}
}

const insertVisitQuery = `
insert into visits(user_id, module_id, block_index)
values(?, ?, ?);
`

func InsertVisit(tx *sql.Tx, userId int64, moduleId int, blockIdx int) (Visit, error) {
	res, err := tx.Exec(insertVisitQuery, userId, moduleId, blockIdx)
	if err != nil {
		return Visit{}, err
	}
	id, error := res.LastInsertId()
	if error != nil {
		return Visit{}, error
	}
	return NewVisit(id, userId, moduleId, blockIdx), nil
}

const getVisitQuery = `
select v.id, v.user_id, v.module_id, v.block_index
from visits v
where v.user_id = ? and v.module_id = ?;
`

func (c *DbClient) GetVisit(userId int64, moduleId int) (Visit, error) {
	row := c.db.QueryRow(getVisitQuery, userId, moduleId)
	var visit Visit
	err := row.Scan(&visit.Id, &visit.UserId, &visit.ModuleId, &visit.BlockIndex)
	if err != nil {
		return Visit{}, err
	}
	return visit, nil
}

func (c *DbClient) GetOrCreateVisit(userId int64, moduleId int) (Visit, error) {
	tx, err := c.db.Begin()
	if err != nil {
		return Visit{}, err
	}
	visit, err := c.GetVisit(userId, moduleId)
	if err != nil && err == sql.ErrNoRows {
		visit, err = InsertVisit(tx, userId, moduleId, 0)
	} else if err != nil {
		return Visit{}, err
	}
	err = tx.Commit()
	if err != nil {
		return Visit{}, err
	}
	return visit, nil
}

const updateVisitQuery = `
update visits
set block_index = ?
where user_id = ? and module_id = ?;
`

func (c *DbClient) UpdateVisit(userId int64, moduleId int, blockIdx int) error {
	_, err := c.db.Exec(updateVisitQuery, blockIdx, userId, moduleId)
	return err
}

const deleteVisitsForModuleQuery = `
delete from visits
where module_id = ?;
`

func DeleteVisitsForModule(tx *sql.Tx, moduleId int) error {
	_, err := tx.Exec(deleteVisitsForModuleQuery, moduleId)
	return err
}

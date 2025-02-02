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
	module_version_id integer not null,
	block_index integer not null,
	foreign key (user_id) references users(id) on delete cascade
	foreign key (module_version_id) references module_versions(id) on delete cascade,
	constraint visit_ unique(user_id, module_version_id) on conflict fail
);
`

type Visit struct {
	Id        int64
	UserId    int64
	ModuleVersionId  int64
	BlockIndex int
}

func NewVisit(id int64, userId int64, moduleVersionId int64, blockIdx int) Visit {
	return Visit{id, userId, moduleVersionId, blockIdx}
}

const insertVisitQuery = `
insert into visits(user_id, module_version_id, block_index)
values(?, ?, ?);
`

func insertVisit(tx *sql.Tx, userId int64, moduleVersionId int64, blockIdx int) (Visit, error) {
	res, err := tx.Exec(insertVisitQuery, userId, moduleVersionId, blockIdx)
	if err != nil {
		return Visit{}, err
	}
	id, error := res.LastInsertId()
	if error != nil {
		return Visit{}, error
	}
	return NewVisit(id, userId, moduleVersionId, blockIdx), nil
}

const getVisitCountQuery = `
select count(*)
from visits v
join module_versions mv on v.module_version_id = mv.id
where mv.module_id = ? and mv.version_number = ?;
`

func GetVisitCount(tx *sql.Tx, moduleId int, versionNumber int64) (int, error) {
	row := tx.QueryRow(getVisitCountQuery, moduleId, versionNumber)
	var count int
	err := row.Scan(&count)
	if err != nil && err != sql.ErrNoRows {
		return 0, err
	}
	return count, nil
}

const getVisitForModuleQuery = `
select v.id, v.user_id, v.module_version_id, v.block_index
from visits v
join module_versions mv on v.module_version_id = mv.id
where v.user_id = ? and mv.module_id = ?;
`

func (c *DbClient) GetVisit(userId int64, moduleId int) (Visit, error) {
	row := c.db.QueryRow(getVisitForModuleQuery, userId, moduleId)
	var visit Visit
	err := row.Scan(&visit.Id, &visit.UserId, &visit.ModuleVersionId, &visit.BlockIndex)
	if err != nil {
		return Visit{}, err
	}
	return visit, nil
}

func (c *DbClient) CreateVisit(userId int64, moduleId int) (Visit, error) {
	tx, err := c.db.Begin()
	if err != nil {
		return Visit{}, err
	}
	version, err := GetLatestModuleVersion(tx, moduleId)
	if err != nil {
		return Visit{}, err
	}
	visit, err := insertVisit(tx, userId, version.Id, 0)
	if err != nil {
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
where user_id = ? and module_version_id = ?;
`

func UpdateVisit(tx *sql.Tx, userId int64, moduleVersionId int64, blockIdx int) error {
	_, err := tx.Exec(updateVisitQuery, blockIdx, userId, moduleVersionId)
	return err
}

func (c *DbClient) UpdateVisit(userId int64, moduleVersionId int64, blockIdx int) error {
	tx, err := c.db.Begin()
	defer tx.Rollback()
	if err != nil {
		return err
	}
	err = UpdateVisit(tx, userId, moduleVersionId, blockIdx)
	if err != nil {
		return err
	}
	return tx.Commit()
}

const deleteVisitsForModuleQuery = `
delete from visits
where module_version_id = ?;
`

func DeleteVisitsForModule(tx *sql.Tx, moduleVersionId int64) error {
	_, err := tx.Exec(deleteVisitsForModuleQuery, moduleVersionId)
	return err
}

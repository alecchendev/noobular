package db

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

// Simple table to store the DB version number.
// Single column, single row.
const createDbVersionTable = `
create table if not exists db_version (
	version integer primary key
)
`

type DbVersion int64

const insertDbVersionQuery = `
insert into db_version(version) values(?);
`

func InsertDbVersion(tx *sql.Tx) (DbVersion, error) {
	_, err := tx.Exec(insertDbVersionQuery, 0)
	return DbVersion(0), err
}

const getDbVersionQuery = `
select version from db_version;
`

func GetDbVersion(tx *sql.Tx) (DbVersion, error) {
	versionRow := tx.QueryRow(getDbVersionQuery)
	version := DbVersion(0)
	err := versionRow.Scan(&version)
	if err != nil {
		return DbVersion(0), err
	}
	return version, nil
}

const updateDbVersionQuery = `
update db_version
set version = ?;
`

func UpdateDbVersion(tx *sql.Tx, version DbVersion) error {
	_, err := tx.Exec(updateDbVersionQuery, version)
	return err
}

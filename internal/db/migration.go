package db

import (
	// "database/sql"
)

const incrementVersionNumber = `
update db_version
set version = version + 1;
`

func migrationQueries() []string {
	return []string{
		"", // Version 0
		addPublicColumnToCoursesTable,
	}
}

func latestDbVersion() DbVersion {
	return DbVersion(len(migrationQueries())) - 1
}

func migrateToVersionQuery(version DbVersion) string {
	return migrationQueries()[version]
}

const addPublicColumnToCoursesTable = `
alter table courses
add column
public integer not null default true;
`


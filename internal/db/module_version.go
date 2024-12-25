package db

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

const createModuleVersionTable = `
create table if not exists module_versions (
	id integer primary key autoincrement,
	module_id integer not null,
	version_number integer not null,
	title text not null,
	description text not null,
	foreign key (module_id) references modules(id) on delete cascade,
	constraint module_version_ unique(module_id, version_number) on conflict fail
);
`

type ModuleVersion struct {
	Id            int64
	ModuleId      int
	VersionNumber int64
	Title         string
	Description   string
}

func NewModuleVersion(id int64, moduleId int, versionNumber int64, title string, description string) ModuleVersion {
	return ModuleVersion{id, moduleId, versionNumber, title, description}
}

const getModuleVersionQuery = `
select mv.id, mv.module_id, mv.version_number, mv.title, mv.description
from module_versions mv
where mv.id = ?;
`

func (c *DbClient) GetModuleVersion(moduleVersionId int64) (ModuleVersion, error) {
	row := c.db.QueryRow(getModuleVersionQuery, moduleVersionId)
	var version ModuleVersion
	err := row.Scan(&version.Id, &version.ModuleId, &version.VersionNumber, &version.Title, &version.Description)
	if err != nil {
		return ModuleVersion{}, err
	}
	return version, nil
}

const getLatestModuleVersionQuery = `
select mv.id, mv.module_id, mv.version_number, mv.title, mv.description
from module_versions mv
where mv.module_id = ?
order by mv.version_number desc
limit 1;
`

func GetLatestModuleVersion(tx *sql.Tx, moduleId int) (ModuleVersion, error) {
	row := tx.QueryRow(getLatestModuleVersionQuery, moduleId)
	var id int64
	var versionNumber int64
	var title string
	var description string
	err := row.Scan(&id, &moduleId, &versionNumber, &title, &description)
	if err != nil {
		return ModuleVersion{}, err
	}
	return NewModuleVersion(id, moduleId, versionNumber, title, description), nil
}

func (c *DbClient) GetLatestModuleVersion(moduleId int) (ModuleVersion, error) {
	tx, err := c.db.Begin()
	if err != nil {
		return ModuleVersion{}, err
	}
	version, err := GetLatestModuleVersion(tx, moduleId)
	if err != nil {
		return ModuleVersion{}, err
	}
	err = tx.Commit()
	if err != nil {
		return ModuleVersion{}, err
	}
	return version, nil
}

const insertModuleVersionQuery = `
insert into module_versions(module_id, version_number, title, description)
values(?, ?, ?, ?);
`

func InsertModuleVersion(tx *sql.Tx, moduleId int, title string, description string) (ModuleVersion, error) {
	latestVersionNumber := int64(0)
	latestVersion, err := GetLatestModuleVersion(tx, moduleId)
	if err != nil && err != sql.ErrNoRows {
		return ModuleVersion{}, err
	} else if err == nil {
		latestVersionNumber = latestVersion.VersionNumber
	}
	// There's technically a race here. If two transactions try to insert
	// a module version at the same time, there could be a conflict.
	// Going to choose not to handle this for now, since someone would have to
	// be doing something pretty abnormal for this to happen.
	newVersionNumber := latestVersionNumber + 1
	res, err := tx.Exec(insertModuleVersionQuery, moduleId, newVersionNumber, title, description)
	if err != nil {
		return ModuleVersion{}, err
	}
	moduleVersionId, err := res.LastInsertId()
	if err != nil {
		return ModuleVersion{}, err
	}
	return NewModuleVersion(moduleVersionId, moduleId, newVersionNumber, title, description), nil
}

const updateModuleVersionMetadataQuery = `
update module_versions
set title = ?, description = ?
where id = ?;
`

func UpdateModuleVersionMetadata(tx *sql.Tx, moduleVersionId int64, title string, description string) error {
	_, err := tx.Exec(updateModuleVersionMetadataQuery, title, description, moduleVersionId)
	return err
}

// Desired behavior: only delete content if it's only referenced by this module version
const deleteContentForModuleVersionQuery = `
with module_version_block_ids as (
	select b.id from blocks b
	join module_versions mv on b.module_version_id = mv.id
	where mv.module_id = ? and mv.version_number = ?
),
content_block_content_ids as (
	select content_id from content_blocks where block_id in module_version_block_ids
),
question_ids as (
	select id from questions where block_id in module_version_block_ids
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
	select content_id from content_blocks where block_id not in module_version_block_ids
),
elsewhere_question_ids as (
	select id from questions where block_id not in module_version_block_ids
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

func DeleteContentForModuleVersion(tx *sql.Tx, moduleId int, versionNumber int64) error {
	_, err := tx.Exec(deleteContentForModuleVersionQuery, moduleId, versionNumber)
	return err
}

func DeleteModuleVersion(tx *sql.Tx, moduleId int, versionNumber int64) error {
	err := DeleteContentForModuleVersion(tx, moduleId, versionNumber)
	if err != nil {
		return err
	}
	_, err = tx.Exec("delete from module_versions where module_id = ? and version_number = ?;", moduleId, versionNumber)
	return err
}

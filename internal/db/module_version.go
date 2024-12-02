package db

import (
	"database/sql"
	"fmt"

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

// TODO: these queries are just too much
const getLatestModuleVersionsForCourseQuery = `
select mv.id, mv.module_id, mv.version_number, mv.title, mv.description
from module_versions mv
join modules m on mv.module_id = m.id
join (
    select module_id, max(version_number) as latest_version
    from module_versions
    group by module_id
) latest_mv on mv.module_id = latest_mv.module_id and mv.version_number = latest_mv.latest_version
where m.course_id = ?
order by mv.version_number desc;
`

const getLatestModuleVersionsWithBlocksForCourseQuery = `
select mv.id, mv.module_id, mv.version_number, mv.title, mv.description
from module_versions mv
join modules m on mv.module_id = m.id
join (
    select module_id, max(version_number) as latest_version
    from module_versions
    group by module_id
) latest_mv on mv.module_id = latest_mv.module_id and mv.version_number = latest_mv.latest_version
where m.course_id = ? and (
	select count(*)
	from blocks b
	where b.module_version_id = mv.id
) > 0
order by mv.version_number desc;
`

func (c *DbClient) GetLatestModuleVersionsForCourse(courseId int, requireHasBlocks bool) ([]ModuleVersion, error) {
	var query string
	if requireHasBlocks {
		query = getLatestModuleVersionsWithBlocksForCourseQuery
	} else {
		query = getLatestModuleVersionsForCourseQuery
	}
	rows, err := c.db.Query(query, courseId)
	if err != nil {
		return []ModuleVersion{}, err
	}
	versions := []ModuleVersion{}
	for rows.Next() {
		var id int64
		var moduleId int
		var versionNumber int64
		var title string
		var description string
		err := rows.Scan(&id, &moduleId, &versionNumber, &title, &description)
		if err != nil {
			return []ModuleVersion{}, err
		}
		versions = append(versions, NewModuleVersion(id, moduleId, versionNumber, title, description))
	}
	if err := rows.Err(); err != nil {
		return []ModuleVersion{}, err
	}
	return versions, nil
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

func (c *DbClient) CreateModuleVersion(moduleId int, title string, description string, blockTypes []string, contents []string, questions []string, choices [][]string, correctChoiceIdxs []int, explanations []string) error {
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	version, err := InsertModuleVersion(tx, moduleId, title, description)
	if err != nil {
		tx.Rollback()
		return err
	}
	questionIdx := 0
	contentIdx := 0
	for i, blockType := range blockTypes {
		blockId, err := InsertBlock(tx, version.Id, i, BlockType(blockType))
		if err != nil {
			tx.Rollback()
			return err
		}
		if blockType == string(ContentBlockType) {
			err = InsertContentBlock(tx, blockId, contents[contentIdx])
			if err != nil {
				tx.Rollback()
				return err
			}
			contentIdx += 1
		} else if blockType == string(QuestionBlockType) {
			err = InsertQuestion(tx, blockId, questions[questionIdx], choices[questionIdx], correctChoiceIdxs[questionIdx], explanations[questionIdx])
			if err != nil {
				tx.Rollback()
				return err
			}
			questionIdx += 1
		} else {
			return fmt.Errorf("invalid block type: %s", blockType)
		}
	}
	err = tx.Commit()
	if err != nil {
		return err
	}
	return nil
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

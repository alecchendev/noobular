package db

import (
	"database/sql"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

// Conventions:
// Each new table should have it's own file.
// The file content should go like this:
// - Table creation query (need to add this to initDb upon creation)
// - in memory struct for the table row
// - Various helpers
//    - Raw query
//    - Method on DbClient using that query
//    - Should usually return the in memory struct
// To keep things simple, we don't need to always optimally do a single
// query to get everything we need. Multiple queries that reuse existing
// methods are fine.

type DbClient struct {
	db *sql.DB
}

func NewDbClient() *DbClient {
	db, err := sql.Open("sqlite3", "test.db?_foreign_keys=on")
	if err != nil {
		log.Fatal(err)
	}
	initDb(db, false)
	return &DbClient{db}
}

func NewMemoryDbClient() *DbClient {
	db, err := sql.Open("sqlite3", ":memory:?_foreign_keys=on")
	if err != nil {
		log.Fatal(err)
	}
	initDb(db, true)
	return &DbClient{db}
}

// `fromScratch` parameter says if we don't have
// a version number, we should jump to the latest.
// If it's false, we start at zero, and run all migrations.
// When initializing from scratch or during tests, we'll
// start from the latest version. The very first time
// we migrate the production database, we want to start at 0.
func initDb(db *sql.DB, fromScratch bool) {
	tx, err := db.Begin()
	defer tx.Rollback()
	if err != nil {
		log.Fatal(err)
	}
	stmts := []string{
		createCourseTable,
		createModuleTable,
		createModuleVersionTable,
		createBlockTable,
		createQuestionTable,
		createChoiceTable,
		createAnswerTable,
		createContentBlockTable,
		createContentTable,
		createExplanationTable,
		createUserTable,
		createCredentialTable,
		createSessionTable,
		createVisitTable,
		createEnrollmentTable,
		createPointTable,
		createPrereqTable,
		createDbVersionTable,
	}
	for _, stmt := range stmts {
		_, err := tx.Exec(stmt)
		if err != nil {
			log.Fatal(err)
		}
	}
	latestVersion := latestDbVersion()
	version, err := GetDbVersion(tx)
	if err != nil && err != sql.ErrNoRows {
		log.Fatal(err)
	}
	if err == sql.ErrNoRows {
		var initialVersion DbVersion
		if fromScratch {
			initialVersion = latestVersion
		} else {
			initialVersion = DbVersion(0)
		}
		version, err = InsertDbVersion(tx, initialVersion)
		if err != nil {
			log.Fatal(err)
		}
	} else if version < latestVersion {
		log.Println("New DB version available. Current:", version, "Latest:", latestVersion)
		for version < latestVersion {
			_, err = tx.Exec(migrateToVersionQuery(version + 1))
			if err != nil {
				log.Fatal(err)
			}
			_, err = tx.Exec(incrementVersionNumber)
			if err != nil {
				log.Fatal(err)
			}
			version, err = GetDbVersion(tx)
			if err != nil {
				log.Fatal(err)
			}
			log.Println("Migrated to version:", version)
		}
	}
	log.Println("Db version:", version)
	tx.Commit()
}

func (c *DbClient) Begin() (*sql.Tx, error) {
	return c.db.Begin()
}

func (c *DbClient) Close() {
	c.db.Close()
}

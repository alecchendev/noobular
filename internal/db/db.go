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
	initDb(db)
	return &DbClient{db}
}

func NewMemoryDbClient() *DbClient {
	db, err := sql.Open("sqlite3", ":memory:?_foreign_keys=on")
	if err != nil {
		log.Fatal(err)
	}
	initDb(db)
	return &DbClient{db}
}

func initDb(db *sql.DB) {
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
		createKnowledgePointTable,
		createKnowledgePointBlockTable,
		createQuestionOrderTable,
	}
	for _, stmt := range stmts {
		_, err := tx.Exec(stmt)
		if err != nil {
			log.Fatal("Failed to create table: ", err)
		}
	}
	latestVersion := latestDbVersion()
	version, err := GetDbVersion(tx)
	if err == sql.ErrNoRows {
		version, err = InsertDbVersion(tx, latestVersion)
		if err != nil {
			log.Fatal(err)
		}
	} else if err != nil {
		log.Fatal(err)
	} else if version < latestVersion {
		log.Println("New DB version available. Current:", version, "Latest:", latestVersion)
		for version < latestVersion {
			err = migrateToVersionFunc(version + 1)(tx)
			if err != nil {
				log.Fatal("Migration failed: ", err)
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

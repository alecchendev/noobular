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
	}
	for _, stmt := range stmts {
		_, err := tx.Exec(stmt)
		if err != nil {
			log.Fatal(err)
		}
	}
	tx.Commit()
}

func (c *DbClient) Begin() (*sql.Tx, error) {
	return c.db.Begin()
}

func (c *DbClient) Close() {
	c.db.Close()
}

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
// - IDs should use UUIDs as blobs
// - in memory struct for the table row
// - Various helpers
//    - Raw query
//    - Method on DbClient using that query
//    - Should usually return the in memory struct
//    - For methods getting optional objects, add an ok bool to the return value
// To keep things simple, we don't need to always optimally do a single
// query to get everything we need. Multiple queries that compose existing
// methods are good too.

func NewDb() *sql.DB {
	db, err := sql.Open("sqlite3", "test.db?_foreign_keys=on")
	if err != nil {
		log.Fatal(err)
	}
	initDb(db)
	return db
}

func NewMemoryDb() *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:?_foreign_keys=on")
	if err != nil {
		log.Fatal(err)
	}
	initDb(db)
	return db
}

func initDb(db *sql.DB) {
	tx, err := db.Begin()
	defer tx.Rollback() // nolint: errcheck
	if err != nil {
		log.Fatal(err)
	}
	stmts := []string{
		createUserTable,
		createCredentialTable,
		createSessionTable,
	}
	for _, stmt := range stmts {
		_, err := tx.Exec(stmt)
		if err != nil {
			log.Fatal("Failed to create table: ", err)
		}
	}
	err = tx.Commit()
	if err != nil {
		log.Fatal(err)
	}
}

type DbClient struct {
	tx *sql.Tx
}

func NewDbClient(tx *sql.Tx) DbClient {
	return DbClient{tx: tx}
}

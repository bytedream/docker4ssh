package database

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
)

var globalDB *Database

type Database struct {
	*sql.DB
}

func newDatabaseConnection(driverName, dataSource string) (*Database, error) {
	database, err := sql.Open(driverName, dataSource)
	if err != nil {
		return nil, err
	}
	db := &Database{DB: database}

	return db, nil
}

func NewSqlite3Connection(databaseFile string) (*Database, error) {
	return newDatabaseConnection("sqlite3", databaseFile)
}

func GetDatabase() *Database {
	return globalDB
}

func SetDatabase(database *Database) {
	globalDB = database
}

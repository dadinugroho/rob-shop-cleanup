package main

import (
	"database/sql"

	_ "github.com/go-sql-driver/mysql"
)

// ConnectDatabase establishes connection to the MySQL database
func ConnectDatabase(config *Config) (*sql.DB, error) {
	db, err := sql.Open("mysql", config.GetDSN())
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

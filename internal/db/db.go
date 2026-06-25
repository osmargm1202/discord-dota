package db

import (
	"database/sql"
	_ "embed"
	"fmt"

	_ "github.com/lib/pq"
)

//go:embed schema.sql
var schemaSQL string

// DB wraps sql.DB with app-level methods.
type DB struct {
	*sql.DB
}

// New opens a PG connection pool and verifies connectivity.
func New(dsn string) (*DB, error) {
	sqlDB, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("sql.Open: %w", err)
	}
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("db ping: %w", err)
	}
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetMaxIdleConns(5)
	return &DB{sqlDB}, nil
}

// RunMigrations creates all tables if they do not exist.
func (d *DB) RunMigrations() error {
	if _, err := d.Exec(schemaSQL); err != nil {
		return fmt.Errorf("migrations: %w", err)
	}
	return nil
}

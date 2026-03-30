package db

import (
	"database/sql"
	"embed"
	"path/filepath"

	"github.com/jmoiron/sqlx"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

func Open(dataDir string) (*sqlx.DB, error) {
	dbPath := filepath.Join(dataDir, "data.db")
	db, err := sqlx.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	if err := configure(db); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

// OpenMemory returns an in-memory SQLite DB with all migrations applied.
// Intended for use in tests.
func OpenMemory() (*sqlx.DB, error) {
	db, err := sqlx.Open("sqlite", ":memory:")
	if err != nil {
		return nil, err
	}

	if err := configure(db); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

func configure(db *sqlx.DB) error {
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return err
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		return err
	}
	return runMigrations(db.DB)
}

func runMigrations(db *sql.DB) error {
	goose.SetBaseFS(migrations)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("sqlite3"); err != nil {
		return err
	}
	return goose.Up(db, "migrations")
}

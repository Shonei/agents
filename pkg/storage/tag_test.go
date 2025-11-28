package storage

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/duckdb/duckdb-go/v2"
)

func Test_initDB_and_run_migrations(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}

	// if this changes make sure you update the gitignore
	// I don't really want to push this to git
	dbPath := filepath.Join(cwd, "testfiles/test.db")

	_, err = os.Stat(dbPath)
	if os.IsNotExist(err) {
		os.MkdirAll(filepath.Dir(dbPath), 0o700)
	} else if err != nil {
		t.Fatalf("unable to check db stats: %v", err)
	}

	// we start with a clean DB
	if err == nil {
		_ = os.Remove(dbPath)
	}

	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		t.Errorf("failed to open database: %v", err)
	}

	if err := InitDatabase(db); err != nil {
		t.Errorf("failed to init database: %v", err)
		return
	}

	if err := RunMigrations(db); err != nil {
		t.Errorf("failed to run migrations: %v", err)
	}
}

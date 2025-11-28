package storage

import (
	"database/sql"
	"fmt"
	"path/filepath"

	"github.com/doug-martin/goqu/v9"
)

const SearchVectorSize = 2048

type Storage struct {
	goquDB  *goqu.Database
	db      *sql.DB
	vecSize int
}

func NewStorage(dbPath string) (*Storage, error) {
	if !filepath.IsAbs(dbPath) {
		return nil, fmt.Errorf("dbPath must be absolute")
	}

	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	goquDB := goqu.New("duckdb", db)

	initErr := initDatabase(goquDB)
	if initErr != nil {
		return nil, fmt.Errorf("failed to init database: %w", initErr)
	}

	runErr := runMigrations(goquDB)
	if runErr != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", runErr)
	}

	return &Storage{
		goquDB:  goquDB,
		db:      db,
		vecSize: SearchVectorSize,
	}, nil
}

func (s *Storage) Close() error {
	return s.db.Close()
}

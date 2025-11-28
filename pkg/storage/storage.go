package storage

import (
	"database/sql"
	"fmt"
	"path/filepath"

	"github.com/doug-martin/goqu/v9"
)

type Storage struct {
	sql     *sql.DB
	goquDB  *goqu.Database
	vecSize int
}

func NewRAG(dbPath string, vecSize int) (*Storage, error) {
	if vecSize <= 0 {
		return nil, fmt.Errorf("vecSize must be positive")
	}

	if !filepath.IsAbs(dbPath) {
		return nil, fmt.Errorf("dbPath must be absolute")
	}

	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	goquDB := goqu.New("duckdb", db)

	return &Storage{
		sql:     db,
		goquDB:  goquDB,
		vecSize: vecSize,
	}, nil
}

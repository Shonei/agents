package storage

import (
	"database/sql"
	"encoding/json"
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
	// Best-effort checkpoint to flush the WAL into the main DB file. If this
	// fails (e.g. read-only DB or already closed) we still try to close.
	_, _ = s.db.Exec("CHECKPOINT;")

	return s.db.Close()
}

func (s *Storage) SaveSession(id string, hash string, prompt string, parentSessionID string, tools []string, serverTools []string) error {
	toolsJSON, err := json.Marshal(tools)
	if err != nil {
		return fmt.Errorf("failed to marshal audit session tools: %w", err)
	}
	serverToolsJSON, err := json.Marshal(serverTools)
	if err != nil {
		return fmt.Errorf("failed to marshal audit session server tools: %w", err)
	}

	record := goqu.Record{
		"id":            id,
		"session_hash":  hash,
		"system_prompt": prompt,
		"tools":         string(toolsJSON),
		"server_tools":  string(serverToolsJSON),
	}
	if parentSessionID != "" {
		record["parent_session_id"] = parentSessionID
	}

	_, err = s.goquDB.Insert("audit_sessions").Rows(record).Executor().Exec()

	return err
}

func (s *Storage) SaveEvent(id string, sessionID string, eventType string, content string, payload []byte) error {
	// If payload is empty or "null", we can store empty JSON object or null
	p := string(payload)
	if len(payload) == 0 {
		p = "{}"
	}

	_, err := s.goquDB.Insert("audit_events").Rows(
		goqu.Record{
			"id":         id,
			"session_id": sessionID,
			"type":       eventType,
			"content":    content,
			"payload":    p,
		},
	).Executor().Exec()

	return err
}

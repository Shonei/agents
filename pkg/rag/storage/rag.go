package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	_ "github.com/duckdb/duckdb-go/v2"
)

type Storage struct {
	sql     *sql.DB
	vecSize int
}

func NewRAG(name string, vecSize int) (*Storage, error) {
	if vecSize <= 0 {
		return nil, fmt.Errorf("vecSize must be positive")
	}

	db, err := sql.Open("duckdb", name)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Load the VSS extension for vector similarity search.
	if _, err = db.Exec("INSTALL vss; LOAD vss;"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to load vss extension: %w", err)
	}

	// Enable persisting the vector index on disk
	if _, err = db.Exec("SET hnsw_enable_experimental_persistence = True"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to load vss extension: %w", err)
	}

	// Create a table to store documents and their embedding vectors.
	createTable := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS documents (
		meta TEXT,
		content TEXT,
		vec FLOAT[%d]
	);`, vecSize)
	if _, err = db.Exec(createTable); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to create documents table: %w", err)
	}

	if _, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_documents_vec ON documents USING HNSW (vec) WITH (metric = 'cosine');`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to create HNSW index: %w", err)
	}

	return &Storage{
		sql:     db,
		vecSize: vecSize,
	}, nil
}

type Document struct {
	Vec     []float32
	Meta    map[string]string
	Content string
}

func (r *Storage) AddDocument(d *Document) error {
	if len(d.Vec) != r.vecSize {
		return fmt.Errorf("vector has length %d, expected %d", len(d.Vec), r.vecSize)
	}

	metaJSON, err := json.Marshal(d.Meta)
	if err != nil {
		return fmt.Errorf("failed to marshal meta: %w", err)
	}

	vecLiteral := formatFloat32ArrayLiteral(d.Vec)
	query := fmt.Sprintf(`INSERT INTO documents (meta, content, vec)
		VALUES (?, ?, %s::FLOAT[%d]);`, vecLiteral, r.vecSize)

	if _, err := r.sql.Exec(query, string(metaJSON), d.Content); err != nil {
		return fmt.Errorf("failed to insert document: %w", err)
	}

	return nil
}

func (r *Storage) Search(query []float32, limit int) ([]Document, error) {
	if r == nil || r.sql == nil {
		return nil, fmt.Errorf("Storage is not initialized")
	}
	if len(query) != r.vecSize {
		return nil, fmt.Errorf("query vector has length %d, expected %d", len(query), r.vecSize)
	}
	if limit <= 0 {
		return []Document{}, nil
	}

	vecLiteral := formatFloat32ArrayLiteral(query)

	// Use array_distance (L2) as described in the DuckDB VSS documentation.
	stmt := fmt.Sprintf(`SELECT  meta, content, vec::TEXT
		FROM documents
		ORDER BY array_distance(vec, %s::FLOAT[%d])
		LIMIT ?;`, vecLiteral, r.vecSize)

	rows, err := r.sql.Query(stmt, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to execute search query: %w", err)
	}
	defer rows.Close()

	var results []Document
	for rows.Next() {
		var (
			metaStr string
			content string
			vecStr  string
		)

		if err := rows.Scan(&metaStr, &content, &vecStr); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		meta := map[string]string{}
		if metaStr != "" {
			if err := json.Unmarshal([]byte(metaStr), &meta); err != nil {
				return nil, fmt.Errorf("failed to unmarshal meta: %w", err)
			}
		}

		vec, err := parseFloat32ArrayLiteral(vecStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse vector: %w", err)
		}

		results = append(results, Document{
			Vec:     vec,
			Meta:    meta,
			Content: content,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("search iteration error: %w", err)
	}

	return results, nil
}

// formatFloat32ArrayLiteral converts a []float32 into a DuckDB FLOAT array literal
// like [1.0, 2.0, 3.0].
func formatFloat32ArrayLiteral(vec []float32) string {
	var b strings.Builder
	b.WriteByte('[')
	for i, f := range vec {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(strconv.FormatFloat(float64(f), 'f', -1, 32))
	}
	b.WriteByte(']')
	return b.String()
}

// parseFloat32ArrayLiteral parses a DuckDB array literal like "[1.0, 2.0, 3.0]"
// into a []float32.
func parseFloat32ArrayLiteral(s string) ([]float32, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return []float32{}, nil
	}
	if len(s) < 2 || s[0] != '[' || s[len(s)-1] != ']' {
		return nil, fmt.Errorf("invalid array literal: %q", s)
	}

	inner := strings.TrimSpace(s[1 : len(s)-1])
	if inner == "" {
		return []float32{}, nil
	}

	parts := strings.Split(inner, ",")
	res := make([]float32, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		v, err := strconv.ParseFloat(p, 32)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %q as float32: %w", p, err)
		}
		res = append(res, float32(v))
	}

	return res, nil
}

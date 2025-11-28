package storage

import (
	"encoding/json"
	"fmt"

	"github.com/doug-martin/goqu/v9"
	_ "github.com/duckdb/duckdb-go/v2"
)

type Document struct {
	Vec      []float32
	Meta     map[string]string
	Content  string
	Store    string
	Distance float32
}

type Store struct {
	Name          string
	DocumentCount int
}

func (s *Storage) AddDocument(d *Document) error {
	if len(d.Vec) != s.vecSize {
		return fmt.Errorf("vector has length %d, expected %d", len(d.Vec), s.vecSize)
	}

	metaJSON, err := json.Marshal(d.Meta)
	if err != nil {
		return fmt.Errorf("failed to marshal meta: %w", err)
	}

	insertQuery := `INSERT INTO documents (document_store, meta, content, vec) 
		VALUES (?, ?, ?, ?::FLOAT[%d]);`

	query := fmt.Sprintf(insertQuery, s.vecSize)

	if _, err := s.goquDB.Exec(query, d.Store, string(metaJSON), d.Content, d.Vec); err != nil {
		return fmt.Errorf("failed to insert document: %w", err)
	}

	return nil
}

func (s *Storage) ListStores() ([]Store, error) {
	ds := s.goquDB.From(goqu.T("documents")).Select("document_store", goqu.COUNT("*").As("document_count")).GroupBy("document_store")

	var rows []struct {
		Name          string `db:"document_store"`
		DocumentCount int    `db:"document_count"`
	}

	if err := ds.ScanStructs(&rows); err != nil {
		return nil, fmt.Errorf("failed to scan stores: %w", err)
	}

	stores := make([]Store, len(rows))
	for i, r := range rows {
		stores[i] = Store{
			Name:          r.Name,
			DocumentCount: r.DocumentCount,
		}
	}

	return stores, nil
}

func (s *Storage) DeleteStore(store string) error {
	_, err := s.goquDB.Exec("DELETE FROM documents WHERE document_store = ?", store)

	return err
}

func (s *Storage) Search(searchVec []float32, store string, limit int) ([]Document, error) {
	if len(searchVec) != s.vecSize {
		return nil, fmt.Errorf("query vector has length %d, expected %d", len(searchVec), s.vecSize)
	}

	if limit <= 0 {
		return []Document{}, nil
	}

	selectQuery := `SELECT  meta, content, vec, document_store, array_distance(vec, ?::FLOAT[%d]) AS distance
		FROM documents
		WHERE document_store = ?
		ORDER BY array_distance(vec, ?::FLOAT[%d]) DESC
		LIMIT ?;`

	stmt := fmt.Sprintf(selectQuery, s.vecSize, s.vecSize)

	rows, err := s.goquDB.Query(stmt, searchVec, store, searchVec, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to execute search query: %w", err)
	}

	defer rows.Close()

	var results []Document
	for rows.Next() {
		var (
			metaAny   map[string]any
			content   string
			vecAny    []any
			storeName string
			distance  float32
		)

		if err := rows.Scan(&metaAny, &content, &vecAny, &storeName, &distance); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		vec := make([]float32, len(vecAny))
		for i, v := range vecAny {
			switch n := v.(type) {
			case float32:
				vec[i] = n
			case float64:
				vec[i] = float32(n)
			case int64:
				vec[i] = float32(n)
			default:
				return nil, fmt.Errorf("unexpected element type %T in vec column", v)
			}
		}

		meta := map[string]string{}
		for k, v := range metaAny {
			meta[k] = fmt.Sprintf("%v", v)
		}

		results = append(results, Document{
			Vec:      vec,
			Meta:     meta,
			Content:  content,
			Store:    storeName,
			Distance: distance,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("search iteration error: %w", err)
	}

	return results, nil
}

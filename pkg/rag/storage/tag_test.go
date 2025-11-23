package storage

import (
	"database/sql"
	"log"
	"testing"

	_ "github.com/duckdb/duckdb-go/v2"
)

func Test_vec(t *testing.T) {
	db, err := sql.Open("duckdb", "")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec("INSTALL vss; LOAD vss;")
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec("CREATE TABLE embeddings (vec FLOAT[3]);")
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec("CREATE INDEX idx ON embeddings USING HNSW (vec);")
	if err != nil {
		log.Fatal(err)
	}
}

func TestRAG_InsertAndSearch_vecSize6(t *testing.T) {
	rag, err := NewRAG("", 6)
	if err != nil {
		t.Fatalf("failed to create Storage: %v", err)
	}

	doc := &Document{
		Vec:     []float32{0, 1, 2, 3, 4, 5},
		Meta:    map[string]string{"k": "v", "name": "doc1"},
		Content: "hello world",
	}

	if err := rag.AddDocument(doc); err != nil {
		t.Fatalf("failed to add document: %v", err)
	}

	results, err := rag.Search([]float32{0, 1, 2, 3, 4, 5}, 1)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Content != doc.Content {
		t.Fatalf("expected content %q, got %q", doc.Content, results[0].Content)
	}
}

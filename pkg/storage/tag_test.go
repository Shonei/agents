package storage

import (
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
		err := os.MkdirAll(filepath.Dir(dbPath), 0o700)
		if err != nil {
			t.Fatalf("failed to create test directory: %v", err)
		}
	} else if err != nil {
		t.Fatalf("unable to check db stats: %v", err)
	} else {
		// we start with a clean DB
		_ = os.Remove(dbPath)
	}

	store, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	defer store.Close()

	testCases := []struct {
		name string
		test func(*testing.T, *Storage)
	}{
		{"add_document", testAddDocument},
		{"search_document", testSearchDocument},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.test(t, store)
		})
	}
}

func testAddDocument(t *testing.T, store *Storage) {
	testVec := make([]float32, SearchVectorSize)

	for i := range testVec {
		testVec[i] = float32(i)
	}

	d := &Document{
		Vec:     testVec,
		Meta:    map[string]string{"test": "test"},
		Content: "test",
	}

	if err := store.AddDocument(d); err != nil {
		t.Errorf("failed to add document: %v", err)
	}
}

func testSearchDocument(t *testing.T, store *Storage) {
	testVec := make([]float32, SearchVectorSize)

	for i := range testVec {
		testVec[i] = float32(i)
	}

	docs, err := store.Search(testVec, "", 10)
	if err != nil {
		t.Errorf("failed to search: %v", err)
	}

	if len(docs) != 1 {
		t.Errorf("expected 1 result, got %d", len(docs))
	}
}

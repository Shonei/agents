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
		{"add_document_invalid_vector", testAddDocumentInvalidVector},
		{"search_invalid_vector", testSearchInvalidVector},
		{"search_zero_limit", testSearchZeroLimit},
		{"list_stores", testListStores},
		{"delete_store", testDeleteStore},
		{"meta_search", testMetaSearch},
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

func testAddDocumentInvalidVector(t *testing.T, store *Storage) {
	shortVec := make([]float32, SearchVectorSize-1)

	d := &Document{
		Vec:     shortVec,
		Meta:    map[string]string{"test": "invalid"},
		Content: "invalid",
		Store:   "store-invalid-add",
	}

	if err := store.AddDocument(d); err == nil {
		t.Errorf("expected error when adding document with invalid vector size, got nil")
	}
}

func testSearchInvalidVector(t *testing.T, store *Storage) {
	shortVec := make([]float32, SearchVectorSize-1)

	if _, err := store.Search(shortVec, "store-invalid-search", 10); err == nil {
		t.Errorf("expected error when searching with invalid vector size, got nil")
	}
}

func testSearchZeroLimit(t *testing.T, store *Storage) {
	vec := make([]float32, SearchVectorSize)

	docs, err := store.Search(vec, "store-zero-limit", 0)
	if err != nil {
		t.Fatalf("expected no error when limit is zero, got %v", err)
	}

	if len(docs) != 0 {
		t.Errorf("expected zero results when limit is zero, got %d", len(docs))
	}
}

func testListStores(t *testing.T, store *Storage) {
	vec := make([]float32, SearchVectorSize)

	storeOne := "store-list-1"
	storeTwo := "store-list-2"

	// add two documents to storeOne
	for i := 0; i < 2; i++ {
		d := &Document{
			Vec:     vec,
			Meta:    map[string]string{"store": "one"},
			Content: "store one document",
			Store:   storeOne,
		}

		if err := store.AddDocument(d); err != nil {
			t.Fatalf("failed to add document to %s: %v", storeOne, err)
		}
	}

	// add one document to storeTwo
	d := &Document{
		Vec:     vec,
		Meta:    map[string]string{"store": "two"},
		Content: "store two document",
		Store:   storeTwo,
	}

	if err := store.AddDocument(d); err != nil {
		t.Fatalf("failed to add document to %s: %v", storeTwo, err)
	}

	stores, err := store.ListStores()
	if err != nil {
		t.Fatalf("failed to list stores: %v", err)
	}

	counts := make(map[string]int)
	for _, s := range stores {
		counts[s.Name] = s.DocumentCount
	}

	if counts[storeOne] != 2 {
		t.Errorf("expected %s to have 2 documents, got %d", storeOne, counts[storeOne])
	}

	if counts[storeTwo] != 1 {
		t.Errorf("expected %s to have 1 document, got %d", storeTwo, counts[storeTwo])
	}
}

func testDeleteStore(t *testing.T, store *Storage) {
	vec := make([]float32, SearchVectorSize)

	targetStore := "store-delete-target"
	otherStore := "store-delete-other"

	// add documents to the target store
	for i := 0; i < 2; i++ {
		d := &Document{
			Vec:     vec,
			Meta:    map[string]string{"store": "delete"},
			Content: "to delete",
			Store:   targetStore,
		}

		if err := store.AddDocument(d); err != nil {
			t.Fatalf("failed to add document to target store: %v", err)
		}
	}

	// add a document to a different store that should not be deleted
	d := &Document{
		Vec:     vec,
		Meta:    map[string]string{"store": "keep"},
		Content: "to keep",
		Store:   otherStore,
	}

	if err := store.AddDocument(d); err != nil {
		t.Fatalf("failed to add document to other store: %v", err)
	}

	if err := store.DeleteStore(targetStore); err != nil {
		t.Fatalf("failed to delete store: %v", err)
	}

	stores, err := store.ListStores()
	if err != nil {
		t.Fatalf("failed to list stores after delete: %v", err)
	}

	var (
		foundTarget bool
		foundOther  bool
	)

	for _, s := range stores {
		if s.Name == targetStore {
			foundTarget = true
		}
		if s.Name == otherStore {
			foundOther = true
		}
	}

	if foundTarget {
		t.Errorf("expected target store %q to be deleted", targetStore)
	}

	if !foundOther {
		t.Errorf("expected other store %q to still exist", otherStore)
	}
}

func testMetaSearch(t *testing.T, store *Storage) {
	vec := make([]float32, SearchVectorSize)

	storeName := "store-meta-search"
	otherStore := "store-meta-search-other"

	// Add doc to main store
	d1 := &Document{
		Vec:     vec,
		Meta:    map[string]string{"type": "test", "id": "1"},
		Content: "content 1",
		Store:   storeName,
	}
	if err := store.AddDocument(d1); err != nil {
		t.Fatalf("failed to add doc 1: %v", err)
	}

	// Add doc to other store with same meta
	d2 := &Document{
		Vec:     vec,
		Meta:    map[string]string{"type": "test", "id": "2"},
		Content: "content 2",
		Store:   otherStore,
	}
	if err := store.AddDocument(d2); err != nil {
		t.Fatalf("failed to add doc 2: %v", err)
	}

	// Test 1: Search without store filter (should find both)
	results, err := store.MetaSearch(map[string]string{"type": "test"}, "")
	if err != nil {
		t.Fatalf("failed to search meta without store: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results without store filter, got %d", len(results))
	}

	// Test 2: Search with store filter (should find one)
	results, err = store.MetaSearch(map[string]string{"type": "test"}, storeName)
	if err != nil {
		t.Fatalf("failed to search meta with store: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result with store filter, got %d", len(results))
	}
	if len(results) > 0 && results[0].Store != storeName {
		t.Errorf("expected result from store %s, got %s", storeName, results[0].Store)
	}
}
}

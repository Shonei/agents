package utils

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChunkGoCode(t *testing.T) {
	content := `package main

import "fmt"

// User represents a user
type User struct {
	Name string
}

// NewUser creates a user
// It takes a name
func NewUser(name string) *User {
	return &User{Name: name}
}

func (u *User) Greet() {
	fmt.Println("Hello " + u.Name)
}
`
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.go")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	chunks, err := Chunk(tmpFile, ChunkingStrategyHeuristic)
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	// Expected chunks:
	// 1. package main + imports
	// 2. type User (with comment)
	// 3. func NewUser (with comment)
	// 4. func Greet
	
	// The implementation might group package and import together or separate them depending on newlines.
	// Let's analyze the expected behavior of chunkGoCode:
	// "package main" starts with 'p' -> top level.
	// "import ..." starts with 'i' -> top level? No, logic is: 
	// (line[0] >= 'a' && line[0] <= 'z') && (prefix func/type/const/var)
	// "package" and "import" are NOT in the list (func, type, const, var).
	// So "package main" and "import" will likely remain in the "currentChunk" until the first "type User" is encountered.
	
	// So Chunk 1: package ... import ...
	// Chunk 2: type User ...
	// Chunk 3: func NewUser ...
	// Chunk 4: func (u *User) ... -> Wait, receiver methods start with 'func '. Correct.

	if len(chunks) != 4 {
		t.Errorf("expected 4 chunks, got %d", len(chunks))
		for i, c := range chunks {
			t.Logf("Chunk %d:\n%s\n---", i, c)
		}
	}

	// Check content of chunks
	if !strings.Contains(chunks[1], "// User represents a user") {
		t.Errorf("Chunk 1 should contain comment for User struct")
	}
	if !strings.Contains(chunks[1], "type User struct") {
		t.Errorf("Chunk 1 should contain User struct definition")
	}

	if !strings.Contains(chunks[2], "// NewUser creates a user") {
		t.Errorf("Chunk 2 should contain comment for NewUser function")
	}
	if !strings.Contains(chunks[2], "func NewUser") {
		t.Errorf("Chunk 2 should contain NewUser function")
	}

	if !strings.Contains(chunks[3], "func (u *User) Greet()") {
		t.Errorf("Chunk 3 should contain Greet method")
	}
}

func TestChunkDefault(t *testing.T) {
	content := `First paragraph.
With multiple lines.

Second paragraph.

Third paragraph.`
	
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	chunks, err := Chunk(tmpFile, ChunkingStrategyHeuristic)
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	if len(chunks) != 3 {
		t.Errorf("expected 3 chunks, got %d", len(chunks))
	}

	if chunks[0] != "First paragraph.\nWith multiple lines." {
		t.Errorf("unexpected chunk 0: %q", chunks[0])
	}
	if chunks[1] != "Second paragraph." {
		t.Errorf("unexpected chunk 1: %q", chunks[1])
	}
}

func TestChunkNone(t *testing.T) {
	content := "This is a\nfile content\nwith multiple lines."
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test_none.txt")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	chunks, err := Chunk(tmpFile, ChunkingStrategyNone)
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	if len(chunks) != 1 {
		t.Errorf("expected 1 chunk, got %d", len(chunks))
	}

	if chunks[0] != content {
		t.Errorf("expected chunk content to match file content")
	}
}

func TestChunkFixedSize(t *testing.T) {
	// Create content of 50 characters: "0123456789" repeated 5 times
	content := ""
	for i := 0; i < 5; i++ {
		content += "0123456789"
	}

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test_fixed.txt")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	chunks, err := Chunk(tmpFile, ChunkingStrategyFixedSize)
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	if len(chunks) != 1 {
		t.Errorf("expected 1 chunk (since content < 2000), got %d", len(chunks))
	}
}

func TestChunkFixedSizeLogic(t *testing.T) {
	content := "012345678901234567890123456789" // 30 chars
	size := 10
	overlap := 5

	chunks := chunkFixedSize(content, size, overlap)

	expectedCount := 5
	// 0: 0-10 "0123456789" (next 5)
	// 1: 5-15 "5678901234" (next 10)
	// 2: 10-20 "0123456789" (next 15)
	// 3: 15-25 "5678901234" (next 20)
	// 4: 20-30 "0123456789" (next 25)
	// Loop check:
	// start=20, end=30. chunk added. end==len(content) -> break.
	// So 5 chunks.

	if len(chunks) != expectedCount {
		t.Errorf("expected %d chunks, got %d", expectedCount, len(chunks))
		for i, c := range chunks {
			t.Logf("Chunk %d: %q", i, c)
		}
	}

	if len(chunks) > 0 {
		expectedFirst := "0123456789"
		if chunks[0] != expectedFirst {
			t.Errorf("expected first chunk %q, got %q", expectedFirst, chunks[0])
		}
	}
	
	if len(chunks) > 1 {
		expectedSecond := "5678901234"
		if chunks[1] != expectedSecond {
			t.Errorf("expected second chunk %q, got %q", expectedSecond, chunks[1])
		}
	}
}

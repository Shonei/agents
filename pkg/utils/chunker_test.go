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
	if err := os.WriteFile(tmpFile, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	chunks, err := Chunk(tmpFile)
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
	if err := os.WriteFile(tmpFile, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	chunks, err := Chunk(tmpFile)
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

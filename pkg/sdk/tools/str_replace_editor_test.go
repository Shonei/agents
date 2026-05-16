package tools

import (
	"os"
	"path/filepath"
	"testing"
)

var testContest = `line 1
line 2
line 3
line 4
line 5
line 6
line 7
line 8
line 9
line 10
line 11
line 12
line 13
line 14
line 15
line 16
line 17
line 18
line 19
line 20
line 21`

func TestEditFile_replace(t *testing.T) {
	strReplaceEditor := &StrReplaceEditorTool{}

	cwd, err := os.Getwd()
	if err != nil {
		t.Errorf("failed to get current directory: %v", err)
	}

	path := filepath.Join(cwd, "testfiles/test.txt")

	err = os.WriteFile(path, []byte(testContest), 0o600)
	if err != nil {
		t.Errorf("failed to write test file: %v", err)
	}

	input := map[string]interface{}{
		"command": "str_replace",
		"path":    path,
		"old_str": "line 4\nline 5\nline 6\nline 7\n",
		"new_str": "new\n",
	}

	result, err := strReplaceEditor.Call(input)
	if err != nil {
		t.Errorf("failed to edit file: %v", err)
	}

	t.Log(result)
}

func TestEditFile_replace_new_lines(t *testing.T) {
	strReplaceEditor := &StrReplaceEditorTool{}
	testContent := `package tools

import (
	"fmt"
	"strings"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/Shonei/agents/pkg/utils"
)
`

	cwd, err := os.Getwd()
	if err != nil {
		t.Errorf("failed to get current directory: %v", err)
	}

	path := filepath.Join(cwd, "testfiles/new_lines.txt")

	err = os.WriteFile(path, []byte(testContent), 0o600)
	if err != nil {
		t.Errorf("failed to write test file: %v", err)
	}

	input := map[string]interface{}{
		"command": "str_replace",
		"path":    path,
		"old_str": "\t\"fmt\"\n\t\"strings\"\n",
		"new_str": "\t\"fmt\"\n\t\"os\"\n\t\"strings\"\n",
	}

	result, err := strReplaceEditor.Call(input)
	if err != nil {
		t.Errorf("failed to edit file: %v", err)
	}

	t.Log(result)
}

func TestEditFile_insert(t *testing.T) {
	strReplaceEditor := &StrReplaceEditorTool{}

	cwd, err := os.Getwd()
	if err != nil {
		t.Errorf("failed to get current directory: %v", err)
	}

	path := filepath.Join(cwd, "testfiles/insert.txt")

	err = os.WriteFile(path, []byte(testContest), 0o600)
	if err != nil {
		t.Errorf("failed to write test file: %v", err)
	}

	input := map[string]interface{}{
		"command":     "insert",
		"path":        path,
		"insert_line": 4,
		"new_str":     "new\n",
	}

	result, err := strReplaceEditor.Call(input)
	if err != nil {
		t.Errorf("failed to edit file: %v", err)
	}

	t.Log(result)
}

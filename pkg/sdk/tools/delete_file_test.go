package tools

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeleteFileToolDeletesFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "obsolete.md")
	require.NoError(t, os.WriteFile(path, []byte("remove me"), 0o600))

	tool := &DeleteFileTool{}
	tool.Init(map[string]string{
		"require_confirmation": "false",
		"allowed_root":         root,
	}, nil)

	result, err := tool.Call(map[string]interface{}{"path": path})
	require.NoError(t, err)
	assert.Contains(t, result, "File deleted:")

	_, err = os.Stat(path)
	assert.True(t, os.IsNotExist(err))
}

func TestDeleteFileToolIgnoreMissing(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "missing.md")

	tool := &DeleteFileTool{}
	tool.Init(map[string]string{
		"require_confirmation": "false",
		"allowed_root":         root,
	}, nil)

	result, err := tool.Call(map[string]interface{}{
		"path":           path,
		"ignore_missing": true,
	})
	require.NoError(t, err)
	assert.Contains(t, result, "File already absent:")
}

func TestDeleteFileToolRefusesDirectories(t *testing.T) {
	root := t.TempDir()

	tool := &DeleteFileTool{}
	tool.Init(map[string]string{
		"require_confirmation": "false",
		"allowed_root":         root,
	}, nil)

	_, err := tool.Call(map[string]interface{}{"path": root})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "refuses to delete directories")
}

func TestDeleteFileToolRefusesOutsideAllowedRoot(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "important.txt")
	require.NoError(t, os.WriteFile(outside, []byte("do not delete"), 0o600))

	tool := &DeleteFileTool{}
	tool.Init(map[string]string{
		"require_confirmation": "false",
		"allowed_root":         root,
	}, nil)

	_, err := tool.Call(map[string]interface{}{"path": outside})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside allowed_root")
	assert.FileExists(t, outside)
}

func TestToolNamesIncludesDeleteFile(t *testing.T) {
	assert.Contains(t, ToolNames(), "delete_file")
}

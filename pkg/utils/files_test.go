package utils

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// collected indexes a CollectFiles result by path for easier assertions.
func collected(t *testing.T, files []File) map[string]string {
	t.Helper()

	byPath := make(map[string]string, len(files))
	for _, f := range files {
		byPath[f.Path] = f.Content
	}

	return byPath
}

func TestCollectFilesGathersTextFiles(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(dir, "top.txt"), []byte("top"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sub", "deep"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sub", "deep", "nested.go"), []byte("package deep"), 0o600))

	files, err := CollectFiles(dir, false)
	require.NoError(t, err)

	byPath := collected(t, files)

	assert.Equal(t, "top", byPath["top.txt"])
	// Paths are relative to the collected directory and always slash-separated.
	assert.Equal(t, "package deep", byPath["sub/deep/nested.go"])
	assert.Len(t, files, 2)
}

func TestCollectFilesDryRunOmitsContent(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("payload"), 0o600))

	files, err := CollectFiles(dir, true)
	require.NoError(t, err)

	require.Len(t, files, 1)
	assert.Equal(t, "a.txt", files[0].Path)
	assert.Empty(t, files[0].Content)
}

func TestCollectFilesSkipsBinaryAndSkipDirs(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(dir, "text.txt"), []byte("fine"), 0o600))
	// A NUL byte is what the sniffer treats as binary.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "blob.bin"), []byte{'a', 0x00, 'b'}, 0o600))

	require.NoError(t, os.MkdirAll(filepath.Join(dir, "node_modules"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "node_modules", "dep.js"), []byte("dep"), 0o600))

	files, err := CollectFiles(dir, false)
	require.NoError(t, err)

	byPath := collected(t, files)

	assert.Contains(t, byPath, "text.txt")
	assert.NotContains(t, byPath, "blob.bin", "binary files should be skipped")
	assert.NotContains(t, byPath, "node_modules/dep.js", "SkipDirs entries should be pruned")
}

func TestCollectFilesRespectsGitignore(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("ignored.txt\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ignored.txt"), []byte("nope"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "kept.txt"), []byte("yes"), 0o600))

	files, err := CollectFiles(dir, false)
	require.NoError(t, err)

	byPath := collected(t, files)

	assert.Contains(t, byPath, "kept.txt")
	assert.NotContains(t, byPath, "ignored.txt")
}

// TestCollectFilesSkipsEscapingSymlink covers the security property: a symlink
// inside the tree must not pull in content from outside it. Just as important,
// encountering one must not abort the whole collection — a single stray symlink
// in a real repo would otherwise break indexing entirely.
func TestCollectFilesSkipsEscapingSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation is not reliably available on Windows CI")
	}

	outside := t.TempDir()
	secretPath := filepath.Join(outside, "secret.txt")
	require.NoError(t, os.WriteFile(secretPath, []byte("SENSITIVE"), 0o600))

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ok.txt"), []byte("ok"), 0o600))
	require.NoError(t, os.Symlink(secretPath, filepath.Join(dir, "leak.txt")))

	files, err := CollectFiles(dir, false)
	require.NoError(t, err, "an escaping symlink must be skipped, not fatal")

	byPath := collected(t, files)

	assert.Equal(t, "ok", byPath["ok.txt"], "regular files are still collected")
	assert.NotContains(t, byPath, "leak.txt")

	for _, f := range files {
		assert.NotContains(t, f.Content, "SENSITIVE",
			"content from outside the collected directory must not be read via %s", f.Path)
	}
}

// TestCollectFilesSkipsInTreeSymlink documents that symlinks are skipped even
// when their target is inside the tree: the target itself is walked, so
// following the link would just duplicate it.
func TestCollectFilesSkipsInTreeSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation is not reliably available on Windows CI")
	}

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "real.txt"), []byte("body"), 0o600))
	require.NoError(t, os.Symlink(filepath.Join(dir, "real.txt"), filepath.Join(dir, "alias.txt")))

	files, err := CollectFiles(dir, false)
	require.NoError(t, err)

	byPath := collected(t, files)

	assert.Equal(t, "body", byPath["real.txt"])
	assert.NotContains(t, byPath, "alias.txt")
}

func TestIsBinaryFile(t *testing.T) {
	dir := t.TempDir()

	textPath := filepath.Join(dir, "text.txt")
	require.NoError(t, os.WriteFile(textPath, []byte("plain text"), 0o600))

	binPath := filepath.Join(dir, "blob.bin")
	require.NoError(t, os.WriteFile(binPath, []byte{'a', 0x00}, 0o600))

	emptyPath := filepath.Join(dir, "empty.txt")
	require.NoError(t, os.WriteFile(emptyPath, nil, 0o600))

	isBin, err := IsBinaryFile(textPath)
	require.NoError(t, err)
	assert.False(t, isBin)

	isBin, err = IsBinaryFile(binPath)
	require.NoError(t, err)
	assert.True(t, isBin)

	isBin, err = IsBinaryFile(emptyPath)
	require.NoError(t, err)
	assert.False(t, isBin, "an empty file is not binary")

	_, err = IsBinaryFile(filepath.Join(dir, "missing.txt"))
	assert.Error(t, err)
}

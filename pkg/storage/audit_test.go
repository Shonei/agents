package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/doug-martin/goqu/v9"
	_ "github.com/duckdb/duckdb-go/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuditStorage(t *testing.T) {
	cwd, err := os.Getwd()
	require.NoError(t, err)

	dbPath := filepath.Join(cwd, "testfiles/audit_test.db")

	_, err = os.Stat(dbPath)
	if os.IsNotExist(err) {
		err := os.MkdirAll(filepath.Dir(dbPath), 0o700)
		require.NoError(t, err)
	} else {
		_ = os.Remove(dbPath)
	}

	store, err := NewStorage(dbPath)
	require.NoError(t, err)
	defer store.Close()

	t.Run("SaveSession", func(t *testing.T) {
		id := "session-1"
		hash := "hash-1"
		prompt := "system prompt"

		err := store.SaveSession(id, hash, prompt, "", []string{"browse_url"}, []string{"google_search", "url_context"})
		require.NoError(t, err)

		// Verify session was saved
		count, err := store.goquDB.From("audit_sessions").Where(goqu.Ex{"id": id}).Count()
		require.NoError(t, err)
		assert.Equal(t, int64(1), count)

		type Session struct {
			ID           string `db:"id"`
			SessionHash  string `db:"session_hash"`
			SystemPrompt string `db:"system_prompt"`
			Tools        string `db:"tools"`
			ServerTools  string `db:"server_tools"`
		}
		var session Session
		found, err := store.goquDB.From("audit_sessions").Where(goqu.Ex{"id": id}).ScanStruct(&session)
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, id, session.ID)
		assert.Equal(t, hash, session.SessionHash)
		assert.Equal(t, prompt, session.SystemPrompt)
		assert.JSONEq(t, `["browse_url"]`, session.Tools)
		assert.JSONEq(t, `["google_search","url_context"]`, session.ServerTools)
	})

	t.Run("SaveEvent", func(t *testing.T) {
		sessionID := "session-1"
		eventID := "event-1"
		eventType := "user_message"
		content := "hello"
		payload := map[string]string{"foo": "bar"}
		payloadBytes, _ := json.Marshal(payload)

		err := store.SaveEvent(eventID, sessionID, eventType, content, payloadBytes)
		require.NoError(t, err)

		type Event struct {
			ID        string         `db:"id"`
			SessionID string         `db:"session_id"`
			Type      string         `db:"type"`
			Content   string         `db:"content"`
			Payload   map[string]any `db:"payload"`
		}
		var event Event
		found, err := store.goquDB.From("audit_events").Where(goqu.Ex{"id": eventID}).ScanStruct(&event)
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, eventID, event.ID)
		assert.Equal(t, sessionID, event.SessionID)
		assert.Equal(t, eventType, event.Type)
		assert.Equal(t, content, event.Content)

		// Convert original payload to map[string]any for comparison
		expectedPayload := map[string]any{"foo": "bar"}
		assert.Equal(t, expectedPayload, event.Payload)
	})

	t.Run("SaveEvent_EmptyPayload", func(t *testing.T) {
		sessionID := "session-1"
		eventID := "event-2"
		eventType := "test"
		content := "test"

		err := store.SaveEvent(eventID, sessionID, eventType, content, nil)
		require.NoError(t, err)

		type Event struct {
			Payload map[string]any `db:"payload"`
		}
		var event Event
		found, err := store.goquDB.From("audit_events").Where(goqu.Ex{"id": eventID}).ScanStruct(&event)
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, map[string]any{}, event.Payload)
	})
}

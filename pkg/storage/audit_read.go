package storage

import (
	"database/sql"
	"fmt"
	"time"
)

// AuditSession is a single recorded conversation. One session is created per
// audit.Audit.User call, i.e. per top-level agent run (router sub-agents share
// the parent's session).
type AuditSession struct {
	ID              string         `db:"id"`
	SessionHash     string         `db:"session_hash"`
	SystemPrompt    string         `db:"system_prompt"`
	Tools           sql.NullString `db:"tools"`
	ServerTools     sql.NullString `db:"server_tools"`
	ParentSessionID sql.NullString `db:"parent_session_id"`
	CreatedAt       time.Time      `db:"created_at"`
}

// AuditSessionSummary is an AuditSession plus the aggregate counts needed to
// pick one out of a list without loading its events.
type AuditSessionSummary struct {
	ID         string    `db:"id"`
	CreatedAt  time.Time `db:"created_at"`
	Events     int       `db:"event_count"`
	ToolCalls  int       `db:"tool_call_count"`
	Compaction int       `db:"compaction_count"`
	Handoffs   int       `db:"handoff_count"`
}

// AuditEvent is one recorded event within a session, in insertion order.
type AuditEvent struct {
	ID        string         `db:"id"`
	SessionID string         `db:"session_id"`
	Type      string         `db:"type"`
	Content   sql.NullString `db:"content"`
	Payload   sql.NullString `db:"payload"`
	CreatedAt time.Time      `db:"created_at"`
}

// ListAuditSessions returns the most recent sessions, newest first. A limit of
// zero or less returns every session.
func (s *Storage) ListAuditSessions(limit int) ([]AuditSessionSummary, error) {
	query := `SELECT s.id,
	                 s.created_at,
	                 COALESCE(e.n, 0)  AS event_count,
	                 COALESCE(tc.n, 0) AS tool_call_count,
	                 COALESCE(c.n, 0)  AS compaction_count,
	                 COALESCE(h.n, 0)  AS handoff_count
	          FROM audit_sessions s
	          LEFT JOIN (SELECT session_id, COUNT(*) AS n FROM audit_events GROUP BY session_id) e
	            ON e.session_id = s.id
	          LEFT JOIN (SELECT session_id, COUNT(*) AS n FROM audit_events WHERE type = 'function_call' GROUP BY session_id) tc
	            ON tc.session_id = s.id
	          LEFT JOIN (SELECT session_id, COUNT(*) AS n FROM audit_events WHERE type = 'compaction' GROUP BY session_id) c
	            ON c.session_id = s.id
	          LEFT JOIN (SELECT session_id, COUNT(*) AS n FROM audit_events WHERE type = 'handoff' GROUP BY session_id) h
	            ON h.session_id = s.id
	          ORDER BY s.created_at DESC`

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	var rows []AuditSessionSummary
	if err := s.goquDB.ScanStructs(&rows, query); err != nil {
		return nil, fmt.Errorf("failed to list audit sessions: %w", err)
	}

	return rows, nil
}

// GetAuditSession loads a single session by exact ID, by ID prefix, or by ID
// suffix.
//
// Suffix matching matters: a session ID is "<sha256 of system prompt>_<salt>",
// so every run of the same agent shares a long common prefix and only the
// trailing salt is unique. `agents context list` therefore shows the salt as
// the session handle.
func (s *Storage) GetAuditSession(id string) (*AuditSession, error) {
	var rows []AuditSession

	query := `SELECT id, session_hash, system_prompt, tools, server_tools, parent_session_id, created_at
	          FROM audit_sessions
	          WHERE id = ? OR starts_with(id, ?) OR ends_with(id, ?)
	          ORDER BY created_at DESC`

	if err := s.goquDB.ScanStructs(&rows, query, id, id, id); err != nil {
		return nil, fmt.Errorf("failed to load audit session: %w", err)
	}

	switch len(rows) {
	case 0:
		return nil, fmt.Errorf("no audit session matching %q", id)
	case 1:
		return &rows[0], nil
	default:
		// An exact match wins over prefix matches so a full ID is never
		// ambiguous even when it prefixes another.
		for i := range rows {
			if rows[i].ID == id {
				return &rows[i], nil
			}
		}

		return nil, fmt.Errorf("%q matches %d sessions, use a longer prefix", id, len(rows))
	}
}

// GetAuditEvents returns every event for a session in chronological order.
func (s *Storage) GetAuditEvents(sessionID string) ([]AuditEvent, error) {
	// payload is a DuckDB JSON column; the driver surfaces it as a map, so cast
	// it to text and let callers decode it themselves.
	query := `SELECT id, session_id, type, content, CAST(payload AS VARCHAR) AS payload, created_at
	          FROM audit_events
	          WHERE session_id = ?
	          ORDER BY created_at ASC`

	var rows []AuditEvent
	if err := s.goquDB.ScanStructs(&rows, query, sessionID); err != nil {
		return nil, fmt.Errorf("failed to load audit events: %w", err)
	}

	return rows, nil
}

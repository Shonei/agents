package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/Shonei/agents/pkg/utils"
)

const (
	AuditTypeFile     = "file"
	AuditTypeDatabase = "database"

	InitialMessageEvent   = "initial_message"
	UserMessageEvent      = "user_message"
	AssistantMessageEvent = "assistant_message"
	FunctionCallEvent     = "function_call"
	FunctionResponseEvent = "function_response"
	CompactionEvent       = "compaction"
	GroundingEvent        = "grounding"
)

type Logger interface {
	LogEvent(event Event)
	LogUser(user User)
}

type AuditStore interface {
	SaveSession(id string, hash string, prompt string) error
	SaveEvent(id string, sessionID string, eventType string, content string, payload []byte) error
}

type Audit struct {
	logger Logger
}

type Event struct {
	Type             string `json:"type"`
	Content          string `json:"content,omitempty"`
	FunctionCall     any    `json:"function_call,omitempty"`
	FunctionResponse any    `json:"function_response,omitempty"`
	InitialMessage   any    `json:"initial_message,omitempty"`
}

type User struct {
	ID           string `json:"id"`
	SystemPrompt string `json:"system_prompt"`
}

type AuditConfig struct {
	Enabled   bool   `yaml:"enabled"`
	AuditType string `yaml:"type"`
	AuditPath string `yaml:"path"`
}

func NewAudit(logger Logger) *Audit {
	return &Audit{
		logger: logger,
	}
}

func (a *Audit) LogEvent(event Event) {
	a.logger.LogEvent(event)
}

func (a *Audit) User(p string) {
	user := User{
		SystemPrompt: p,
	}

	idHash := sha256.New()
	idHash.Write([]byte(user.SystemPrompt))
	user.ID = hex.EncodeToString(idHash.Sum(nil))

	a.logger.LogUser(user)
}

func NewFileLogger(path string) (Logger, error) {
	stats, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat audit path: %w", err)
	}

	if !stats.IsDir() {
		return nil, fmt.Errorf("audit path must be a directory")
	}

	return &fileLogger{
		path: path,
	}, nil
}

type fileLogger struct {
	path      string
	auditName string
}

func (f *fileLogger) LogEvent(event Event) {
	b, err := json.Marshal(event)
	if err != nil {
		return
	}

	f.appendToFile(append(b, '\n'))
}

func (f *fileLogger) LogUser(user User) {
	salt := time.Now().Unix()

	f.auditName = fmt.Sprintf("%s_%d.json", user.ID, salt)

	b, err := json.Marshal(user)
	if err != nil {
		return
	}

	f.appendToFile(append(b, '\n'))
}

func (f *fileLogger) appendToFile(data []byte) {
	if f.auditName == "" {
		f.auditName = "unknown.json"
	}

	fullPath := f.path + "/" + f.auditName

	af, err := os.OpenFile(fullPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		utils.NewExitError().WithMessage("failed to open audit file").WithReason(err).Done()
	}

	_, err = af.Write(data)
	if err != nil {
		utils.NewExitError().WithMessage("failed to write to audit file").WithReason(err).Done()
	}

	err = af.Close()
	if err != nil {
		utils.NewExitError().WithMessage("failed to close audit file").WithReason(err).Done()
	}
}

func NewNoopLogger() Logger {
	return &noopAudit{}
}

type noopAudit struct{}

func (a *noopAudit) LogEvent(event Event) {
}

func (a *noopAudit) LogUser(user User) {
}

func NewDBLogger(store AuditStore) Logger {
	return &dbLogger{
		store: store,
	}
}

type dbLogger struct {
	store     AuditStore
	sessionID string
}

func (d *dbLogger) LogEvent(event Event) {
	if d.sessionID == "" {
		return
	}

	var payload any
	if event.FunctionCall != nil {
		payload = event.FunctionCall
	} else if event.FunctionResponse != nil {
		payload = event.FunctionResponse
	} else if event.InitialMessage != nil {
		payload = event.InitialMessage
	}

	b, _ := json.Marshal(payload)

	id := utils.RandomString(32)
	_ = d.store.SaveEvent(id, d.sessionID, event.Type, event.Content, b)
}

func (d *dbLogger) LogUser(user User) {
	salt := time.Now().UnixNano()
	d.sessionID = fmt.Sprintf("%s_%d", user.ID, salt)

	_ = d.store.SaveSession(d.sessionID, user.ID, user.SystemPrompt)
}

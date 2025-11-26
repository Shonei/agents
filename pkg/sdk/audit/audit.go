package audit

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/Shonei/agents/pkg/utils"
)

const (
	AuditTypeFile = "file"

	InitialMessageEvent   = "initial_message"
	UserMessageEvent      = "user_message"
	AssistantMessageEvent = "assistant_message"
	FunctionCallEvent     = "function_call"
	FunctionResponseEvent = "function_response"
)

type logger interface {
	logEvent(event Event)
	user(user User)
}
type Audit struct {
	logger logger
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

func NewAudit(c AuditConfig) *Audit {
	if !c.Enabled {
		return &Audit{
			logger: &noopAudit{},
		}
	}

	switch c.AuditType {
	case AuditTypeFile:
		if c.AuditPath == "" {
			utils.NewExitError().WithMessage("audit path is required when audit type is file").Done()
		}

		stats, err := os.Stat(c.AuditPath)
		if err != nil {
			utils.NewExitError().WithMessage("failed to stat audit path").WithReason(err).Done()
		}

		if !stats.IsDir() {
			utils.NewExitError().WithMessage("audit path must be a directory").Done()
		}

		return &Audit{
			logger: &fileLogger{
				path: c.AuditPath,
			},
		}
	default:
		utils.NewExitError().WithMessage("unsupported audit type: " + c.AuditType).Done()
	}

	// unreachable
	// the default error results in an os.Exit(1)
	return &Audit{
		logger: &noopAudit{},
	}
}

func (a *Audit) LogEvent(event Event) {
	a.logger.logEvent(event)
}

func (a *Audit) User(p string) {
	user := User{
		SystemPrompt: p,
	}

	idHash := sha256.New()
	idHash.Write([]byte(user.SystemPrompt))
	user.ID = string(idHash.Sum(nil))

	a.logger.user(user)
}

type fileLogger struct {
	path      string
	auditName string
}

func (f *fileLogger) logEvent(event Event) {
	b, err := json.Marshal(event)
	if err != nil {
		return
	}

	f.appendToFile(append(b, '\n'))
}

func (f *fileLogger) logUser(user User) {
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

func (f *fileLogger) user(user User) {
}

type noopAudit struct{}

func (a *noopAudit) logEvent(event Event) {
}

func (a *noopAudit) user(user User) {
}

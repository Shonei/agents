package context

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/Shonei/agents/pkg/config"
	"github.com/Shonei/agents/pkg/utils"
)

func init() {
	utils.RegisterResource(sessionRow{}, []string{"Session", "Prompt", "Started", "Events", "ToolCalls", "Compactions", "Handoffs"})
}

// sessionRow is the display shape for a recorded conversation.
//
// A session ID is "<sha256 of system prompt>_<nanosecond salt>", so the prefix
// is shared by every run of the same agent and only the salt distinguishes
// them. Session therefore shows the salt (a unique handle that compact/handoff
// accept), and Prompt shows a short hash prefix so runs of the same agent are
// still recognisable as such.
type sessionRow struct {
	Session     string `json:"session" yaml:"session"`
	Prompt      string `json:"prompt" yaml:"prompt"`
	Started     string `json:"started" yaml:"started"`
	Events      int    `json:"events" yaml:"events"`
	ToolCalls   int    `json:"tool_calls" yaml:"tool_calls"`
	Compactions int    `json:"compactions" yaml:"compactions"`
	Handoffs    int    `json:"handoffs" yaml:"handoffs"`
}

type listCommand struct {
	configFactory *config.ConfigFactory
	limit         int
	full          bool
}

// NewListCommand builds `agents context list`.
func NewListCommand(c *config.ConfigFactory) *cobra.Command {
	l := &listCommand{configFactory: c}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List recorded conversations from the audit log",
		Long: "List recorded conversations from the audit log, newest first.\n\n" +
			"Requires database audit logging (audit.type: database) and db_path in your config.\n" +
			"The IDs shown here can be passed to `agents context compact` and `agents context handoff`.",
		Run:  l.Run,
		Args: cobra.NoArgs,
	}

	flags := cmd.Flags()
	flags.IntVar(&l.limit, "limit", 20, "maximum number of sessions to show (0 for all)")
	flags.BoolVar(&l.full, "full", false, "show full session IDs instead of a short prefix")

	return cmd
}

func (l *listCommand) Run(cmd *cobra.Command, args []string) {
	l.configFactory.LoadConfig()

	sessions, err := l.configFactory.GetDB().ListAuditSessions(l.limit)
	if err != nil {
		utils.NewExitError().WithMessage("failed to list sessions").WithReason(err).Done()
	}

	rows := make([]sessionRow, 0, len(sessions))
	for _, s := range sessions {
		handle, promptHash := splitID(s.ID)
		if l.full {
			handle = s.ID
		}

		rows = append(rows, sessionRow{
			Session:     handle,
			Prompt:      promptHash,
			Started:     s.CreatedAt.Format("2006-01-02 15:04"),
			Events:      s.Events,
			ToolCalls:   s.ToolCalls,
			Compactions: s.Compaction,
			Handoffs:    s.Handoffs,
		})
	}

	l.configFactory.Print(rows)
}

// splitID splits a session ID into its unique handle (the trailing salt) and a
// short prefix of the system-prompt hash. IDs that don't follow the
// "<hash>_<salt>" shape are returned unchanged as the handle.
func splitID(id string) (handle, promptHash string) {
	i := strings.LastIndex(id, "_")
	if i < 0 {
		return id, ""
	}

	promptHash = id[:i]
	if len(promptHash) > 8 {
		promptHash = promptHash[:8]
	}

	return id[i+1:], promptHash
}

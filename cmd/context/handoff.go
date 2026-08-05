package context

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Shonei/agents/pkg/config"
	"github.com/Shonei/agents/pkg/sdk"
	"github.com/Shonei/agents/pkg/utils"
)

type handoffCommand struct {
	configFactory *config.ConfigFactory
	agentName     string
	model         string
	from          string
	nextMessage   string
	showInput     bool
	showPrompt    bool
}

// NewHandoffCommand builds `agents context handoff`.
func NewHandoffCommand(c *config.ConfigFactory) *cobra.Command {
	hc := &handoffCommand{configFactory: c}

	cmd := &cobra.Command{
		Use:   "handoff <session_id>",
		Short: "Replay a recorded conversation through a router handoff",
		Long: "Replay a recorded conversation through a router handoff and show what the incoming\n" +
			"sub-agent would start from: the briefing produced and the history it is injected into.\n\n" +
			"Handoff briefs a DIFFERENT agent, with a different system prompt and different tools, and\n" +
			"evicts the whole history. Compare with `agents context compact`, which continues the SAME\n" +
			"agent. Both currently share one summarizer prompt, which is the thing worth eyeballing.\n\n" +
			"This makes one real model call to generate the briefing.",
		Run:  hc.Run,
		Args: cobra.ExactArgs(1),
	}

	flags := cmd.Flags()
	flags.StringVar(&hc.agentName, "agent", "", "configured agent whose model to use for summarizing")
	flags.StringVar(&hc.model, "model", "", "model ID to use directly, bypassing the config")
	flags.StringVar(&hc.from, "from", "planner", "outgoing agent name recorded in the handoff prefix")
	flags.StringVar(&hc.nextMessage, "next-message", "", "user message appended after the briefing, as the router would on the turn that triggered the switch")
	flags.BoolVar(&hc.showInput, "show-input", false, "print the serialized transcript handed to the summarizer")
	flags.BoolVar(&hc.showPrompt, "show-prompt", false, "print the summarizer system prompt")

	return cmd
}

func (hc *handoffCommand) Run(cmd *cobra.Command, args []string) {
	hc.configFactory.LoadConfig()

	l, err := loadSession(hc.configFactory, args[0])
	if err != nil {
		utils.NewExitError().WithMessage("failed to load session").WithReason(err).Done()
	}

	ai, agentCfg, err := summarizerAI(hc.configFactory, hc.agentName, hc.model)
	if err != nil {
		utils.NewExitError().WithMessage("failed to build summarizer").WithReason(err).Done()
	}

	section("SESSION")
	fmt.Printf("  id:    %s\n", l.session.ID)
	fmt.Printf("  start: %s\n", l.session.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("  model: %s", agentCfg.Model)
	if agentCfg.Name != "" {
		fmt.Printf("  (%s)", agentCfg.Name)
	}
	fmt.Println()

	section("REPLAYED HISTORY")
	renderHistory(l.replay.Messages, nil)
	renderSkipped(l.replay.Skipped)
	renderWarnings(l.replay.Warnings)

	if hc.showPrompt {
		section("HANDOFF SYSTEM PROMPT")
		fmt.Println(sdk.HandoffSystemPrompt())
	}

	preview, err := ai.PreviewHandoff(hc.from, l.replay.Messages)
	if err != nil {
		utils.NewExitError().WithMessage("handoff failed").WithReason(err).Done()
	}

	before := totalChars(l.replay.Messages)

	section(fmt.Sprintf("HANDOFF (%s -> incoming agent)", preview.From))
	fmt.Printf("  evicting all %d messages; the incoming agent keeps nothing verbatim\n", len(l.replay.Messages))

	if hc.showInput {
		section("SUMMARIZER INPUT")
		fmt.Println(preview.SummarizerInput)
	} else {
		dimColor.Printf("  summarizer input: %d chars (--show-input to print)\n", len(preview.SummarizerInput))
	}

	section("BRIEFING PRODUCED")
	fmt.Println(preview.Summary)

	rebuilt := preview.Rebuilt
	notes := map[int]string{
		0: fmt.Sprintf("HANDOFF BRIEFING, injected as role=%s", rebuilt[0].Role),
	}

	// The router appends the user's live message right after the briefing on
	// the turn that triggered the switch, so the incoming agent does see a real
	// user turn. Reproduce that when asked, since it is the main structural
	// difference from compaction.
	if hc.nextMessage != "" {
		rebuilt = append(rebuilt, sdk.NewTextMessage(sdk.RoleUser, hc.nextMessage))
		notes[len(rebuilt)-1] = "user's live message (router appends this)"
	}

	section("REBUILT HISTORY (what the incoming agent starts from)")
	renderHistory(rebuilt, notes)

	if hc.nextMessage == "" {
		dimColor.Println("  (pass --next-message to also show the live user turn the router appends)")
	}

	after := totalChars(rebuilt)
	section("NET EFFECT")
	fmt.Printf("  before: %7d chars (~%d tokens est)\n", before, estTokens(before))
	fmt.Printf("  after:  %7d chars (~%d tokens est)\n", after, estTokens(after))
}

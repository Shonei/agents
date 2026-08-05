package context

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Shonei/agents/pkg/config"
	"github.com/Shonei/agents/pkg/sdk"
	"github.com/Shonei/agents/pkg/utils"
)

type compactCommand struct {
	configFactory *config.ConfigFactory
	agentName     string
	model         string
	keepTurns     int
	dryRun        bool
	showInput     bool
	showPrompt    bool
}

// NewCompactCommand builds `agents context compact`.
func NewCompactCommand(c *config.ConfigFactory) *cobra.Command {
	cc := &compactCommand{configFactory: c}

	cmd := &cobra.Command{
		Use:   "compact <session_id>",
		Short: "Replay a recorded conversation through one compaction pass",
		Long: "Replay a recorded conversation through one compaction pass and show what the agent\n" +
			"would see afterwards: the cut point, what gets evicted, the summary produced, and the\n" +
			"rebuilt history including the role the summary is injected under.\n\n" +
			"Compaction feeds the SAME agent, with its system prompt unchanged. Compare with\n" +
			"`agents context handoff`, which briefs a DIFFERENT agent.\n\n" +
			"Unless --dry-run is set this makes one real model call to generate the summary.",
		Run:  cc.Run,
		Args: cobra.ExactArgs(1),
	}

	flags := cmd.Flags()
	flags.StringVar(&cc.agentName, "agent", "", "configured agent whose model (and max_context_turns) to use")
	flags.StringVar(&cc.model, "model", "", "model ID to use directly, bypassing the config")
	flags.IntVar(&cc.keepTurns, "keep-turns", 0, "turn boundaries to preserve verbatim (0 = the agent's configured value, else 2)")
	flags.BoolVar(&cc.dryRun, "dry-run", false, "resolve the cut point without calling the model")
	flags.BoolVar(&cc.showInput, "show-input", false, "print the serialized transcript handed to the summarizer")
	flags.BoolVar(&cc.showPrompt, "show-prompt", false, "print the summarizer system prompt")

	return cmd
}

func (cc *compactCommand) Run(cmd *cobra.Command, args []string) {
	cc.configFactory.LoadConfig()

	l, err := loadSession(cc.configFactory, args[0])
	if err != nil {
		utils.NewExitError().WithMessage("failed to load session").WithReason(err).Done()
	}

	ai, agentCfg, err := summarizerAI(cc.configFactory, cc.agentName, cc.model)
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

	if cc.showPrompt {
		section("COMPACTION SYSTEM PROMPT")
		fmt.Println(sdk.CompactionSystemPrompt())
	}

	preview, err := cc.run(ai, l.replay.Messages)
	if err != nil {
		utils.NewExitError().WithMessage("compaction failed").WithReason(err).Done()
	}

	before := totalChars(l.replay.Messages)

	section(fmt.Sprintf("COMPACTION (keep_turns=%d)", preview.KeepTurns))
	fmt.Printf("  pinned %d message(s) at the head (never evicted)\n", len(preview.Pinned))
	fmt.Printf("  cut at body index %d: evicting %d of %d messages (%d chars, ~%d tokens est)\n",
		preview.Cut, len(preview.Evicted), len(l.replay.Messages),
		totalChars(preview.Evicted), estTokens(totalChars(preview.Evicted)))
	fmt.Printf("  keeping %d messages verbatim (%d chars)\n",
		len(preview.Kept), totalChars(preview.Kept))

	if cc.showInput {
		section("SUMMARIZER INPUT")
		fmt.Println(preview.SummarizerInput)
	} else {
		dimColor.Printf("  summarizer input: %d chars (--show-input to print)\n", len(preview.SummarizerInput))
	}

	if cc.dryRun {
		noteColor.Println("\n  --dry-run: stopped before calling the model, no summary generated")

		return
	}

	section("SUMMARY PRODUCED")
	fmt.Println(preview.Summary)

	notes := make(map[int]string, 2)
	for i := range preview.Pinned {
		notes[i] = "PINNED original request (kept verbatim)"
	}

	summaryIdx := len(preview.Pinned)
	notes[summaryIdx] = fmt.Sprintf("COMPACTION SUMMARY, injected as role=%s",
		preview.Rebuilt[summaryIdx].Role)

	section("REBUILT HISTORY (what the agent sees next turn)")
	renderHistory(preview.Rebuilt, notes)

	after := totalChars(preview.Rebuilt)
	section("NET EFFECT")
	fmt.Printf("  before: %7d chars (~%d tokens est)\n", before, estTokens(before))
	fmt.Printf("  after:  %7d chars (~%d tokens est)\n", after, estTokens(after))

	if before > 0 {
		fmt.Printf("  saved:  %7d chars (%.1f%%)\n", before-after, 100*float64(before-after)/float64(before))
	}

	if after >= before {
		noteColor.Println("  ! compaction did not reduce the history")
	}
}

// run dispatches to the model-calling or model-free path.
func (cc *compactCommand) run(ai *sdk.AI, msgs []sdk.InputMessage) (*sdk.CompactionPreview, error) {
	if cc.dryRun {
		return ai.PlanCompaction(msgs, cc.keepTurns)
	}

	return ai.PreviewCompaction(msgs, cc.keepTurns)
}

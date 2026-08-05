package context

import (
	"fmt"
	"sort"
	"strings"

	"github.com/fatih/color"

	"github.com/Shonei/agents/pkg/sdk"
)

var (
	headerColor = color.New(color.FgCyan, color.Bold)
	noteColor   = color.New(color.FgYellow)
	roleUser    = color.New(color.FgGreen)
	roleAsst    = color.New(color.FgBlue)
	dimColor    = color.New(color.Faint)
)

// section prints a labeled divider so the phases of a preview are easy to scan.
func section(label string) {
	fmt.Println()
	headerColor.Printf("── %s ", label)
	headerColor.Println(strings.Repeat("─", max(0, 66-len(label))))
}

// renderHistory prints one row per message: index, role, the block types it
// carries, and its size. notes annotates specific indexes (used to flag the
// synthetic message compaction/handoff injects).
func renderHistory(msgs []sdk.InputMessage, notes map[int]string) {
	if len(msgs) == 0 {
		dimColor.Println("  (empty)")

		return
	}

	fmt.Printf("  %3s  %-9s  %-42s %9s\n", "#", "ROLE", "BLOCKS", "CHARS")

	for i, msg := range msgs {
		blocks, chars := describeBlocks(msg)

		roleStyle := roleAsst
		if msg.Role == sdk.RoleUser {
			roleStyle = roleUser
		}

		fmt.Printf("  %3d  ", i)
		roleStyle.Printf("%-9s", msg.Role)
		fmt.Printf("  %-42s %9d", truncate(blocks, 42), chars)

		if note, ok := notes[i]; ok {
			noteColor.Printf("  <- %s", note)
		}

		fmt.Println()
	}

	total := totalChars(msgs)
	dimColor.Printf("  %d messages, %d chars (~%d tokens est)\n", len(msgs), total, estTokens(total))
}

// describeBlocks summarizes a message's content blocks and total size.
func describeBlocks(msg sdk.InputMessage) (string, int) {
	switch v := msg.Content.(type) {
	case string:
		return "text", len(v)
	case []sdk.ContentBlock:
		parts := make([]string, 0, len(v))
		chars := 0

		for _, b := range v {
			switch b.Type {
			case sdk.ContentTypeToolUse:
				parts = append(parts, fmt.Sprintf("tool_use(%s)", b.Name))
			case sdk.ContentTypeToolResult:
				parts = append(parts, "tool_result")
			default:
				parts = append(parts, b.Type)
			}

			chars += blockChars(b)
		}

		return strings.Join(parts, ", "), chars
	default:
		return "?", 0
	}
}

func blockChars(b sdk.ContentBlock) int {
	n := len(b.Text)

	if s, ok := b.Content.(string); ok {
		n += len(s)
	}

	for k, v := range b.Input {
		n += len(k) + len(fmt.Sprintf("%v", v))
	}

	return n
}

func totalChars(msgs []sdk.InputMessage) int {
	total := 0
	for _, m := range msgs {
		_, chars := describeBlocks(m)
		total += chars
	}

	return total
}

// estTokens is a deliberately crude chars/4 estimate. It exists to give a sense
// of scale in the output, not to drive any decision.
func estTokens(chars int) int {
	return chars / 4
}

// renderSkipped prints the non-history event types that were ignored during
// replay, so nothing looks silently dropped.
func renderSkipped(skipped map[string]int) {
	if len(skipped) == 0 {
		return
	}

	types := make([]string, 0, len(skipped))
	for t := range skipped {
		types = append(types, t)
	}
	sort.Strings(types)

	parts := make([]string, 0, len(types))
	for _, t := range types {
		parts = append(parts, fmt.Sprintf("%s=%d", t, skipped[t]))
	}

	dimColor.Printf("  non-history events ignored: %s\n", strings.Join(parts, " "))
}

// renderWarnings prints the reconstruction caveats.
func renderWarnings(warnings []string) {
	for _, w := range warnings {
		noteColor.Printf("  ! %s\n", w)
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}

	if n <= 1 {
		return s[:n]
	}

	return s[:n-1] + "…"
}

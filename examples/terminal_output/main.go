// Package main demonstrates the terminal rendering options of agentmeter.
//
// It shows three ways to print the same trace:
//
//  1. Default coloured output (dark terminal)
//  2. Plain output — no ANSI codes, suitable for log files or CI
//  3. Custom styles — override individual colours via WithStyles
//
// Run with:
//
//	go run ./examples/terminal_output/
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/erlangb/agentmeter"
	"github.com/erlangb/agentmeter/pricing"
	"github.com/erlangb/agentmeter/reasoning"
	"github.com/charmbracelet/lipgloss"
)

func buildSnap() agentmeter.Snapshot {
	meter := agentmeter.New(pricing.WithDefaultPricing())
	meter.Reset("demo-run")

	start := time.Now()
	time.Sleep(15 * time.Millisecond)
	meter.Record(agentmeter.AgentStep{
		Role:            "model",
		Cluster:         agentmeter.ClusterCognitive,
		AgentName:       "assistant",
		ModelID:         "claude-sonnet-4-6",
		StartedAt:       start,
		ThinkingContent: "The user wants a poem. I will compose one.",
		Content:         "Here is a short poem about Go.",
		Usage: agentmeter.TokenUsage{
			PromptTokens:     180,
			CompletionTokens: 42,
		},
	})

	start = time.Now()
	time.Sleep(10 * time.Millisecond)
	meter.Record(agentmeter.AgentStep{
		Role:      "tool",
		Cluster:   agentmeter.ClusterAction,
		AgentName: "assistant",
		StartedAt: start,
		ToolName:  "save_poem",
		Content:   `{"status":"ok","path":"poem.txt"}`,
	})

	meter.Finalize()
	return meter.Snapshot()
}

func main() {
	snap := buildSnap()

	// 1. Default: coloured output for dark terminals.
	fmt.Println("── default (coloured) ──────────────────────────────────────────")
	reasoning.NewPrinter(os.Stdout).Print(snap)

	// 2. Plain: no ANSI codes — safe for log files and CI pipelines.
	fmt.Println("\n── plain (no ANSI) ─────────────────────────────────────────────")
	reasoning.NewPlainPrinter(os.Stdout).Print(snap)

	// 3. Custom styles: start from the defaults and override what you need.
	fmt.Println("\n── custom styles ───────────────────────────────────────────────")
	custom := reasoning.DefaultPrinterStyles()
	custom.Header = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	custom.Thinking = lipgloss.NewStyle().Foreground(lipgloss.Color("51"))
	reasoning.NewPrinter(os.Stdout, reasoning.WithStyles(custom)).Print(snap)

	// 4. Truncated content: limit long outputs to N runes.
	fmt.Println("\n── truncated to 30 runes ───────────────────────────────────────")
	reasoning.NewPlainPrinter(os.Stdout, reasoning.WithMaxContentLen(30)).Print(snap)
}
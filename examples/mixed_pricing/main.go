// Package main demonstrates per-model cost tracking with agentmeter.
//
// A real agent run often routes cheap classification or filtering work to a
// small model and reserves the expensive flagship model for final synthesis.
// agentmeter accumulates tokens per model in TokenSummary.ByModel so that
// OpenAICostFunc applies each model's rate to its own tokens — not all tokens
// at the last model's rate.
//
// Run with:
//
//	go run ./examples/mixed_pricing/
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/erlangb/agentmeter"
	"github.com/erlangb/agentmeter/pricing"
	"github.com/erlangb/agentmeter/reasoning"
)

func main() {
	tel := agentmeter.New(pricing.WithDefaultPricing())

	tel.Reset("analyst-agent")

	// Use the small model to triage three documents:
	for i, doc := range []string{"Q3 earnings transcript", "press release", "regulatory filing"} {
		start := time.Now()
		time.Sleep(5 * time.Millisecond)
		tel.Record(agentmeter.AgentStep{
			Role:      "model",
			Cluster:   agentmeter.ClusterCognitive,
			AgentName: "analyst-agent",
			ModelID:   "gpt-4o-mini",
			StartedAt: start,
			Content:   fmt.Sprintf("Document %d (%s): material — yes", i+1, doc),
			Usage: agentmeter.TokenUsage{
				PromptTokens:     300,
				CompletionTokens: 20,
				TotalTokens:      320,
			},
		})
	}

	// ── Phase 2: deep synthesis (gpt-4o) ──────────────────────────────────────
	//
	// Feed the classified summaries to the flagship model for final analysis.

	synthStart := time.Now()
	time.Sleep(40 * time.Millisecond)
	tel.Record(agentmeter.AgentStep{
		Role:      "model",
		Cluster:   agentmeter.ClusterCognitive,
		AgentName: "analyst-agent",
		ModelID:   "gpt-4o",
		StartedAt: synthStart,
		Content:   "Based on all three documents, revenue guidance was raised 8% and operating margins improved by 120 bps. Recommend overweight.",
		Usage: agentmeter.TokenUsage{
			PromptTokens:      1_800,
			CompletionTokens:  120,
			TotalTokens:       1_920,
			CachedInputTokens: 900, // shared system prompt served from cache
		},
	})

	tel.Finalize()
	snap := tel.Snapshot()

	// ── Print the reasoning trace ──────────────────────────────────────────────

	printer := reasoning.NewPrinter(os.Stdout)
	printer.Print(snap)
}

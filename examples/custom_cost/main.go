package main

import (
	"os"
	"time"

	"github.com/erlangb/agentmeter"
	"github.com/erlangb/agentmeter/reasoning"
)

// customCostSystem returns a CostFunc for Anthropic Claude.
// Note: We use the AggregateTokenUsage() helper to get totals across all models
// because your TokenSummary now stores usage in a map.
func customCostSystem() agentmeter.CostFunc {
	const (
		priceInput      = 3.00 / 1e6
		priceCacheWrite = 3.75 / 1e6
		priceCacheRead  = 0.30 / 1e6
		priceOutput     = 15.00 / 1e6
		priceReasoning  = 15.00 / 1e6
	)

	return func(s agentmeter.TokenSummary) float64 {
		// We calculate the aggregate usage across all models in this summary
		u := s.AggregateTokenUsage()

		standardInput := float64(u.PromptTokens - u.CachedInputTokens - u.CacheWriteTokens)
		if standardInput < 0 {
			standardInput = 0
		}

		return (standardInput * priceInput) +
			(float64(u.CacheWriteTokens) * priceCacheWrite) +
			(float64(u.CachedInputTokens) * priceCacheRead) +
			(float64(u.CompletionTokens) * priceOutput) +
			(float64(u.ReasoningTokens) * priceReasoning)
	}
}

func main() {
	// Initialize with the custom cost function
	tel := agentmeter.New(agentmeter.WithCostFunc(customCostSystem()))

	tel.Reset("research-agent")

	// Step 1: Extended-thinking phase.
	start := time.Now()
	time.Sleep(150 * time.Millisecond)
	tel.Record(agentmeter.AgentStep{
		Role:            "thinking",
		Cluster:         agentmeter.ClusterCognitive,
		ModelID:         "claude-3-5-sonnet",
		StartedAt:       start,
		ThinkingContent: "The user is asking about quantum entanglement. Let me reason through the key concepts...",
		Usage:           agentmeter.TokenUsage{ReasoningTokens: 800},
	})

	// Step 2: Model response with cache write.
	start = time.Now()
	time.Sleep(20 * time.Millisecond)
	tel.Record(agentmeter.AgentStep{
		Role:      "model",
		Cluster:   agentmeter.ClusterCognitive,
		ModelID:   "claude-3-5-sonnet",
		StartedAt: start,
		Content:   "Quantum entanglement is a phenomenon where particles become correlated...",
		Usage: agentmeter.TokenUsage{
			PromptTokens:     1200,
			CompletionTokens: 180,
			TotalTokens:      1380,
			CacheWriteTokens: 1000,
		},
	})

	// Step 3: Follow-up question — cache hit.
	start = time.Now()
	time.Sleep(12 * time.Millisecond)
	tel.Record(agentmeter.AgentStep{
		Role:      "model",
		Cluster:   agentmeter.ClusterCognitive,
		ModelID:   "claude-3-5-sonnet",
		StartedAt: start,
		Content:   "Bell's theorem shows that no local hidden-variable theory...",
		Usage: agentmeter.TokenUsage{
			PromptTokens:      1400,
			CompletionTokens:  140,
			TotalTokens:       1540,
			CachedInputTokens: 1000,
		},
	})

	// Finalize and print
	tel.Finalize()
	snap := tel.Snapshot()

	// Using the Printer we designed
	p := reasoning.NewPrinter(os.Stdout)
	p.Print(snap)
}

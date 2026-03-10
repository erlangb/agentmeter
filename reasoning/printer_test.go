package reasoning_test

import (
	"testing"
	"time"

	"github.com/erlangb/agentmeter"
	"github.com/erlangb/agentmeter/reasoning"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helpers

func cognitiveSnap(modelID string, prompt, completion int) agentmeter.Snapshot {
	return agentmeter.Snapshot{
		Steps: []agentmeter.AgentStep{
			{
				Role:    "model",
				Cluster: agentmeter.ClusterCognitive,
				ModelID: modelID,
				Content: "response",
				Usage:   agentmeter.TokenUsage{PromptTokens: prompt, CompletionTokens: completion},
			},
		},
		TokenSummary: agentmeter.TokenSummary{
			ModelCalls: 1,
			ByModel: map[string]agentmeter.TokenUsage{
				modelID: {PromptTokens: prompt, CompletionTokens: completion},
			},
		},
	}
}

// Truncate

func TestTruncate_UnderLimit(t *testing.T) {
	assert.Equal(t, "hello", reasoning.Truncate("hello", 10))
}

func TestTruncate_ExactlyAtLimit(t *testing.T) {
	assert.Equal(t, "hello", reasoning.Truncate("hello", 5))
}

func TestTruncate_Over(t *testing.T) {
	result := reasoning.Truncate("hello world", 5)
	assert.Equal(t, "hello…", result)
}

func TestTruncate_Unicode(t *testing.T) {
	// Each emoji is one rune; truncation must count runes not bytes.
	result := reasoning.Truncate("🐙🦑🐠🐡", 2)
	assert.Equal(t, "🐙🦑…", result)
}

// PrintTokenSummary

func TestPrintTokenSummary_SilentWhenNoModelCalls(t *testing.T) {
	p, buf := reasoning.NewBufferedPrinter()
	p.PrintTokenSummary(agentmeter.Snapshot{})
	assert.Empty(t, buf.String(), "no output when ModelCalls == 0")
}

func TestPrintTokenSummary_ShowsCostWhenPositive(t *testing.T) {
	p, buf := reasoning.NewBufferedPrinter()
	snap := agentmeter.Snapshot{
		TokenSummary: agentmeter.TokenSummary{
			ModelCalls:       1,
			EstimatedCostUSD: 0.0042,
			ByModel:          map[string]agentmeter.TokenUsage{"gpt-4o": {PromptTokens: 100}},
		},
	}
	p.PrintTokenSummary(snap)
	assert.Contains(t, buf.String(), "0.0042")
}

func TestPrintTokenSummary_NoCostLabel(t *testing.T) {
	p, buf := reasoning.NewBufferedPrinter()
	snap := agentmeter.Snapshot{
		TokenSummary: agentmeter.TokenSummary{
			ModelCalls: 1,
			ByModel:    map[string]agentmeter.TokenUsage{"gpt-4o": {}},
		},
	}
	p.PrintTokenSummary(snap)
	assert.Contains(t, buf.String(), "no cost estimate")
}

func TestPrintTokenSummary_SingleModel_ShowsModelName(t *testing.T) {
	p, buf := reasoning.NewBufferedPrinter()
	p.PrintTokenSummary(cognitiveSnap("gpt-4o", 100, 50))
	// single model → model name appears, no breakdown section
	assert.Contains(t, buf.String(), "gpt-4o")
}

func TestPrintTokenSummary_MultiModel_ShowsBreakdown(t *testing.T) {
	p, buf := reasoning.NewBufferedPrinter()
	snap := agentmeter.Snapshot{
		TokenSummary: agentmeter.TokenSummary{
			ModelCalls: 2,
			ByModel: map[string]agentmeter.TokenUsage{
				"gpt-4o":      {PromptTokens: 100},
				"gpt-4o-mini": {PromptTokens: 50},
			},
		},
	}
	p.PrintTokenSummary(snap)
	out := buf.String()
	assert.Contains(t, out, "gpt-4o")
	assert.Contains(t, out, "gpt-4o-mini")
	// breakdown lines appear for both models
	assert.Equal(t, 2, countOccurrences(out, "--"), "expected one breakdown line per model")
}

func TestPrintTokenSummary_BreakdownSorted(t *testing.T) {
	p, buf := reasoning.NewBufferedPrinter()
	snap := agentmeter.Snapshot{
		TokenSummary: agentmeter.TokenSummary{
			ModelCalls: 2,
			ByModel: map[string]agentmeter.TokenUsage{
				"z-model": {PromptTokens: 10},
				"a-model": {PromptTokens: 20},
			},
		},
	}
	p.PrintTokenSummary(snap)
	out := buf.String()
	posA := findIndex(out, "a-model")
	posZ := findIndex(out, "z-model")
	assert.Less(t, posA, posZ, "breakdown lines must be sorted alphabetically")
}

// Print

func TestPrint_StepNumberedFromOne(t *testing.T) {
	p, buf := reasoning.NewBufferedPrinter()
	snap := agentmeter.Snapshot{
		Steps: []agentmeter.AgentStep{
			{Role: "model", Cluster: agentmeter.ClusterCognitive, Content: "hello"},
			{Role: "tool", Cluster: agentmeter.ClusterAction, ToolName: "search", Content: "results"},
		},
	}
	p.Print(snap)
	out := buf.String()
	assert.Contains(t, out, "step 1")
	assert.Contains(t, out, "step 2")
}

func TestPrint_ResetsStepCounterOnEachCall(t *testing.T) {
	p, buf := reasoning.NewBufferedPrinter()
	snap := agentmeter.Snapshot{
		Steps: []agentmeter.AgentStep{
			{Role: "model", Cluster: agentmeter.ClusterCognitive, Content: "hello"},
		},
	}

	p.Print(snap)
	buf.Reset()
	p.Print(snap) // second call must restart from step 1

	out := buf.String()
	assert.Contains(t, out, "step 1", "step counter must restart from 1 on second Print")
	assert.NotContains(t, out, "step 2")
}

func TestPrint_Cognitivestep_ShowsContent(t *testing.T) {
	p, buf := reasoning.NewBufferedPrinter()
	snap := agentmeter.Snapshot{
		Steps: []agentmeter.AgentStep{
			{Role: "model", Cluster: agentmeter.ClusterCognitive, Content: "my reasoning here"},
		},
	}
	p.Print(snap)
	assert.Contains(t, buf.String(), "my reasoning here")
}

func TestPrint_ToolStep_ShowsToolName(t *testing.T) {
	p, buf := reasoning.NewBufferedPrinter()
	snap := agentmeter.Snapshot{
		Steps: []agentmeter.AgentStep{
			{Role: "tool", Cluster: agentmeter.ClusterAction, ToolName: "web_search", Content: "10 results"},
		},
	}
	p.Print(snap)
	assert.Contains(t, buf.String(), "web_search")
}

func TestPrint_ThinkingStep_ShowsThinkingContent(t *testing.T) {
	p, buf := reasoning.NewBufferedPrinter()
	snap := agentmeter.Snapshot{
		Steps: []agentmeter.AgentStep{
			{Role: "thinking", Cluster: agentmeter.ClusterCognitive, ThinkingContent: "inner monologue"},
		},
	}
	p.Print(snap)
	assert.Contains(t, buf.String(), "inner monologue")
}

func TestPrint_ErrorStep_ShowsContent(t *testing.T) {
	p, buf := reasoning.NewBufferedPrinter()
	snap := agentmeter.Snapshot{
		Steps: []agentmeter.AgentStep{
			{Role: "error", Cluster: agentmeter.ClusterError, Content: "timeout after 30s"},
		},
	}
	p.Print(snap)
	assert.Contains(t, buf.String(), "timeout after 30s")
}

func TestPrint_StepDuration_Shown(t *testing.T) {
	p, buf := reasoning.NewBufferedPrinter()
	snap := agentmeter.Snapshot{
		Steps: []agentmeter.AgentStep{
			{Role: "model", Cluster: agentmeter.ClusterCognitive, Duration: 250 * time.Millisecond},
		},
	}
	p.Print(snap)
	assert.Contains(t, buf.String(), "250ms")
}

// PrintHistory

func TestPrintHistory_Empty_NoOutput(t *testing.T) {
	p, buf := reasoning.NewBufferedPrinter()
	p.PrintHistory(nil)
	assert.Empty(t, buf.String())
}

func TestPrintHistory_SingleRun_NoAggregateLine(t *testing.T) {
	p, buf := reasoning.NewBufferedPrinter()
	p.PrintHistory([]agentmeter.Snapshot{cognitiveSnap("gpt-4o", 100, 50)})
	assert.NotContains(t, buf.String(), "Total across")
}

func TestPrintHistory_MultiRun_ShowsRunHeaders(t *testing.T) {
	p, buf := reasoning.NewBufferedPrinter()
	snaps := []agentmeter.Snapshot{
		cognitiveSnap("gpt-4o", 100, 50),
		cognitiveSnap("gpt-4o", 200, 80),
	}
	p.PrintHistory(snaps)
	out := buf.String()
	assert.Contains(t, out, "Run 1 / 2")
	assert.Contains(t, out, "Run 2 / 2")
}

func TestPrintHistory_MultiRun_ShowsAggregateLine(t *testing.T) {
	p, buf := reasoning.NewBufferedPrinter()
	snaps := []agentmeter.Snapshot{
		cognitiveSnap("gpt-4o", 100, 50),
		cognitiveSnap("gpt-4o", 200, 80),
	}
	p.PrintHistory(snaps)
	assert.Contains(t, buf.String(), "Total across 2 runs")
}

func TestPrintHistory_StepsRestartFromOnePerRun(t *testing.T) {
	p, buf := reasoning.NewBufferedPrinter()
	snap := cognitiveSnap("gpt-4o", 100, 50)
	p.PrintHistory([]agentmeter.Snapshot{snap, snap})

	out := buf.String()
	// "step 1" must appear twice (once per run), never "step 2"
	require.Equal(t, 2, countOccurrences(out, "step 1"), "each run should restart from step 1")
	assert.NotContains(t, out, "step 2")
}

// helpers

func countOccurrences(s, sub string) int {
	count := 0
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			count++
		}
	}
	return count
}

func findIndex(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
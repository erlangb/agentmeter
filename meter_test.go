package agentmeter_test

import (
	"testing"
	"time"

	"github.com/erlangb/agentmeter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecord_AutoDurationFromStartedAt(t *testing.T) {
	tel := agentmeter.New()
	tel.Reset("agent")

	start := time.Now()
	time.Sleep(10 * time.Millisecond)
	tel.Record(agentmeter.AgentStep{
		Role:      "model",
		Cluster:   agentmeter.ClusterCognitive,
		StartedAt: start,
		// Duration intentionally zero — should be auto-computed from StartedAt.
	})

	snap := tel.Snapshot()
	require.Len(t, snap.Steps, 1)
	assert.GreaterOrEqual(t, snap.Steps[0].Duration, 10*time.Millisecond, "Duration should be auto-computed from StartedAt")
}

func TestTotalDuration_IsWallClock(t *testing.T) {
	tel := agentmeter.New()
	tel.Reset("agent")
	time.Sleep(10 * time.Millisecond)
	tel.Record(agentmeter.AgentStep{
		Role:     "model",
		Cluster:  agentmeter.ClusterCognitive,
		Duration: time.Microsecond, // tiny explicit step duration
	})
	tel.Finalize()

	snap := tel.Snapshot()
	// TotalDuration is wall-clock from Reset to Finalize — must be >= sleep, not just step duration.
	assert.GreaterOrEqual(t, snap.TotalDuration, 10*time.Millisecond, "TotalDuration must be wall-clock, not sum of step durations")
}

func TestReset_SetsLabel(t *testing.T) {
	tel := agentmeter.New()
	tel.Reset("conversation-42")
	snap := tel.Snapshot()
	assert.Equal(t, "conversation-42", snap.Label)
}

func TestRecord_PreservesAgentName(t *testing.T) {
	tel := agentmeter.New()
	tel.Reset("run-label")

	// Two steps from two different agents in the same run.
	tel.Record(agentmeter.AgentStep{
		Role:      "model",
		Cluster:   agentmeter.ClusterCognitive,
		AgentName: "planner",
		Content:   "I will delegate to executor",
		Duration:  time.Millisecond,
	})
	tel.Record(agentmeter.AgentStep{
		Role:      "model",
		Cluster:   agentmeter.ClusterCognitive,
		AgentName: "executor",
		Content:   "Executing the plan",
		Duration:  time.Millisecond,
	})

	snap := tel.Snapshot()
	assert.Equal(t, "run-label", snap.Label, "run label set by Reset")
	assert.Equal(t, "planner", snap.Steps[0].AgentName, "Record must not overwrite AgentName")
	assert.Equal(t, "executor", snap.Steps[1].AgentName, "Record must not overwrite AgentName")
}

func TestNew_DefaultsAreApplied(t *testing.T) {
	tel := agentmeter.New()
	require.NotNil(t, tel)

	snap := tel.Snapshot()
	assert.Empty(t, snap.Steps)
	assert.Zero(t, snap.TotalDuration)
	assert.Zero(t, snap.TokenSummary.EstimatedCostUSD)
}

func TestReset_ClearsRunState(t *testing.T) {
	tel := agentmeter.New()
	tel.Reset("agent-a")
	tel.Record(agentmeter.AgentStep{
		Role:      "model",
		Cluster:   agentmeter.ClusterCognitive,
		AgentName: "agent-a",
		Content:   "hello",
		Usage:     agentmeter.TokenUsage{PromptTokens: 10, CompletionTokens: 5},
		Duration:  time.Millisecond,
	})

	tel.Reset("agent-b")
	snap := tel.Snapshot()
	assert.Empty(t, snap.Steps, "Reset should clear steps")
	// TotalDuration is wall-clock from Reset; immediately after Reset it is near zero.
	assert.Less(t, snap.TotalDuration, time.Millisecond, "TotalDuration should be near zero immediately after Reset")
}

func TestRecordModelStep_AccumulatesTokens(t *testing.T) {
	tel := agentmeter.New()
	tel.Reset("agent")

	tel.Record(agentmeter.AgentStep{
		Role:      "model",
		Cluster:   agentmeter.ClusterCognitive,
		AgentName: "agent",
		ModelID:   "test-model",
		Content:   "first",
		Usage:     agentmeter.TokenUsage{PromptTokens: 100, CompletionTokens: 50},
		Duration:  10 * time.Millisecond,
	})
	tel.Record(agentmeter.AgentStep{
		Role:      "model",
		Cluster:   agentmeter.ClusterCognitive,
		AgentName: "agent",
		ModelID:   "test-model",
		Content:   "second",
		Usage:     agentmeter.TokenUsage{PromptTokens: 200, CompletionTokens: 80},
		Duration:  20 * time.Millisecond,
	})

	snap := tel.Snapshot()
	require.Len(t, snap.Steps, 2)
	assert.Equal(t, 2, snap.TokenSummary.ModelCalls)
	assert.Equal(t, 300, snap.TokenSummary.ByModel["test-model"].PromptTokens)
	assert.Equal(t, 130, snap.TokenSummary.ByModel["test-model"].CompletionTokens)
	// Per-step durations are stored as provided by the caller.
	assert.Equal(t, 10*time.Millisecond, snap.Steps[0].Duration)
	assert.Equal(t, 20*time.Millisecond, snap.Steps[1].Duration)
	// TotalDuration is wall-clock elapsed from Reset, not the sum of step durations.
	assert.Greater(t, snap.TotalDuration, time.Duration(0))
}

func TestRecordToolStep(t *testing.T) {
	tel := agentmeter.New()
	tel.Reset("agent")

	tel.Record(agentmeter.AgentStep{
		Role:      "tool",
		Cluster:   agentmeter.ClusterAction,
		AgentName: "agent",
		ToolName:  "search",
		ToolInput: `{"q":"go"}`,
		Content:   "results",
		Duration:  5 * time.Millisecond,
	})

	snap := tel.Snapshot()
	require.Len(t, snap.Steps, 1)
	step := snap.Steps[0]
	assert.Equal(t, agentmeter.StepRole("tool"), step.Role)
	assert.Equal(t, agentmeter.ClusterAction, step.Cluster)
	assert.Equal(t, "search", step.ToolName)
	assert.Equal(t, `{"q":"go"}`, step.ToolInput)
	assert.Equal(t, "results", step.Content)
	// Action cluster steps do not count toward ModelCalls.
	assert.Zero(t, snap.TokenSummary.ModelCalls)
}

func TestRecordThinkingStep(t *testing.T) {
	tel := agentmeter.New()
	tel.Reset("agent")

	tel.Record(agentmeter.AgentStep{
		Role:            "thinking",
		Cluster:         agentmeter.ClusterCognitive,
		AgentName:       "agent",
		ThinkingContent: "let me think...",
		Duration:        3 * time.Millisecond,
		Usage:           agentmeter.TokenUsage{ReasoningTokens: 42},
	})

	snap := tel.Snapshot()
	require.Len(t, snap.Steps, 1)
	step := snap.Steps[0]
	assert.Equal(t, agentmeter.StepRole("thinking"), step.Role)
	assert.Equal(t, agentmeter.ClusterCognitive, step.Cluster)
	assert.Equal(t, "let me think...", step.ThinkingContent)
	// Thinking is ClusterCognitive — it increments ModelCalls.
	assert.Equal(t, 1, snap.TokenSummary.ModelCalls)
}

func TestCostFunc_IsAppliedInSnapshot(t *testing.T) {
	costFn := func(s agentmeter.TokenSummary) float64 {
		return float64(s.AggregateTokenUsage().PromptTokens) * 0.001
	}
	tel := agentmeter.New(agentmeter.WithCostFunc(costFn))
	tel.Reset("agent")
	tel.Record(agentmeter.AgentStep{
		Role:      "model",
		Cluster:   agentmeter.ClusterCognitive,
		AgentName: "agent",
		ModelID:   "test-model",
		Usage:     agentmeter.TokenUsage{PromptTokens: 1000},
		Duration:  time.Millisecond,
	})

	snap := tel.Snapshot()
	assert.InDelta(t, 1.0, snap.TokenSummary.EstimatedCostUSD, 1e-9)
}

func TestSnapshot_IsDeepCopy(t *testing.T) {
	tel := agentmeter.New()
	tel.Reset("agent")
	tel.Record(agentmeter.AgentStep{
		Role:      "model",
		Cluster:   agentmeter.ClusterCognitive,
		AgentName: "agent",
		Content:   "original",
		Duration:  time.Millisecond,
	})

	snap := tel.Snapshot()
	require.Len(t, snap.Steps, 1)

	// Mutate the snapshot; Meter must be unaffected.
	snap.Steps[0].Content = "mutated"

	snap2 := tel.Snapshot()
	assert.Equal(t, "original", snap2.Steps[0].Content)
}

func TestFinalize_AppendsToHistory(t *testing.T) {
	tel := agentmeter.New()

	tel.Reset("run-1")
	tel.Record(agentmeter.AgentStep{Role: "model", Cluster: agentmeter.ClusterCognitive, AgentName: "run-1", Duration: time.Millisecond})
	tel.Finalize()

	tel.Reset("run-2")
	tel.Record(agentmeter.AgentStep{Role: "model", Cluster: agentmeter.ClusterCognitive, AgentName: "run-2", Duration: time.Millisecond})
	tel.Finalize()

	hist := tel.History()
	require.Len(t, hist, 2)
	assert.Equal(t, "run-1", hist[0].Label)
	assert.Equal(t, "run-2", hist[1].Label)
	assert.Equal(t, 1, hist[0].TokenSummary.ModelCalls)
	assert.Equal(t, 1, hist[1].TokenSummary.ModelCalls)
}

func TestFinalize_IdempotentDoubleCall(t *testing.T) {
	tel := agentmeter.New()
	tel.Reset("agent")
	tel.Record(agentmeter.AgentStep{Role: "model", Cluster: agentmeter.ClusterCognitive, AgentName: "agent", Duration: time.Millisecond})
	tel.Finalize()
	tel.Finalize() // second call must be a no-op

	hist := tel.History()
	assert.Len(t, hist, 1, "double Finalize must not append twice")
}

func TestHistory_BoundedByMaxHistory(t *testing.T) {
	const max = 3
	tel := agentmeter.New(agentmeter.WithMaxHistory(max))

	for i := 0; i < max+2; i++ {
		tel.Reset("agent")
		tel.Record(agentmeter.AgentStep{Role: "model", Cluster: agentmeter.ClusterCognitive, AgentName: "agent", Duration: time.Millisecond})
		tel.Finalize()
	}

	hist := tel.History()
	assert.Len(t, hist, max, "history should be capped at MaxHistory")
}

func TestClearHistory(t *testing.T) {
	tel := agentmeter.New()
	tel.Reset("agent")
	tel.Record(agentmeter.AgentStep{Role: "model", Cluster: agentmeter.ClusterCognitive, AgentName: "agent", Duration: time.Millisecond})
	tel.Finalize()

	tel.ClearHistory()
	assert.Empty(t, tel.History())
}

func TestCacheWriteTokens_AccumulatedInSummary(t *testing.T) {
	tel := agentmeter.New()
	tel.Reset("agent")
	tel.Record(agentmeter.AgentStep{
		Role:      "model",
		Cluster:   agentmeter.ClusterCognitive,
		AgentName: "agent",
		ModelID:   "test-model",
		Usage: agentmeter.TokenUsage{
			PromptTokens:      500,
			CompletionTokens:  100,
			CachedInputTokens: 200,
			CacheWriteTokens:  50,
		},
		Duration: time.Millisecond,
	})

	snap := tel.Snapshot()
	assert.Equal(t, 200, snap.TokenSummary.ByModel["test-model"].CachedInputTokens)
	assert.Equal(t, 50, snap.TokenSummary.ByModel["test-model"].CacheWriteTokens)
}

func TestByModel_AccumulatesPerModel(t *testing.T) {
	tel := agentmeter.New()
	tel.Reset("agent")

	tel.Record(agentmeter.AgentStep{
		Role:      "model",
		Cluster:   agentmeter.ClusterCognitive,
		AgentName: "agent",
		ModelID:   "gpt-4o-mini",
		Usage:     agentmeter.TokenUsage{PromptTokens: 100, CompletionTokens: 50, TotalTokens: 150},
		Duration:  time.Millisecond,
	})
	tel.Record(agentmeter.AgentStep{
		Role:      "model",
		Cluster:   agentmeter.ClusterCognitive,
		AgentName: "agent",
		ModelID:   "gpt-4o",
		Usage:     agentmeter.TokenUsage{PromptTokens: 200, CompletionTokens: 80, TotalTokens: 280},
		Duration:  time.Millisecond,
	})
	// Second call to gpt-4o-mini — tokens should accumulate.
	tel.Record(agentmeter.AgentStep{
		Role:      "model",
		Cluster:   agentmeter.ClusterCognitive,
		AgentName: "agent",
		ModelID:   "gpt-4o-mini",
		Usage:     agentmeter.TokenUsage{PromptTokens: 50, CompletionTokens: 20, TotalTokens: 70},
		Duration:  time.Millisecond,
	})

	snap := tel.Snapshot()

	mini := snap.TokenSummary.ByModel["gpt-4o-mini"]
	assert.Equal(t, 150, mini.PromptTokens, "gpt-4o-mini prompt tokens")
	assert.Equal(t, 70, mini.CompletionTokens, "gpt-4o-mini completion tokens")
	assert.Equal(t, 220, mini.TotalTokens, "gpt-4o-mini total tokens")

	full := snap.TokenSummary.ByModel["gpt-4o"]
	assert.Equal(t, 200, full.PromptTokens, "gpt-4o prompt tokens")
	assert.Equal(t, 80, full.CompletionTokens, "gpt-4o completion tokens")

	// Aggregate totals must equal the sum across all steps.
	assert.Equal(t, 350, snap.TokenSummary.AggregateTokenUsage().PromptTokens, "flat prompt total")
	assert.Equal(t, 150, snap.TokenSummary.AggregateTokenUsage().CompletionTokens, "flat completion total")
	assert.Equal(t, 3, snap.TokenSummary.ModelCalls)
}

func TestRecord_CustomRoleWithCluster(t *testing.T) {
	tel := agentmeter.New()
	tel.Reset("agent")
	// Custom role unknown to the library — cluster drives counter gating.
	tel.Record(agentmeter.AgentStep{
		Role:      "retrieval",
		Cluster:   agentmeter.ClusterAction,
		AgentName: "agent",
		Content:   "found 3 docs",
		Duration:  time.Millisecond,
	})
	snap := tel.Snapshot()
	require.Len(t, snap.Steps, 1)
	assert.Equal(t, agentmeter.StepRole("retrieval"), snap.Steps[0].Role)
	assert.Equal(t, agentmeter.ClusterAction, snap.Steps[0].Cluster)
	assert.Zero(t, snap.TokenSummary.ModelCalls) // action cluster, not cognitive
}

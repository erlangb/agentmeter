// Package agentmeter provides framework-agnostic observability for LLM agent runs.
// It records reasoning traces, token usage, tool calls, and estimated costs
// without introducing any external dependencies in the core package.
package agentmeter

import (
	"maps"
	"time"
)

// StepRole is a free-form descriptive label for a reasoning step.
// The library defines the type but no constants — clients supply whatever
// role labels make sense for their framework or application (e.g. "model",
// "tool", "retrieval", "rerank"). Rendering and counter gating are driven
// by StepCluster, not by Role.
type StepRole string

// StepCluster is the stable rendering and routing category for a step.
// It is the library's fixed vocabulary; Role is owned by the client.
// Use one of the Cluster* constants when recording a step.
type StepCluster string

const (
	// ClusterCognitive covers model inference and extended-thinking steps.
	// Steps with this cluster increment ModelCalls and are rendered with
	// thinking/plan/call labels.
	ClusterCognitive StepCluster = "cognitive"
	// ClusterAction covers tool calls and external side-effects.
	ClusterAction StepCluster = "action"
	// ClusterMessage covers text responses directed at the user.
	ClusterMessage StepCluster = "message"
	// ClusterError covers failures, exceptions, and retries.
	ClusterError StepCluster = "error"
)

// TokenUsage captures raw token counts.
// It is the "source of truth" for what happened in a single model interaction.
type TokenUsage struct {
	PromptTokens      int
	CompletionTokens  int
	TotalTokens       int
	CachedInputTokens int
	CacheWriteTokens  int
	ReasoningTokens   int
}

// Add returns a NEW TokenUsage that is the sum of u and other.
func (u TokenUsage) Add(other TokenUsage) TokenUsage {
	return TokenUsage{
		PromptTokens:      u.PromptTokens + other.PromptTokens,
		CompletionTokens:  u.CompletionTokens + other.CompletionTokens,
		TotalTokens:       u.TotalTokens + other.TotalTokens,
		ReasoningTokens:   u.ReasoningTokens + other.ReasoningTokens,
		CachedInputTokens: u.CachedInputTokens + other.CachedInputTokens,
		CacheWriteTokens:  u.CacheWriteTokens + other.CacheWriteTokens,
	}
}

// TokenSummary aggregates multiple usages into a final report.
type TokenSummary struct {
	// ByModel holds token usage aggregated per model ID.
	ByModel map[string]TokenUsage
	// ModelCalls is the number of ClusterCognitive steps recorded.
	ModelCalls int
	// EstimatedCostUSD is the cost computed by CostFunc at Snapshot time.
	EstimatedCostUSD float64
}

// AggregateTokenUsage returns the sum of all per-model token usage.
func (s TokenSummary) AggregateTokenUsage() TokenUsage {
	var total TokenUsage
	for _, usage := range s.ByModel {
		total = total.Add(usage)
	}
	return total
}

// AgentStep is an immutable record of one step in an agent run.
type AgentStep struct {
	// Role is a free descriptive label set by the client (e.g. "model", "tool",
	// "retrieval"). It appears in headers and logs but does not drive rendering
	// or counter gating — use Cluster for that.
	Role StepRole
	// Cluster drives display and counter gating. Set to one of the Cluster*
	// constants. ClusterCognitive increments ModelCalls; ClusterAction renders
	// as a tool result; ClusterMessage and ClusterError have dedicated styles.
	Cluster StepCluster
	// AgentName is the name of the agent that performed this step.
	AgentName string
	// Provider identifies the API provider serving the model
	// (e.g. "openai", "anthropic", "google", "azure_openai", "aws_bedrock").
	// Follows the gen_ai.system semantic convention from OpenTelemetry.
	// Optional: leave empty if the provider is unambiguous from ModelID in your setup.
	// Explicit over inferred — the same ModelID can be served by multiple providers
	// (e.g. "claude-3-5-sonnet" via Anthropic direct, AWS Bedrock, or Vertex AI).
	Provider string
	// ModelID is the model identifier (e.g. "gpt-4o"). Non-empty for cognitive steps.
	ModelID string
	// Content holds the primary text output (model response text or tool result).
	Content string
	// ThinkingContent holds chain-of-thought text for thinking steps.
	ThinkingContent string
	// ToolName is the name of the tool invoked. Non-empty for action steps.
	ToolName string
	// ToolInput is the serialised input passed to the tool.
	ToolInput string
	// ToolCallID is the provider-assigned correlation ID linking a tool call
	// in an assistant message to its corresponding tool result step.
	ToolCallID string
	// ToolCalls is a list of tool-call summaries (e.g. "search({q:\"go\"})") for
	// model steps that dispatched one or more tools. Empty for other step types.
	ToolCalls []string
	// Usage records token counts for model and thinking steps; zero for tool steps.
	Usage TokenUsage
	// StartedAt is the wall-clock time at which this step began.
	// If Duration is zero and StartedAt is non-zero, Record() auto-computes
	// Duration = time.Since(StartedAt). Set this immediately before issuing
	// the model or tool call — before any blocking I/O — for accurate timing.
	StartedAt time.Time
	// Duration is the wall-clock time spent in this step.
	// If set explicitly it takes precedence over StartedAt-based auto-calculation.
	Duration time.Duration
}

// Snapshot is a point-in-time, immutable view of an agent run.
// It is returned by Meter.Snapshot() and stored in History().
// Unlike Meter, a Snapshot carries no mutex and is safe to share freely.
type Snapshot struct {
	// Label is the run identifier set by Reset(label). It identifies the run,
	// not the agents within it. Individual steps carry their own AgentName.
	Label string
	// Steps is a deep copy of the reasoning steps recorded so far.
	Steps []AgentStep
	// TokenSummary is an aggregated token and cost summary for this run.
	TokenSummary TokenSummary
	// TotalDuration is the accumulated wall-clock time across all steps in this snapshot.
	TotalDuration time.Duration
}

// deepCopy creates a complete copy of the Snapshot, including nested slices and maps.
func (s Snapshot) deepCopy() Snapshot {
	cp := s
	if s.Steps != nil {
		cp.Steps = make([]AgentStep, len(s.Steps))
		copy(cp.Steps, s.Steps)
	}

	if s.TokenSummary.ByModel != nil {
		cp.TokenSummary.ByModel = make(map[string]TokenUsage, len(s.TokenSummary.ByModel))
		//copy maps
		maps.Copy(cp.TokenSummary.ByModel, s.TokenSummary.ByModel)
	}
	return cp
}

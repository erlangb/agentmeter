package einometer_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/cloudwego/eino/callbacks"
	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/erlangb/agentmeter"
	einometer "github.com/erlangb/agentmeter/adapters/eino"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func chainInfo(name string) *callbacks.RunInfo {
	return &callbacks.RunInfo{Name: name, Component: compose.ComponentOfChain}
}

func toolNodeInfo(name string) *callbacks.RunInfo {
	return &callbacks.RunInfo{Name: name, Component: compose.ComponentOfToolsNode}
}

func modelInfo(name string) *callbacks.RunInfo {
	return &callbacks.RunInfo{Name: name, Component: "ChatModel"}
}

// --- OnStart ---

func TestOnStart_Chain_ResetsLabel(t *testing.T) {
	meter := agentmeter.New()
	h := einometer.NewAgentMeterHandler(meter)

	h.OnStart(context.Background(), chainInfo("run-1"), nil)

	assert.Equal(t, "run-1", meter.Snapshot().Label)
}

func TestOnStart_NonChain_DoesNotReset(t *testing.T) {
	meter := agentmeter.New()
	meter.Reset("original")
	h := einometer.NewAgentMeterHandler(meter)

	h.OnStart(context.Background(), toolNodeInfo("tools"), nil)

	assert.Equal(t, "original", meter.Snapshot().Label)
}

// --- OnEnd: model ---

func TestOnEnd_ModelOutput_RecordsCognitiveStep(t *testing.T) {
	tests := []struct {
		name        string
		cbOut       *einomodel.CallbackOutput
		wantContent string
		wantModel   string
		wantPrompt  int
		wantComp    int
		wantTotal   int
		wantCached  int
		wantReason  int
	}{
		{
			name: "full output with token usage",
			cbOut: &einomodel.CallbackOutput{
				Config: &einomodel.Config{Model: "gpt-4o"},
				Message: &schema.Message{
					Role:    schema.Assistant,
					Content: "hello world",
				},
				TokenUsage: &einomodel.TokenUsage{
					PromptTokens:     100,
					CompletionTokens: 50,
					TotalTokens:      150,
					PromptTokenDetails: einomodel.PromptTokenDetails{
						CachedTokens: 20,
					},
					CompletionTokensDetails: einomodel.CompletionTokensDetails{
						ReasoningTokens: 10,
					},
				},
			},
			wantContent: "hello world",
			wantModel:   "gpt-4o",
			wantPrompt:  100,
			wantComp:    50,
			wantTotal:   150,
			wantCached:  20,
			wantReason:  10,
		},
		{
			name: "no token usage",
			cbOut: &einomodel.CallbackOutput{
				Message: &schema.Message{
					Role:    schema.Assistant,
					Content: "no tokens",
				},
			},
			wantContent: "no tokens",
		},
		{
			name:  "nil message",
			cbOut: &einomodel.CallbackOutput{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meter := agentmeter.New()
			h := einometer.NewAgentMeterHandler(meter)

			h.OnEnd(context.Background(), modelInfo("planner"), tt.cbOut)

			snap := meter.Snapshot()
			require.Len(t, snap.Steps, 1)
			step := snap.Steps[0]
			assert.Equal(t, agentmeter.ClusterCognitive, step.Cluster)
			assert.Equal(t, "planner", step.AgentName)
			assert.Equal(t, tt.wantContent, step.Content)
			assert.Equal(t, tt.wantModel, step.ModelID)
			assert.Equal(t, tt.wantPrompt, step.Usage.PromptTokens)
			assert.Equal(t, tt.wantComp, step.Usage.CompletionTokens)
			assert.Equal(t, tt.wantTotal, step.Usage.TotalTokens)
			assert.Equal(t, tt.wantCached, step.Usage.CachedInputTokens)
			assert.Equal(t, tt.wantReason, step.Usage.ReasoningTokens)
		})
	}
}

func TestOnEnd_ModelOutput_ToolCalls(t *testing.T) {
	meter := agentmeter.New()
	h := einometer.NewAgentMeterHandler(meter)

	cbOut := &einomodel.CallbackOutput{
		Message: &schema.Message{
			Role:    schema.Assistant,
			Content: "",
			ToolCalls: []schema.ToolCall{
				{Function: schema.FunctionCall{Name: "search", Arguments: `{"q":"go"}`}},
				{Function: schema.FunctionCall{Name: "calc", Arguments: `{"x":1}`}},
			},
		},
	}

	h.OnEnd(context.Background(), modelInfo("planner"), cbOut)

	snap := meter.Snapshot()
	require.Len(t, snap.Steps, 1)
	assert.Equal(t, []string{`search{"q":"go"}`, `calc{"x":1}`}, snap.Steps[0].ToolCalls)
}

// --- OnEnd: tools node ---

func TestOnEnd_ToolsNode_RecordsActionSteps(t *testing.T) {
	meter := agentmeter.New()
	h := einometer.NewAgentMeterHandler(meter)

	messages := []*schema.Message{
		{Role: schema.Tool, Content: "result-1", ToolName: "search", ToolCallID: "call-1"},
		{Role: schema.Tool, Content: "result-2", ToolName: "calc", ToolCallID: "call-2"},
	}

	h.OnEnd(context.Background(), toolNodeInfo("tools"), messages)

	snap := meter.Snapshot()
	require.Len(t, snap.Steps, 2)

	assert.Equal(t, agentmeter.ClusterAction, snap.Steps[0].Cluster)
	assert.Equal(t, "search", snap.Steps[0].ToolName)
	assert.Equal(t, "call-1", snap.Steps[0].ToolCallID)
	assert.Equal(t, "result-1", snap.Steps[0].Content)

	assert.Equal(t, agentmeter.ClusterAction, snap.Steps[1].Cluster)
	assert.Equal(t, "calc", snap.Steps[1].ToolName)
}

func TestOnEnd_ToolsNode_IgnoresNonToolMessages(t *testing.T) {
	meter := agentmeter.New()
	h := einometer.NewAgentMeterHandler(meter)

	messages := []*schema.Message{
		{Role: schema.User, Content: "not a tool result"},
	}

	h.OnEnd(context.Background(), toolNodeInfo("tools"), messages)

	assert.Empty(t, meter.Snapshot().Steps)
}

// --- OnEnd: chain finalize ---

func TestOnEnd_Chain_Finalizes(t *testing.T) {
	meter := agentmeter.New()
	h := einometer.NewAgentMeterHandler(meter)

	meter.Reset("run-42")
	// Chain finalize is triggered when the chain's output is a model response.
	cbOut := &einomodel.CallbackOutput{
		Message: &schema.Message{Role: schema.Assistant, Content: "done"},
	}
	h.OnEnd(context.Background(), chainInfo("run-42"), cbOut)

	history := meter.History()
	require.Len(t, history, 1)
	assert.Equal(t, "run-42", history[0].Label)
}

func TestOnEnd_Chain_RecordsCompletionStep(t *testing.T) {
	meter := agentmeter.New()
	h := einometer.NewAgentMeterHandler(meter)

	meter.Reset("run-1")
	cbOut := &einomodel.CallbackOutput{
		Message: &schema.Message{Role: schema.Assistant, Content: "done"},
	}
	h.OnEnd(context.Background(), chainInfo("run-1"), cbOut)

	history := meter.History()
	require.Len(t, history, 1)
	steps := history[0].Steps
	require.NotEmpty(t, steps)

	last := steps[len(steps)-1]
	assert.Equal(t, agentmeter.ClusterCognitive, last.Cluster)
	assert.Equal(t, "done", last.Content)
}

// --- OnError ---

func TestOnError_RecordsErrorStep(t *testing.T) {
	meter := agentmeter.New()
	h := einometer.NewAgentMeterHandler(meter)

	info := modelInfo("planner")
	h.OnError(context.Background(), info, fmt.Errorf("model call failed"))

	snap := meter.Snapshot()
	require.Len(t, snap.Steps, 1)
	step := snap.Steps[0]
	assert.Equal(t, agentmeter.ClusterError, step.Cluster)
	assert.Equal(t, "model call failed", step.Content)
	assert.Equal(t, "planner", step.AgentName)
}

// --- ModelCalls counter ---

func TestOnEnd_ModelOutput_IncrementsModelCalls(t *testing.T) {
	meter := agentmeter.New()
	h := einometer.NewAgentMeterHandler(meter)

	cbOut := &einomodel.CallbackOutput{
		Message: &schema.Message{Role: schema.Assistant, Content: "a"},
	}

	h.OnEnd(context.Background(), modelInfo("a"), cbOut)
	h.OnEnd(context.Background(), modelInfo("b"), cbOut)

	assert.Equal(t, 2, meter.Snapshot().TokenSummary.ModelCalls)
}

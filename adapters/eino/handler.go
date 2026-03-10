package einometer

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/cloudwego/eino/callbacks"
	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/erlangb/agentmeter"
	"github.com/samber/lo"
)

type timingKey struct{}

type AgentMeterHandler struct {
	meter *agentmeter.Meter
}

func NewAgentMeterHandler(t *agentmeter.Meter) *AgentMeterHandler {
	return &AgentMeterHandler{meter: t}
}

func (a *AgentMeterHandler) OnStart(ctx context.Context, info *callbacks.RunInfo, _ callbacks.CallbackInput) context.Context {
	startTimes, ok := ctx.Value(timingKey{}).(map[string]time.Time)
	if !ok {
		startTimes = make(map[string]time.Time)
	}

	if info.Component == compose.ComponentOfChain {
		a.meter.Reset(info.Name)
		startTimes[info.Name] = time.Now()
	}

	if info.Component == compose.ComponentOfToolsNode {
		startTimes[info.Name] = time.Now()
	}

	if info.Component == compose.ComponentOfGraph {
		startTimes[info.Name] = time.Now()
	}

	return context.WithValue(ctx, timingKey{}, startTimes)
}

func (a *AgentMeterHandler) OnStartWithStreamInput(ctx context.Context, info *callbacks.RunInfo, _ *schema.StreamReader[callbacks.CallbackInput]) context.Context {
	if info.Component == compose.ComponentOfChain {
		a.meter.Reset(info.Name)
	}

	return ctx
}

func (a *AgentMeterHandler) OnError(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
	a.meter.Record(agentmeter.AgentStep{
		Role:      "system",
		Cluster:   agentmeter.ClusterError,
		AgentName: info.Name,
		Content:   err.Error(),
	})
	return ctx
}

func (a *AgentMeterHandler) OnEnd(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
	switch out := output.(type) {

	case *einomodel.CallbackOutput:
		a.recordModel(ctx, info.Name, out)

	case []*schema.Message:
		for _, msg := range out {
			if msg.Role == schema.Tool && info.Component == compose.ComponentOfToolsNode {
				a.meter.Record(agentmeter.AgentStep{
					Role:       "tool",
					Cluster:    agentmeter.ClusterAction,
					AgentName:  info.Name,
					ToolName:   msg.ToolName,
					ToolCallID: msg.ToolCallID,
					Content:    msg.Content,
				})
			}

			if msg.Role == schema.User {
				a.meter.Record(agentmeter.AgentStep{
					Role:      "user",
					Cluster:   agentmeter.ClusterMessage,
					AgentName: info.Name,
					Content:   msg.Content,
				})
			}
		}
	}

	if info.Component == compose.ComponentOfChain || info.Component == compose.ComponentOfGraph {
		a.meter.Finalize()
	}

	return ctx
}

func (a *AgentMeterHandler) OnEndWithStreamOutput(
	ctx context.Context,
	info *callbacks.RunInfo,
	output *schema.StreamReader[callbacks.CallbackOutput],
) context.Context {
	go func() {
		defer output.Close()

		var final *einomodel.CallbackOutput
		for {
			frame, err := output.Recv()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return
			}
			if cb, ok := frame.(*einomodel.CallbackOutput); ok {
				final = cb
			}
		}

		if final != nil {
			a.recordModel(ctx, info.Name, final)
		}
	}()

	return ctx
}

func (a *AgentMeterHandler) recordModel(ctx context.Context, name string, cbOut *einomodel.CallbackOutput) {
	step := agentmeter.AgentStep{
		Role:      "model",
		Cluster:   agentmeter.ClusterCognitive,
		AgentName: name,
	}

	if startTimes, ok := ctx.Value(timingKey{}).(map[string]time.Time); ok {
		if startTime, exists := startTimes[name]; exists {
			step.Duration = time.Since(startTime)
		}
	}

	if cbOut.Config != nil {
		step.ModelID = cbOut.Config.Model
	}

	if cbOut.Message != nil {
		step.Role = agentmeter.StepRole(cbOut.Message.Role)
		step.ToolName = cbOut.Message.ToolName
		step.ToolCallID = cbOut.Message.ToolCallID
		step.Content = cbOut.Message.Content
		step.ToolCalls = lo.Map(cbOut.Message.ToolCalls, func(item schema.ToolCall, _ int) string {
			return item.Function.Name + item.Function.Arguments
		})
	}

	if cbOut.TokenUsage != nil {
		step.Usage = agentmeter.TokenUsage{
			PromptTokens:      cbOut.TokenUsage.PromptTokens,
			CompletionTokens:  cbOut.TokenUsage.CompletionTokens,
			TotalTokens:       cbOut.TokenUsage.TotalTokens,
			CachedInputTokens: cbOut.TokenUsage.PromptTokenDetails.CachedTokens,
			ReasoningTokens:   cbOut.TokenUsage.CompletionTokensDetails.ReasoningTokens,
		}
	}

	if cbOut.Message != nil {
		step.Content = cbOut.Message.Content
	}

	a.meter.Record(step)
}

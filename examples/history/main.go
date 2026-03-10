// Package main demonstrates multi-run history with agentmeter.
//
// It records two separate agent runs into the same Meter and uses
// PrintHistory to render them together with an aggregate cost summary.
//
// Run with:
//
//	go run ./examples/history/
package main

import (
	"os"
	"time"

	"github.com/erlangb/agentmeter"
	"github.com/erlangb/agentmeter/pricing"
	"github.com/erlangb/agentmeter/reasoning"
)

func main() {
	meter := agentmeter.New(pricing.WithDefaultPricing())

	// ── Run 1: weather lookup ─────────────────────────────────────────────────
	meter.Reset("weather-run-1")

	// Step 1: model decides to call a tool.
	start := time.Now()
	time.Sleep(10 * time.Millisecond)
	meter.Record(agentmeter.AgentStep{
		Role:      "model",
		Cluster:   agentmeter.ClusterCognitive,
		AgentName: "weather-agent",
		ModelID:   "gpt-4o",
		StartedAt: start,
		Content:   "I need to fetch the current weather. Let me call get_weather.",
		ToolCalls: []string{`get_weather({"city":"Rome"})`},
		Usage: agentmeter.TokenUsage{
			PromptTokens:     512,
			CompletionTokens: 48,
			TotalTokens:      560,
		},
	})

	// Step 2: tool executes and returns a result.
	start = time.Now()
	time.Sleep(55 * time.Millisecond)
	meter.Record(agentmeter.AgentStep{
		Role:       "tool",
		Cluster:    agentmeter.ClusterAction,
		AgentName:  "weather-agent",
		StartedAt:  start,
		ToolName:   "get_weather",
		ToolCallID: "call-abc123",
		ToolInput:  `{"city":"Rome"}`,
		Content:    `{"temp_c":22,"condition":"Sunny","humidity":45}`,
	})

	// Step 3: model produces the final answer.
	start = time.Now()
	time.Sleep(8 * time.Millisecond)
	meter.Record(agentmeter.AgentStep{
		Role:      "model",
		Cluster:   agentmeter.ClusterCognitive,
		AgentName: "weather-agent",
		ModelID:   "gpt-4o",
		StartedAt: start,
		Content:   "The current weather in Rome is 22 °C and Sunny with 45% humidity.",
		Usage: agentmeter.TokenUsage{
			PromptTokens:      620,
			CompletionTokens:  32,
			TotalTokens:       652,
			CachedInputTokens: 480,
		},
	})

	meter.Finalize()

	// ── Run 2: single-step follow-up ──────────────────────────────────────────
	meter.Reset("weather-run-2")

	start = time.Now()
	time.Sleep(8 * time.Millisecond)
	meter.Record(agentmeter.AgentStep{
		Role:      "model",
		Cluster:   agentmeter.ClusterCognitive,
		AgentName: "weather-agent",
		ModelID:   "gpt-4o",
		StartedAt: start,
		Content:   "The current weather in Rome is 22 °C and Sunny with 45% humidity.",
		Usage: agentmeter.TokenUsage{
			PromptTokens:      620,
			CompletionTokens:  32,
			TotalTokens:       652,
			CachedInputTokens: 480,
		},
	})
	meter.Finalize()

	printer := reasoning.NewPrinter(os.Stdout)
	printer.PrintHistory(meter.History())
}
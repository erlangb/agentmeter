// Package main is the minimal agentmeter example.
//
// It records one agent run — a model call followed by a tool call — then
// prints the reasoning trace and token summary. Start here if you are new
// to agentmeter.
//
// Run with:
//
//	go run ./examples/basic/
package main

import (
	"os"
	"time"

	"github.com/erlangb/agentmeter"
	"github.com/erlangb/agentmeter/pricing"
	"github.com/erlangb/agentmeter/reasoning"
)

func main() {
	// 1. Create a meter. WithDefaultPricing enables cost estimation for
	//    OpenAI, Anthropic, and Gemini models out of the box.
	meter := agentmeter.New(pricing.WithDefaultPricing())

	// 2. Start a run. The label identifies this run in history and output.
	meter.Reset("search-agent")

	// 3. Record steps. Set StartedAt before the blocking call so that
	//    Record() can auto-compute the step duration.

	// Step 1: model decides what to do.
	start := time.Now()
	time.Sleep(20 * time.Millisecond) // simulates model latency
	meter.Record(agentmeter.AgentStep{
		Role:      "model",
		Cluster:   agentmeter.ClusterCognitive,
		AgentName: "search-agent",
		ModelID:   "gpt-4o",
		StartedAt: start,
		Content:   "I need to search for recent Go releases.",
		ToolCalls: []string{`web_search({"q":"Go 1.23 release notes"})`},
		Usage: agentmeter.TokenUsage{
			PromptTokens:     200,
			CompletionTokens: 30,
		},
	})

	// Step 2: tool returns its result.
	start = time.Now()
	time.Sleep(80 * time.Millisecond) // simulates network call
	meter.Record(agentmeter.AgentStep{
		Role:      "tool",
		Cluster:   agentmeter.ClusterAction,
		AgentName: "search-agent",
		StartedAt: start,
		ToolName:  "web_search",
		Content:   "Go 1.23 was released in August 2024 with improvements to iterators...",
	})

	// Step 3: model produces the final answer.
	start = time.Now()
	time.Sleep(15 * time.Millisecond)
	meter.Record(agentmeter.AgentStep{
		Role:      "model",
		Cluster:   agentmeter.ClusterCognitive,
		AgentName: "search-agent",
		ModelID:   "gpt-4o",
		StartedAt: start,
		Content:   "Go 1.23 introduced range-over-function iterators and improved timer precision.",
		Usage: agentmeter.TokenUsage{
			PromptTokens:      450,
			CompletionTokens:  60,
			CachedInputTokens: 180,
		},
	})

	// 4. Finalize the run. This seals it and records the wall-clock duration.
	meter.Finalize()

	// 5. Read the snapshot and print it.
	snap := meter.Snapshot()
	printer := reasoning.NewPrinter(os.Stdout)
	printer.Print(snap)
}
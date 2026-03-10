package pricing

import (
	"github.com/erlangb/agentmeter"
	"github.com/samber/lo"
)

type DefaultPricing struct {
	// InputPer1M is the cost per one million input (prompt) tokens.
	InputPer1M float64
	// OutputPer1M is the cost per one million output (completion) tokens.
	OutputPer1M float64
	// CachedInputPer1M is the cost per one million cached input tokens.
	// Cached tokens are billed at a discount compared to InputPer1M.
	CachedInputPer1M float64

	// CacheWritePer1M is the cost per one million cache write tokens.
	// Cache writes are billed at a premium compared to standard input tokens.
	CacheWritePer1M float64
}

func DefaultAnthropicPricing() map[string]DefaultPricing {
	return map[string]DefaultPricing{
		"claude-4-0-opus":   {InputPer1M: 15.00, OutputPer1M: 75.00, CacheWritePer1M: 18.75, CachedInputPer1M: 1.50},
		"claude-4-0-sonnet": {InputPer1M: 3.00, OutputPer1M: 15.00, CacheWritePer1M: 3.75, CachedInputPer1M: 0.30},
		"claude-4-0-haiku":  {InputPer1M: 0.25, OutputPer1M: 1.25, CacheWritePer1M: 0.30, CachedInputPer1M: 0.03},
	}
}

func DefaultGeminiPricing() map[string]DefaultPricing {
	return map[string]DefaultPricing{
		"gemini-2.0-pro":   {InputPer1M: 1.25, OutputPer1M: 5.00, CachedInputPer1M: 0.625},
		"gemini-2.0-flash": {InputPer1M: 0.10, OutputPer1M: 0.40, CachedInputPer1M: 0.05},
		"gemini-1.5-flash": {InputPer1M: 0.075, OutputPer1M: 0.30},
	}
}

// DefaultOpenAIPricing returns a pricing table for commonly used OpenAI models.
// Prices reflect published rates as of early 2026 and may need updating as
// OpenAI adjusts its pricing.
func DefaultOpenAIPricing() map[string]DefaultPricing {
	return map[string]DefaultPricing{
		"gpt-4.1":       {InputPer1M: 2.00, OutputPer1M: 8.00, CachedInputPer1M: 0.50},
		"gpt-4.1-mini":  {InputPer1M: 0.40, OutputPer1M: 1.60, CachedInputPer1M: 0.10},
		"gpt-4.1-nano":  {InputPer1M: 0.10, OutputPer1M: 0.40, CachedInputPer1M: 0.025},
		"gpt-4o":        {InputPer1M: 2.50, OutputPer1M: 10.00, CachedInputPer1M: 1.25},
		"gpt-4o-mini":   {InputPer1M: 0.15, OutputPer1M: 0.60, CachedInputPer1M: 0.075},
		"gpt-4-turbo":   {InputPer1M: 10.00, OutputPer1M: 30.00, CachedInputPer1M: 5.00},
		"gpt-3.5-turbo": {InputPer1M: 0.50, OutputPer1M: 1.50},
		"o1":            {InputPer1M: 15.00, OutputPer1M: 60.00, CachedInputPer1M: 7.50},
		"o1-mini":       {InputPer1M: 3.00, OutputPer1M: 12.00, CachedInputPer1M: 1.50},
		"o3-mini":       {InputPer1M: 1.10, OutputPer1M: 4.40, CachedInputPer1M: 0.55},
	}
}

// RegistryCostFunc looks up pricing in a compiled table of various providers.
func RegistryCostFunc(table map[string]DefaultPricing) agentmeter.CostFunc {
	return func(summary agentmeter.TokenSummary) float64 {
		var total float64

		for modelID, u := range summary.ByModel {
			p, ok := table[modelID]
			if !ok {
				continue
			}

			standardInput := float64(u.PromptTokens-u.CachedInputTokens-u.CacheWriteTokens) / 1e6
			cacheRead := float64(u.CachedInputTokens) / 1e6
			cacheWrite := float64(u.CacheWriteTokens) / 1e6
			output := float64(u.CompletionTokens+u.ReasoningTokens) / 1e6

			total += (standardInput * p.InputPer1M) +
				(cacheRead * p.CachedInputPer1M) +
				(cacheWrite * p.CacheWritePer1M) +
				(output * p.OutputPer1M)
		}
		return total
	}
}

// DefaultRegistryCostFunc returns a CostFunc covering all built-in providers.
// Use this when you need the raw function (e.g. in tests or custom wiring).
// Use WithDefaultPricing when configuring a Meter via New().
func DefaultRegistryCostFunc() agentmeter.CostFunc {
	return RegistryCostFunc(lo.Assign(
		DefaultOpenAIPricing(),
		DefaultAnthropicPricing(),
		DefaultGeminiPricing(),
	))
}

// WithDefaultPricing wires in a master table of all known major providers.
func WithDefaultPricing() agentmeter.Option {
	return agentmeter.WithCostFunc(DefaultRegistryCostFunc())
}

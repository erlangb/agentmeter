package pricing_test

import (
	"testing"

	"github.com/erlangb/agentmeter"
	"github.com/erlangb/agentmeter/pricing"
	"github.com/stretchr/testify/assert"
)

func TestOpenAICostFunc_MultiModel(t *testing.T) {
	costFn := pricing.DefaultRegistryCostFunc()

	// gpt-4o-mini: $0.15/M input, $0.60/M output, $0.075/M cached
	// gpt-4o:      $2.50/M input, $10.00/M output, $1.25/M cached

	tests := []struct {
		name    string
		summary agentmeter.TokenSummary
		wantUSD float64
	}{
		{
			name: "single known model via ByModel",
			summary: agentmeter.TokenSummary{
				ByModel: map[string]agentmeter.TokenUsage{
					"gpt-4o-mini": {PromptTokens: 1_000_000, CompletionTokens: 1_000_000},
				},
			},
			// 1M input * $0.15 + 1M output * $0.60
			wantUSD: 0.75,
		},
		{
			name: "two different models summed correctly",
			summary: agentmeter.TokenSummary{
				ByModel: map[string]agentmeter.TokenUsage{
					"gpt-4o-mini": {PromptTokens: 1_000_000, CompletionTokens: 500_000},
					"gpt-4o":      {PromptTokens: 500_000, CompletionTokens: 200_000},
				},
			},
			// mini: 1M*0.15 + 0.5M*0.60 = 0.15 + 0.30 = 0.45
			// 4o:   0.5M*2.50 + 0.2M*10.00 = 1.25 + 2.00 = 3.25
			wantUSD: 3.70,
		},
		{
			name: "unknown model in ByModel returns 0",
			summary: agentmeter.TokenSummary{
				ByModel: map[string]agentmeter.TokenUsage{
					"unknown-model-xyz": {PromptTokens: 1_000_000, CompletionTokens: 1_000_000},
				},
			},
			wantUSD: 0,
		},
		{
			name: "mixed known and unknown — cost for known model only",
			summary: agentmeter.TokenSummary{
				ByModel: map[string]agentmeter.TokenUsage{
					"gpt-4o-mini":       {PromptTokens: 1_000_000, CompletionTokens: 0},
					"unknown-model-xyz": {PromptTokens: 1_000_000, CompletionTokens: 1_000_000},
				},
			},
			// only gpt-4o-mini: 1M*0.15 = 0.15
			wantUSD: 0.15,
		},
		{
			name: "cached tokens reduce cost via ByModel",
			summary: agentmeter.TokenSummary{
				ByModel: map[string]agentmeter.TokenUsage{
					"gpt-4o-mini": {
						PromptTokens:      1_000_000,
						CachedInputTokens: 500_000,
						CompletionTokens:  0,
					},
				},
			},
			// non-cached: 0.5M*0.15 = 0.075; cached: 0.5M*0.075 = 0.0375 → total 0.1125
			wantUSD: 0.1125,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := costFn(tc.summary)
			assert.InDelta(t, tc.wantUSD, got, 1e-9, "cost mismatch")
		})
	}
}

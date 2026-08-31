//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetModelPricing_CursorPrefixedUsesCursorListPrices(t *testing.T) {
	svc := newTestBillingService()

	tests := []struct {
		model      string
		input      float64
		output     float64
		cacheRead  float64
		cacheWrite float64
	}{
		{model: "cursor-composer-2.5", input: 0.5e-6, output: 2.5e-6, cacheRead: 0.2e-6},
		{model: "cursor-composer-2.5-fast", input: 3e-6, output: 15e-6, cacheRead: 0.5e-6},
		{model: "cursor-grok-4.6", input: 2e-6, output: 6e-6, cacheRead: 0.5e-6},
		{model: "cursor-grok-4.6-fast", input: 4e-6, output: 12e-6, cacheRead: 1e-6},
		{model: "cursor-grok-4.5-fast", input: 4e-6, output: 18e-6, cacheRead: 1e-6},
		{model: "cursor-gpt-5.4-mini-fast", input: 1.5e-6, output: 9e-6, cacheRead: 0.15e-6},
		{model: "cursor-kimi-k3", input: 3e-6, output: 15e-6, cacheRead: 0.3e-6},
		{model: "cursor-claude-opus-4-7-fast", input: 30e-6, output: 150e-6, cacheRead: 3e-6, cacheWrite: 37.5e-6},
		{model: "cursor-claude-opus-5-thinking-fast", input: 10e-6, output: 50e-6, cacheRead: 1e-6, cacheWrite: 12.5e-6},
		{model: "CursorProxy/cursor-gpt-5.6-sol-fast", input: 8e-6, output: 40e-6, cacheRead: 0.8e-6, cacheWrite: 10e-6},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			pricing, err := svc.GetModelPricing(tt.model)
			require.NoError(t, err, tt.model)
			require.InDelta(t, tt.input, pricing.InputPricePerToken, 1e-12, "input")
			require.InDelta(t, tt.output, pricing.OutputPricePerToken, 1e-12, "output")
			require.InDelta(t, tt.cacheRead, pricing.CacheReadPricePerToken, 1e-12, "cache read")
			require.InDelta(t, tt.cacheWrite, pricing.CacheCreationPricePerToken, 1e-12, "cache write")
			require.True(t, svc.HasIdentifiedTokenPricing(tt.model))
		})
	}
}

func TestGetModelPricing_UnprefixedGrokComposerStaysOnBuildCard(t *testing.T) {
	svc := newTestBillingService()
	pricing, err := svc.GetModelPricing("composer-2.5")
	require.NoError(t, err)
	require.InDelta(t, 1e-6, pricing.InputPricePerToken, 1e-12)
	require.InDelta(t, 2e-6, pricing.OutputPricePerToken, 1e-12)
}

func TestGetMappedModel_StripsCursorClientPrefixWithCapture(t *testing.T) {
	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"model_mapping": map[string]any{
				"cursor-grok-*": "cursor-grok-*",
				"cursor-*":      "*",
			},
		},
	}

	require.Equal(t, "kimi-k3", account.GetMappedModel("cursor-kimi-k3"))
	require.Equal(t, "kimi-k3-fast", account.GetMappedModel("cursor-kimi-k3-fast"))
	require.Equal(t, "composer-2.5-fast", account.GetMappedModel("cursor-composer-2.5-fast"))
	require.Equal(t, "cursor-grok-4.6", account.GetMappedModel("cursor-grok-4.6"))
	require.Equal(t, "cursor-grok-4.6-fast", account.GetMappedModel("cursor-grok-4.6-fast"))
}

func TestUsageBillingModelCandidates_PrefersCursorPrefixedOriginal(t *testing.T) {
	candidates := usageBillingModelCandidates(
		"kimi-k3-fast",
		"kimi-k3-fast",
		"kimi-k3-fast",
		"cursor-kimi-k3-fast",
		"kimi-k3-fast",
	)
	require.Equal(t, "cursor-kimi-k3-fast", candidates[0])
	require.Contains(t, candidates, "kimi-k3-fast")
}

func TestNormalizeCursorBillingTokens_ClaudeUsesDisjointBuckets(t *testing.T) {
	tokens := normalizeCursorBillingTokens("cursor-claude-4.5-sonnet", UsageTokens{
		InputTokens:         100,
		CacheReadTokens:     200,
		CacheCreationTokens: 300,
		OutputTokens:        50,
	})
	require.Equal(t, 100, tokens.InputTokens)
	require.Equal(t, 200, tokens.CacheReadTokens)
	require.Equal(t, 300, tokens.CacheCreationTokens)
	require.Equal(t, 50, tokens.OutputTokens)
}

func TestNormalizeCursorBillingTokens_CodexMergesDisjointCounters(t *testing.T) {
	tokens := normalizeCursorBillingTokens("cursor-gpt-5.4", UsageTokens{
		InputTokens:         200,
		CacheReadTokens:     800,
		OutputTokens:        50,
	})
	require.Equal(t, 200, tokens.InputTokens)
	require.Equal(t, 800, tokens.CacheReadTokens)
	require.Zero(t, tokens.CacheCreationTokens)
	require.Equal(t, 50, tokens.OutputTokens)
}

func TestNormalizeCursorBillingTokens_CodexSubtractsOpenAIStyleTotals(t *testing.T) {
	tokens := normalizeCursorBillingTokens("cursor-gpt-5.4", UsageTokens{
		InputTokens:         1000,
		CacheReadTokens:     800,
		CacheCreationTokens: 100,
		OutputTokens:        50,
	})
	require.Equal(t, 100, tokens.InputTokens)
	require.Equal(t, 800, tokens.CacheReadTokens)
	require.Equal(t, 100, tokens.CacheCreationTokens)
}

func TestFinalizeCursorBillingTokens_ProxyPromptUsesCacheReadWhenNoBreakdown(t *testing.T) {
	tokens := finalizeCursorBillingTokens("cursor-grok-4.6-fast", UsageTokens{
		InputTokens:  1000,
		OutputTokens: 50,
	})
	require.Zero(t, tokens.InputTokens)
	require.Equal(t, 1000, tokens.CacheReadTokens)
	require.Zero(t, tokens.CacheCreationTokens)
}

func TestFinalizeCursorBillingTokens_KeepsDashboardBreakdown(t *testing.T) {
	tokens := finalizeCursorBillingTokens("cursor-gpt-5.4", UsageTokens{
		InputTokens:     100,
		CacheReadTokens: 200,
		OutputTokens:    50,
	})
	require.Equal(t, 100, tokens.InputTokens)
	require.Equal(t, 200, tokens.CacheReadTokens)
}

func TestCalculateCost_CursorClaudeMatchesDisjointListPricing(t *testing.T) {
	svc := newTestBillingService()
	cost, err := svc.CalculateCost("cursor-claude-4.5-sonnet", UsageTokens{
		InputTokens:         100,
		CacheReadTokens:     200,
		CacheCreationTokens: 300,
		OutputTokens:        50,
	}, 1)
	require.NoError(t, err)
	expected := 100*3e-6 + 200*0.3e-6 + 300*3.75e-6 + 50*15e-6
	require.InDelta(t, expected, cost.TotalCost, 1e-12)
}

func TestCalculateCost_CursorGPTWithoutCacheUsesCacheReadRate(t *testing.T) {
	svc := newTestBillingService()
	cost, err := svc.CalculateCost("cursor-gpt-5", UsageTokens{
		InputTokens:  200,
		OutputTokens: 20,
	}, 1)
	require.NoError(t, err)
	expected := 200*0.125e-6 + 20*10e-6
	require.InDelta(t, expected, cost.TotalCost, 1e-12)
}

func TestCalculateCost_CursorGPTOpenAIStyleTotalsAvoidDoubleBilling(t *testing.T) {
	svc := newTestBillingService()
	cost, err := svc.CalculateCost("cursor-composer-2.5", UsageTokens{
		InputTokens:     1000,
		CacheReadTokens: 800,
		OutputTokens:    50,
	}, 1)
	require.NoError(t, err)
	expected := 200*0.5e-6 + 800*0.2e-6 + 50*2.5e-6
	require.InDelta(t, expected, cost.TotalCost, 1e-12)
}

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestOpenAIUsageFromGJSONParsesCostInUsdTicks(t *testing.T) {
	usage, ok := openAIUsageFromGJSON(gjson.Parse(`{
		"input_tokens": 199,
		"output_tokens": 1,
		"total_tokens": 200,
		"input_tokens_details": {"cached_tokens": 50},
		"cost_in_usd_ticks": 37756000
	}`))
	require.True(t, ok)
	require.Equal(t, 199, usage.InputTokens)
	require.Equal(t, 50, usage.CacheReadInputTokens)
	require.Equal(t, int64(37756000), usage.CostInUsdTicks)
}

func TestApplyGrokUpstreamReportedCostOverridesTotals(t *testing.T) {
	cost := &CostBreakdown{InputCost: 1, OutputCost: 2, CacheReadCost: 3, TotalCost: 6, ActualCost: 6}
	applyGrokUpstreamReportedCost(
		&Account{Platform: PlatformGrok},
		OpenAIUsage{CostInUsdTicks: 37_756_000},
		cost,
		2,
	)
	require.InDelta(t, 0.0037756, cost.TotalCost, 1e-12)
	require.InDelta(t, 0.0075512, cost.ActualCost, 1e-12)
	require.InDelta(t, 1, cost.InputCost, 1e-12)
	require.InDelta(t, 3, cost.CacheReadCost, 1e-12)
}

func TestApplyGrokUpstreamReportedCostIgnoresNonGrok(t *testing.T) {
	cost := &CostBreakdown{TotalCost: 6, ActualCost: 6}
	applyGrokUpstreamReportedCost(
		&Account{Platform: PlatformOpenAI},
		OpenAIUsage{CostInUsdTicks: 37_756_000},
		cost,
		1,
	)
	require.InDelta(t, 6, cost.TotalCost, 1e-12)
}

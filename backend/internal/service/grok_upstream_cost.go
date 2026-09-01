package service

import "github.com/Wei-Shaw/sub2api/internal/pkg/xai"

// applyGrokUpstreamReportedCost replaces estimated token dollars with xAI's
// per-request cost_in_usd_ticks when the account is Grok and the response
// included a positive tick count. Component costs stay as local estimates so
// cache/input/output rows remain inspectable; TotalCost/ActualCost become the
// amount xAI actually billed (then scaled by the same rate multiplier).
func applyGrokUpstreamReportedCost(account *Account, usage OpenAIUsage, cost *CostBreakdown, rateMultiplier float64) {
	if cost == nil || account == nil || !account.IsGrok() {
		return
	}
	usd, ok := xai.CostUSDFromTicks(usage.CostInUsdTicks)
	if !ok {
		return
	}
	if rateMultiplier < 0 {
		rateMultiplier = 0
	}
	cost.TotalCost = usd
	cost.ActualCost = usd * rateMultiplier
}

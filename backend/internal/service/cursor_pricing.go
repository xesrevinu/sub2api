package service

import "strings"

// Cursor list prices are USD per million tokens from
// https://cursor.com/docs/models-and-pricing. Converted to USD/token here
// to match LiteLLM / BillingService units.
//
// Fast is a distinct Cursor series (model id suffix -fast), not OpenAI
// service_tier. Unprefixed Grok Composer aliases stay on the xAI Build card.

type cursorListCost struct {
	input      float64
	output     float64
	cacheRead  float64
	cacheWrite float64
}

const cursorFastCostMultiplier = 2.0

var cursorListPrices = map[string]cursorListCost{
	"grok-4.6":             {input: 2, output: 6, cacheRead: 0.5},
	"grok-4.5":             {input: 2, output: 6, cacheRead: 0.5},
	"grok-4.5-fast":        {input: 4, output: 18, cacheRead: 1},
	"composer-2.5":         {input: 0.5, output: 2.5, cacheRead: 0.2},
	"composer-2.5-fast":    {input: 3, output: 15, cacheRead: 0.5},
	"claude-4-sonnet":      {input: 3, output: 15, cacheRead: 0.3, cacheWrite: 3.75},
	"claude-4.5-opus":      {input: 5, output: 25, cacheRead: 0.5, cacheWrite: 6.25},
	"claude-4.5-sonnet":    {input: 3, output: 15, cacheRead: 0.3, cacheWrite: 3.75},
	"claude-4.6-opus":      {input: 5, output: 25, cacheRead: 0.5, cacheWrite: 6.25},
	"claude-4.6-sonnet":    {input: 3, output: 15, cacheRead: 0.3, cacheWrite: 3.75},
	"claude-opus-4-7":      {input: 5, output: 25, cacheRead: 0.5, cacheWrite: 6.25},
	"claude-opus-4-7-fast": {input: 30, output: 150, cacheRead: 3, cacheWrite: 37.5},
	"claude-opus-4-8":      {input: 5, output: 25, cacheRead: 0.5, cacheWrite: 6.25},
	"claude-opus-5":        {input: 5, output: 25, cacheRead: 0.5, cacheWrite: 6.25},
	"claude-sonnet-5":      {input: 2, output: 10, cacheRead: 0.2, cacheWrite: 2.5},
	"claude-fable-5":       {input: 10, output: 50, cacheRead: 1, cacheWrite: 12.5},
	"gemini-3-flash":       {input: 0.5, output: 3, cacheRead: 0.05},
	"gemini-3.1-pro":       {input: 2, output: 12, cacheRead: 0.2},
	"gemini-3.5-flash":     {input: 1.5, output: 9, cacheRead: 0.15},
	"gemini-3.6-flash":     {input: 1.5, output: 7.5, cacheRead: 0.15},
	"gemini-3.7-flash":     {input: 0.75, output: 3.5, cacheRead: 0.075},
	"glm-5.2":              {input: 1.4, output: 4.4, cacheRead: 0.26},
	"gpt-5":                {input: 1.25, output: 10, cacheRead: 0.125},
	"gpt-5-fast":           {input: 2.5, output: 20, cacheRead: 0.25},
	"gpt-5-mini":           {input: 0.25, output: 2, cacheRead: 0.025},
	"gpt-5.2":              {input: 1.75, output: 14, cacheRead: 0.175},
	"gpt-5.3-codex":        {input: 1.75, output: 14, cacheRead: 0.175},
	"gpt-5.4":              {input: 2.5, output: 15, cacheRead: 0.25},
	"gpt-5.4-mini":         {input: 0.75, output: 4.5, cacheRead: 0.075},
	"gpt-5.4-nano":         {input: 0.2, output: 1.25, cacheRead: 0.02},
	"gpt-5.5":              {input: 5, output: 30, cacheRead: 0.5},
	"gpt-5.6-luna":         {input: 0.2, output: 1.2, cacheRead: 0.02, cacheWrite: 0.25},
	"gpt-5.6-sol":          {input: 4, output: 20, cacheRead: 0.4, cacheWrite: 5},
	"gpt-5.6-terra":        {input: 2, output: 12, cacheRead: 0.2, cacheWrite: 2.5},
	"kimi-k2.7-code":       {input: 0.95, output: 4, cacheRead: 0.19},
	"kimi-k3":              {input: 3, output: 15, cacheRead: 0.3},
}

func cursorListPricing(model string) *ModelPricing {
	series, ok := cursorClientSeriesID(model)
	if !ok {
		return nil
	}
	cost, ok := cursorListCostForSeries(series)
	if !ok {
		return nil
	}
	return cursorCostToModelPricing(cost)
}

func cursorClientSeriesID(model string) (string, bool) {
	id := strings.ToLower(strings.TrimSpace(model))
	if id == "" {
		return "", false
	}
	if idx := strings.LastIndex(id, "/"); idx >= 0 {
		id = id[idx+1:]
	}
	if !strings.HasPrefix(id, "cursor-") {
		return "", false
	}
	id = strings.TrimPrefix(id, "cursor-")
	if id == "" || id == "auto" {
		return "", false
	}
	fast := strings.HasSuffix(id, "-fast")
	stem := id
	if fast {
		stem = strings.TrimSuffix(id, "-fast")
	}
	stem = strings.ReplaceAll(stem, "-thinking", "")
	stem = strings.TrimSuffix(stem, "-minimal")
	for _, suffix := range []string{"-none", "-low", "-medium", "-high", "-xhigh", "-max"} {
		if strings.HasSuffix(stem, suffix) {
			stem = strings.TrimSuffix(stem, suffix)
			break
		}
	}
	stem = strings.Trim(stem, "-")
	if stem == "" {
		return "", false
	}
	if fast {
		return stem + "-fast", true
	}
	return stem, true
}

func cursorListCostForSeries(series string) (cursorListCost, bool) {
	if cost, ok := cursorListPrices[series]; ok {
		return cost, true
	}
	if !strings.HasSuffix(series, "-fast") {
		return cursorListCost{}, false
	}
	base := strings.TrimSuffix(series, "-fast")
	cost, ok := cursorListPrices[base]
	if !ok {
		return cursorListCost{}, false
	}
	return scaleCursorListCost(cost, cursorFastCostMultiplier), true
}

func scaleCursorListCost(cost cursorListCost, multiplier float64) cursorListCost {
	return cursorListCost{
		input:      cost.input * multiplier,
		output:     cost.output * multiplier,
		cacheRead:  cost.cacheRead * multiplier,
		cacheWrite: cost.cacheWrite * multiplier,
	}
}

func cursorCostToModelPricing(cost cursorListCost) *ModelPricing {
	return &ModelPricing{
		InputPricePerToken:         cost.input * 1e-6,
		OutputPricePerToken:        cost.output * 1e-6,
		CacheReadPricePerToken:     cost.cacheRead * 1e-6,
		CacheCreationPricePerToken: cost.cacheWrite * 1e-6,
		SupportsCacheBreakdown:     false,
	}
}

func isCursorClientModelID(model string) bool {
	id := strings.ToLower(strings.TrimSpace(model))
	if idx := strings.LastIndex(id, "/"); idx >= 0 {
		id = id[idx+1:]
	}
	return strings.HasPrefix(id, "cursor-")
}

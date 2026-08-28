package service

import (
	"strings"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/tidwall/gjson"
)

// reasoningSummaryProbe compares streamed reasoning_summary_text deltas against
// the terminal summary text. A mismatch usually means dropped SSE events; a
// ~200-char summary ending in "..." is the xAI/CLI hard cap we have seen in
// Grok sessions.
type reasoningSummaryProbe struct {
	deltaByItem map[string]*strings.Builder
}

func newReasoningSummaryProbe() *reasoningSummaryProbe {
	return &reasoningSummaryProbe{deltaByItem: make(map[string]*strings.Builder)}
}

func (p *reasoningSummaryProbe) Observe(account *Account, model, eventType string, data []byte) {
	if p == nil || len(data) == 0 {
		return
	}
	switch strings.TrimSpace(eventType) {
	case "response.reasoning_summary_text.delta":
		id := reasoningSummaryItemID(gjson.GetBytes(data, "item_id").String())
		b := p.deltaByItem[id]
		if b == nil {
			b = new(strings.Builder)
			p.deltaByItem[id] = b
		}
		b.WriteString(gjson.GetBytes(data, "delta").String())
	case "response.reasoning_summary_text.done":
		p.report(account, model, "reasoning_summary_text.done",
			gjson.GetBytes(data, "item_id").String(),
			gjson.GetBytes(data, "text").String())
	case "response.output_item.done":
		item := gjson.GetBytes(data, "item")
		if strings.TrimSpace(item.Get("type").String()) != "reasoning" {
			return
		}
		var summary strings.Builder
		for _, part := range item.Get("summary").Array() {
			if strings.TrimSpace(part.Get("type").String()) == "summary_text" {
				summary.WriteString(part.Get("text").String())
			}
		}
		p.report(account, model, "output_item.done", item.Get("id").String(), summary.String())
	}
}

func (p *reasoningSummaryProbe) report(account *Account, model, source, itemID, summary string) {
	id := reasoningSummaryItemID(itemID)
	deltas := ""
	if b := p.deltaByItem[id]; b != nil {
		deltas = b.String()
	}
	truncated := looksTruncatedReasoningSummary(summary)
	if deltas == "" {
		if !truncated {
			return
		}
		logger.LegacyPrintf("service.openai_gateway",
			"Reasoning summary looks truncated: account=%d model=%s source=%s item=%s summary_runes=%d summary=%q",
			accountIDOrZero(account), model, source, id, utf8.RuneCountInString(summary), clipReasoningLog(summary))
		return
	}
	if deltas == summary && !truncated {
		return
	}
	logger.LegacyPrintf("service.openai_gateway",
		"Reasoning summary mismatch: account=%d model=%s source=%s item=%s delta_runes=%d summary_runes=%d truncated=%v prefix_match=%v delta=%q summary=%q",
		accountIDOrZero(account), model, source, id,
		utf8.RuneCountInString(deltas), utf8.RuneCountInString(summary),
		truncated, strings.HasPrefix(summary, deltas) || strings.HasPrefix(deltas, summary),
		clipReasoningLog(deltas), clipReasoningLog(summary))
}

func reasoningSummaryItemID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return "_"
	}
	return id
}

func accountIDOrZero(account *Account) int64 {
	if account == nil {
		return 0
	}
	return account.ID
}

func looksTruncatedReasoningSummary(text string) bool {
	if !strings.HasSuffix(text, "...") {
		return false
	}
	n := utf8.RuneCountInString(text)
	return n >= 180 && n <= 220
}

func clipReasoningLog(text string) string {
	const maxRunes = 96
	if utf8.RuneCountInString(text) <= maxRunes {
		return text
	}
	runes := []rune(text)
	return string(runes[:maxRunes]) + "…"
}

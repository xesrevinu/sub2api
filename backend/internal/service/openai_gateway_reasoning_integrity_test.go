package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLooksTruncatedReasoningSummary(t *testing.T) {
	t.Parallel()
	require.False(t, looksTruncatedReasoningSummary("short..."))
	require.False(t, looksTruncatedReasoningSummary(strings.Repeat("a", 200)))
	require.True(t, looksTruncatedReasoningSummary(strings.Repeat("a", 200)+"..."))
	require.False(t, looksTruncatedReasoningSummary(strings.Repeat("a", 400)+"..."))
}

func TestReasoningSummaryProbeDetectsDroppedDeltas(t *testing.T) {
	t.Parallel()
	probe := newReasoningSummaryProbe()
	probe.Observe(nil, "grok-4.6", "response.reasoning_summary_text.delta",
		[]byte(`{"type":"response.reasoning_summary_text.delta","item_id":"rs_1","delta":"Let me follow the Multica workflow first"}`))
	probe.Observe(nil, "grok-4.6", "response.reasoning_summary_text.delta",
		[]byte(`{"type":"response.reasoning_summary_text.delta","item_id":"rs_1","delta":" since pack failed"}`))
	probe.Observe(nil, "grok-4.6", "response.output_item.done",
		[]byte(`{"type":"response.output_item.done","item":{"id":"rs_1","type":"reasoning","summary":[{"type":"summary_text","text":"Let me follow the Multica workflow first"}]}}`))
	require.Equal(t, "Let me follow the Multica workflow first since pack failed", probe.deltaByItem["rs_1"].String())
}

func TestReasoningSummaryProbeIgnoresMatchingSummary(t *testing.T) {
	t.Parallel()
	probe := newReasoningSummaryProbe()
	probe.Observe(nil, "grok-4.6", "response.reasoning_summary_text.delta",
		[]byte(`{"delta":"ok","item_id":"rs_1"}`))
	probe.Observe(nil, "grok-4.6", "response.reasoning_summary_text.done",
		[]byte(`{"text":"ok","item_id":"rs_1"}`))
	require.Equal(t, "ok", probe.deltaByItem["rs_1"].String())
}

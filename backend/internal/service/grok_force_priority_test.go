package service

import (
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestGrokForcePriorityServiceTierEnabled(t *testing.T) {
	t.Parallel()

	require.False(t, grokForcePriorityServiceTierEnabled(nil))
	require.False(t, grokForcePriorityServiceTierEnabled(&Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}))
	require.True(t, grokForcePriorityServiceTierEnabled(&Account{Platform: PlatformGrok, Type: AccountTypeOAuth}))
	require.False(t, grokForcePriorityServiceTierEnabled(&Account{Platform: PlatformGrok, Type: AccountTypeAPIKey}))
	require.False(t, grokForcePriorityServiceTierEnabled(&Account{
		Platform: PlatformGrok, Type: AccountTypeOAuth,
		Extra: map[string]any{GrokForcePriorityServiceTierExtraKey: false},
	}))
	require.True(t, grokForcePriorityServiceTierEnabled(&Account{
		Platform: PlatformGrok, Type: AccountTypeAPIKey,
		Extra: map[string]any{GrokForcePriorityServiceTierExtraKey: true},
	}))
	require.False(t, grokForcePriorityServiceTierEnabled(&Account{
		Platform: PlatformGrok, Type: AccountTypeOAuth,
		Extra: map[string]any{GrokForcePriorityServiceTierExtraKey: "true"},
	}))
}

func TestApplyGrokForcePriorityServiceTier(t *testing.T) {
	t.Parallel()

	oauth := &Account{Platform: PlatformGrok, Type: AccountTypeOAuth}
	apiKey := &Account{Platform: PlatformGrok, Type: AccountTypeAPIKey}

	t.Run("injects for oauth default", func(t *testing.T) {
		t.Parallel()
		out, err := applyGrokForcePriorityServiceTier([]byte(`{"model":"grok-4.6"}`), oauth)
		require.NoError(t, err)
		require.Equal(t, "priority", gjson.GetBytes(out, "service_tier").String())
	})

	t.Run("overwrites existing tier", func(t *testing.T) {
		t.Parallel()
		out, err := applyGrokForcePriorityServiceTier([]byte(`{"model":"grok-4.6","service_tier":"flex"}`), oauth)
		require.NoError(t, err)
		require.Equal(t, "priority", gjson.GetBytes(out, "service_tier").String())
	})

	t.Run("skips api key default", func(t *testing.T) {
		t.Parallel()
		body := []byte(`{"model":"grok-4.6"}`)
		out, err := applyGrokForcePriorityServiceTier(body, apiKey)
		require.NoError(t, err)
		require.Equal(t, string(body), string(out))
	})

	t.Run("honors explicit opt-out", func(t *testing.T) {
		t.Parallel()
		body := []byte(`{"model":"grok-4.6"}`)
		out, err := applyGrokForcePriorityServiceTier(body, &Account{
			Platform: PlatformGrok, Type: AccountTypeOAuth,
			Extra: map[string]any{GrokForcePriorityServiceTierExtraKey: false},
		})
		require.NoError(t, err)
		require.Equal(t, string(body), string(out))
	})
}

func TestBuildGrokResponsesRequestForcePriorityServiceTier(t *testing.T) {
	t.Setenv(xai.EnvAllowUnsafeURLOverrides, "true")

	account := &Account{
		Platform: PlatformGrok,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"base_url": "https://xai.test/v1/",
		},
		Extra: map[string]any{GrokForcePriorityServiceTierExtraKey: true},
	}

	req, err := buildGrokResponsesRequest(context.Background(), nil, account, []byte(`{"model":"grok-4.6"}`), "api-key", "", nil)
	require.NoError(t, err)
	require.Equal(t, http.MethodPost, req.Method)

	data, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	require.Equal(t, "priority", gjson.GetBytes(data, "service_tier").String())
}

func TestNormalizeGrokForcePriorityServiceTierExtra(t *testing.T) {
	t.Parallel()

	out, err := normalizeGrokForcePriorityServiceTierExtra(PlatformGrok, map[string]any{
		GrokForcePriorityServiceTierExtraKey: true,
		"keep":                               "yes",
	})
	require.NoError(t, err)
	require.Equal(t, true, out[GrokForcePriorityServiceTierExtraKey])
	require.Equal(t, "yes", out["keep"])

	out, err = normalizeGrokForcePriorityServiceTierExtra(PlatformOpenAI, map[string]any{
		GrokForcePriorityServiceTierExtraKey: true,
	})
	require.NoError(t, err)
	require.NotContains(t, out, GrokForcePriorityServiceTierExtraKey)

	_, err = normalizeGrokForcePriorityServiceTierExtra(PlatformGrok, map[string]any{
		GrokForcePriorityServiceTierExtraKey: "true",
	})
	require.Error(t, err)
}

func TestNormalizeGrokForcePriorityServiceTierUpdateExtra(t *testing.T) {
	t.Parallel()

	account := &Account{Platform: PlatformGrok, Extra: map[string]any{GrokForcePriorityServiceTierExtraKey: false}}

	t.Run("omitted override preserves current value", func(t *testing.T) {
		t.Parallel()
		input := &UpdateAccountInput{Extra: map[string]any{"quota_used": float64(1)}}
		normalized, err := normalizeGrokForcePriorityServiceTierUpdateExtra(account, input, map[string]any{"quota_used": float64(1)})
		require.NoError(t, err)
		require.Equal(t, false, normalized[GrokForcePriorityServiceTierExtraKey])
	})

	t.Run("null removes current override", func(t *testing.T) {
		t.Parallel()
		input := &UpdateAccountInput{Extra: map[string]any{GrokForcePriorityServiceTierExtraKey: nil}}
		normalized, err := normalizeGrokForcePriorityServiceTierUpdateExtra(account, input, map[string]any{GrokForcePriorityServiceTierExtraKey: nil})
		require.NoError(t, err)
		require.NotContains(t, normalized, GrokForcePriorityServiceTierExtraKey)
	})

	t.Run("provided boolean replaces current override", func(t *testing.T) {
		t.Parallel()
		input := &UpdateAccountInput{Extra: map[string]any{GrokForcePriorityServiceTierExtraKey: true}}
		normalized, err := normalizeGrokForcePriorityServiceTierUpdateExtra(account, input, map[string]any{GrokForcePriorityServiceTierExtraKey: true})
		require.NoError(t, err)
		require.Equal(t, true, normalized[GrokForcePriorityServiceTierExtraKey])
	})

	t.Run("malformed override is rejected on update", func(t *testing.T) {
		t.Parallel()
		input := &UpdateAccountInput{Extra: map[string]any{GrokForcePriorityServiceTierExtraKey: "false"}}
		_, err := normalizeGrokForcePriorityServiceTierUpdateExtra(account, input, nil)
		require.Error(t, err)
	})

	t.Run("non grok update strips the key", func(t *testing.T) {
		t.Parallel()
		input := &UpdateAccountInput{Extra: map[string]any{GrokForcePriorityServiceTierExtraKey: true}}
		got, err := normalizeGrokForcePriorityServiceTierUpdateExtra(
			&Account{Platform: PlatformOpenAI},
			input,
			map[string]any{GrokForcePriorityServiceTierExtraKey: true},
		)
		require.NoError(t, err)
		require.NotContains(t, got, GrokForcePriorityServiceTierExtraKey)
	})
}

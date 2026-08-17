package service

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
)

func TestApplyDefaultGrokUpstreamHeadersUsesCLIUserAgent(t *testing.T) {
	t.Setenv(xai.CLIVersionEnv, "")

	req, err := http.NewRequest(http.MethodGet, "https://api.x.ai/v1/responses", nil)
	require.NoError(t, err)
	req.Header.Set("User-Agent", "claude-code/1.2.3")
	req.Header.Set("x-grok-client-version", "none")

	applyDefaultGrokUpstreamHeaders(req)

	require.Equal(t, xai.CLIUserAgent(xai.CLIClientVersion), req.Header.Get("User-Agent"))
	require.Equal(t, xai.CLIClientVersion, req.Header.Get("x-grok-client-version"))
	require.Equal(t, xai.CLIClientIdentifier, req.Header.Get("x-grok-client-identifier"))
}

func TestApplyDefaultGrokUpstreamHeadersHonorsCLIVersionOverride(t *testing.T) {
	t.Setenv(xai.CLIVersionEnv, "0.2.95")

	req, err := http.NewRequest(http.MethodGet, "https://api.x.ai/v1/responses", nil)
	require.NoError(t, err)
	req.Header.Set("User-Agent", "codex_cli_rs/0.144.0")

	applyDefaultGrokUpstreamHeaders(req)

	require.Equal(t, "0.2.95", req.Header.Get("x-grok-client-version"))
	require.Equal(t, xai.CLIUserAgent("0.2.95"), req.Header.Get("User-Agent"))
	require.Equal(t, xai.CLIClientIdentifier, req.Header.Get("x-grok-client-identifier"))
}

func TestApplyGrokOAuthInferenceHeadersMatchesLocalBuild(t *testing.T) {
	t.Setenv(xai.CLIVersionEnv, "")

	account := &Account{
		ID:       42,
		Platform: PlatformGrok,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"email": "owner@example.com",
			"sub":   "user-123",
		},
	}
	headers := make(http.Header)
	applyGrokOAuthInferenceHeaders(headers, account, "grok-4.5", "conv-abc")

	require.Equal(t, xai.CLIUserAgent(xai.CLIClientVersion), headers.Get("User-Agent"))
	require.Equal(t, xai.CLIClientIdentifier, headers.Get("x-grok-client-identifier"))
	require.Equal(t, xai.CLIClientMode, headers.Get("x-grok-client-mode"))
	require.Equal(t, xai.CLIAuthenticateResponse, headers.Get("x-authenticateresponse"))
	require.Equal(t, "grok-4.5", headers.Get("x-grok-model-override"))
	require.Equal(t, "conv-abc", headers.Get(grokConversationIDHeader))
	require.Equal(t, "conv-abc", headers.Get("x-grok-session-id"))
	require.NotEmpty(t, headers.Get("x-grok-req-id"))
	require.Equal(t, grokStableAgentID(42), headers.Get("x-grok-agent-id"))
	require.Equal(t, "true", headers.Get("x-grok-doom-loop-check"))
	require.NotEmpty(t, headers.Get("x-grok-turn-idx"))
	require.Empty(t, headers.Get("x-email"))
	require.Empty(t, headers.Get("x-userid"))
	require.Equal(t, "user-123", headers.Get("x-grok-user-id"))
}

func TestApplyGrokCLIAccountHeadersKeepsEmailForModelsSettings(t *testing.T) {
	account := &Account{
		ID:       42,
		Platform: PlatformGrok,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"email": "owner@example.com",
			"sub":   "user-123",
		},
	}
	headers := make(http.Header)
	applyGrokCLIAccountHeaders(headers, account)

	require.Equal(t, "owner@example.com", headers.Get("x-email"))
	require.Equal(t, "user-123", headers.Get("x-userid"))
	require.Equal(t, "user-123", headers.Get("x-grok-user-id"))
}

func TestResolveGrokUpstreamUserAgentNeverPassthrough(t *testing.T) {
	t.Setenv(xai.CLIVersionEnv, "")
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Request.Header.Set("User-Agent", "claude-cli/2.0.0 (Mac OS; arm64)")

	require.Equal(t, xai.CLIUserAgent(xai.CLIClientVersion), resolveGrokUpstreamUserAgent(c))
	require.Equal(t, xai.CLIUserAgent(xai.CLIClientVersion), resolveGrokUpstreamUserAgent(nil))
}

func TestApplyGrokRuntimeHeadersKeepsCLIUserAgent(t *testing.T) {
	t.Setenv(xai.CLIVersionEnv, "")

	req, err := http.NewRequest(http.MethodPost, "https://cli-chat-proxy.grok.com/v1/responses", nil)
	require.NoError(t, err)
	req.Header.Set("User-Agent", "claude-code/9.9.9")

	applyGrokRuntimeHeaders(req, openAITLSFingerprintRuntime{
		UpstreamUserAgent:  "codex_cli_rs/0.144.0",
		UpstreamOriginator: "codex_cli_rs",
	})

	require.Equal(t, xai.CLIUserAgent(xai.CLIClientVersion), req.Header.Get("User-Agent"))
	require.Equal(t, "codex_cli_rs", req.Header.Get("Originator"))
	require.Equal(t, xai.CLIClientVersion, req.Header.Get("x-grok-client-version"))
}

func TestApplyGrokTLSProfileHeadersAlwaysUsesCLIUserAgent(t *testing.T) {
	t.Setenv(xai.CLIVersionEnv, "")

	req, err := http.NewRequest(http.MethodPost, "https://api.x.ai/v1/responses", nil)
	require.NoError(t, err)
	req.Header.Set("User-Agent", "grok-native/1.0")

	// HEAD Profile is TLS-only; Originator/UserAgent HTTP fields are not present.
	applyGrokTLSProfileHeaders(req, &tlsfingerprint.Profile{Name: "chrome"})

	require.Equal(t, xai.CLIUserAgent(xai.CLIClientVersion), req.Header.Get("User-Agent"))
	require.Equal(t, xai.CLIClientVersion, req.Header.Get("x-grok-client-version"))
}

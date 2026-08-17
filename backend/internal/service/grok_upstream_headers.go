package service

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
)

var grokTurnCounters sync.Map // session -> *uint64

// grokUpstreamUserAgent is kept for compatibility with older Grok request
// tests. Current requests use the pinned default UA from this package.
const grokUpstreamUserAgent = "sub2api-grok/1.0"

// Fixed CLI identity aliases — single source of truth is internal/pkg/xai.
const (
	grokClientVersionHeader    = xai.CLIStableVersion
	grokClientIdentifierHeader = xai.CLIClientIdentifier
	grokClientModeHeader       = xai.CLIClientMode
)

// defaultGrokUpstreamUserAgent is the pinned Grok CLI / workspace UA.
// Grok upstream must not forward Claude Code / Codex / browser client UAs.
func defaultGrokUpstreamUserAgent() string {
	return xai.CLIUserAgent(xai.ResolveCLIVersion())
}

func applyDefaultGrokUpstreamHeaders(req *http.Request) {
	if req == nil {
		return
	}
	// Always stamp CLI identity. Do not preserve inbound client UA (Claude Code,
	// Codex, curl, etc.) — xAI chat/CLI surfaces fingerprint the client string.
	xai.ApplyCLIIdentityHeaders(req.Header, xai.ResolveCLIVersion())
}

func applyGrokCLIAccountHeaders(headers http.Header, account *Account) {
	if headers == nil || account == nil {
		return
	}
	// x-email/x-userid appear on Grok Build settings/models requests; they are
	// intentionally not added to inference requests by the local CLI.
	if email := account.GetGrokEmail(); email != "" {
		headers.Set("x-email", email)
	}
	if userID := account.GetGrokUserID(); userID != "" {
		headers.Set("x-userid", userID)
		headers.Set("x-grok-user-id", userID)
	}
	if account.ID > 0 {
		headers.Set("x-grok-agent-id", grokStableAgentID(account.ID))
	}
}

func applyGrokCLIInferenceAccountHeaders(headers http.Header, account *Account) {
	if headers == nil || account == nil {
		return
	}
	if userID := account.GetGrokUserID(); userID != "" {
		headers.Set("x-grok-user-id", userID)
	}
	if account.ID > 0 {
		headers.Set("x-grok-agent-id", grokStableAgentID(account.ID))
	}
}

func applyGrokCLITurnHeaders(headers http.Header, model, cacheIdentity string) {
	if headers == nil {
		return
	}
	if model = strings.TrimSpace(model); model != "" {
		headers.Set("x-grok-model-override", model)
	}
	if cacheIdentity = strings.TrimSpace(cacheIdentity); cacheIdentity != "" {
		headers.Set(grokConversationIDHeader, cacheIdentity)
		headers.Set("x-grok-session-id", cacheIdentity)
	}
	headers.Set("x-grok-req-id", uuid.NewString())
	headers.Set("x-grok-doom-loop-check", "true")
	headers.Set("x-grok-turn-idx", grokTurnIndex(cacheIdentity))
}

func applyGrokOAuthInferenceHeaders(headers http.Header, account *Account, model, cacheIdentity string) {
	applyGrokCLIHeaders(headers)
	applyGrokCLIInferenceAccountHeaders(headers, account)
	applyGrokCLITurnHeaders(headers, model, cacheIdentity)
}

func grokTurnIndex(session string) string {
	if strings.TrimSpace(session) == "" {
		session = "_"
	}
	counter, _ := grokTurnCounters.LoadOrStore(session, new(uint64))
	return strconv.FormatUint(atomic.AddUint64(counter.(*uint64), 1), 10)
}

func grokStableAgentID(accountID int64) string {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(fmt.Sprintf("sub2api/grok-agent/%d", accountID))).String()
}

func applyGrokTLSProfileHeaders(req *http.Request, profile *tlsfingerprint.Profile) {
	// HEAD Profile is TLS-only (no HTTP UserAgent/Originator fields). Always stamp CLI identity.
	applyDefaultGrokUpstreamHeaders(req)
	_ = profile
}

// openAITLSFingerprintRuntime is the resolved TLS fingerprint routing result
// used by OpenAI/Grok outbound header application. Defined here so Grok header
// helpers compile even when the full OpenAI TLS router is not present on HEAD.
type openAITLSFingerprintRuntime struct {
	Profile            *tlsfingerprint.Profile
	UpstreamUserAgent  string
	UpstreamOriginator string
	Matched            bool
}

func applyGrokRuntimeHeaders(req *http.Request, runtime openAITLSFingerprintRuntime) {
	applyDefaultGrokUpstreamHeaders(req)
	if req == nil {
		return
	}
	// Apply Originator only; force CLI UA after so router overrides cannot
	// leak Codex/Claude Code identity onto Grok upstream.
	if originator := strings.TrimSpace(runtime.UpstreamOriginator); originator != "" {
		req.Header.Set("Originator", originator)
	}
	req.Header.Set("User-Agent", defaultGrokUpstreamUserAgent())
}

// resolveGrokUpstreamUserAgent always returns the pinned Grok CLI User-Agent.
// Inbound client UAs (Claude Code, Codex, browsers, libraries) are never forwarded.
func resolveGrokUpstreamUserAgent(_ *gin.Context) string {
	return defaultGrokUpstreamUserAgent()
}

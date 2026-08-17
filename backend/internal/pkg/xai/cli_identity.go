package xai

import (
	"net/http"
	"os"
	"strings"

	"golang.org/x/mod/semver"
)

// Fixed Grok Build / CLI-chat-proxy client identity.
// These values are intentionally pinned in-binary (not scraped from live CLI).
// Operators may bump the version via XAI_GROK_CLI_VERSION without a release.
const (
	// CLIProxyHost is the hostname that requires the official CLI identity headers.
	CLIProxyHost = "cli-chat-proxy.grok.com"

	// CLIStableVersion is the known-good minimum client version accepted by cli-chat-proxy.
	CLIStableVersion = "0.2.93"

	// CLIVersionEnv is the optional operator override for CLIStableVersion.
	CLIVersionEnv = "XAI_GROK_CLI_VERSION"

	// CLITokenAuth is required by cli-chat-proxy for Grok Build OAuth tokens.
	CLITokenAuth = "xai-grok-cli"

	// CLIClientIdentifier is the x-grok-client-identifier value used by the
	// interactive Grok Build TUI (grok-pager). Headless `grok -p` uses grok-shell.
	CLIClientIdentifier = "grok-pager"

	// CLIClientMode is the x-grok-client-mode value for an interactive TUI session.
	CLIClientMode = "interactive"

	// CLIAuthenticateResponse is required by cli-chat-proxy auth middleware.
	CLIAuthenticateResponse = "authenticate-response"

	// CLIAcceptEncoding matches the official reqwest client.
	CLIAcceptEncoding = "gzip, br, deflate"

	// CLIPlatformUA is the os/arch token official Build emits on macOS arm64.
	CLIPlatformUA = "macos; aarch64"
)

// ResolveCLIVersion returns a supported CLI client version.
// Empty or invalid overrides fall back to CLIClientVersion (the pinned
// preferred client pin in billing.go). CLIStableVersion is only the minimum
// accepted by IsSupportedCLIVersion, not the default identity we advertise.
func ResolveCLIVersion() string {
	version := strings.TrimSpace(os.Getenv(CLIVersionEnv))
	if !IsSupportedCLIVersion(version) {
		return CLIClientVersion
	}
	return version
}

// IsSupportedCLIVersion reports whether version is a valid semver string at or
// above CLIStableVersion (prereleases below a higher release are rejected when
// they compare less than the stable pin).
func IsSupportedCLIVersion(version string) bool {
	canonical := "v" + version
	minimum := "v" + CLIStableVersion
	return semver.IsValid(canonical) &&
		semver.Canonical(canonical) == canonical &&
		semver.Compare(canonical, minimum) >= 0
}

// CLIUserAgent builds the official Grok Build TUI User-Agent.
// Captured from local grok 1.0.3 on macOS arm64:
//
//	grok-pager/1.0.3 grok-shell/1.0.3 (macos; aarch64)
func CLIUserAgent(version string) string {
	if strings.TrimSpace(version) == "" {
		version = CLIClientVersion
	}
	return "grok-pager/" + version + " grok-shell/" + version + " (" + CLIPlatformUA + ")"
}

// ApplyCLIIdentityHeaders stamps the static Grok Build identity headers.
// Version must already be resolved by the caller.
func ApplyCLIIdentityHeaders(headers http.Header, version string) {
	if headers == nil {
		return
	}
	if strings.TrimSpace(version) == "" {
		version = CLIClientVersion
	}
	headers.Set("X-XAI-Token-Auth", CLITokenAuth)
	headers.Set("x-grok-client-version", version)
	headers.Set("x-grok-client-identifier", CLIClientIdentifier)
	headers.Set("x-grok-client-mode", CLIClientMode)
	headers.Set("x-authenticateresponse", CLIAuthenticateResponse)
	headers.Set("User-Agent", CLIUserAgent(version))
	if strings.TrimSpace(headers.Get("Accept-Encoding")) == "" {
		headers.Set("Accept-Encoding", CLIAcceptEncoding)
	}
}

// ApplyCLIProxyHeaders stamps the fixed Grok CLI identity when the request
// targets cli-chat-proxy. Direct api.x.ai traffic is left unchanged.
func ApplyCLIProxyHeaders(req *http.Request) {
	if req == nil || req.URL == nil || !strings.EqualFold(strings.TrimSpace(req.URL.Hostname()), CLIProxyHost) {
		return
	}
	if req.Header == nil {
		req.Header = make(http.Header)
	}
	ApplyCLIIdentityHeaders(req.Header, ResolveCLIVersion())
}

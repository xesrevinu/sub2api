package service

import (
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// grokForcePriorityServiceTierEnabled reports whether this Grok account should
// inject service_tier=priority. Explicit extra bools win. Missing values
// default on for OAuth and off for API keys. Malformed values fail closed.
func grokForcePriorityServiceTierEnabled(account *Account) bool {
	if account == nil || !account.IsGrok() {
		return false
	}
	if account.Extra != nil {
		if raw, exists := account.Extra[GrokForcePriorityServiceTierExtraKey]; exists {
			enabled, ok := raw.(bool)
			if !ok {
				return false
			}
			return enabled
		}
	}
	return account.IsGrokOAuth()
}

// applyGrokForcePriorityServiceTier writes service_tier=priority onto a Grok
// text inference body when the account extra is enabled. Image/video/audio
// callers must not use this helper.
func applyGrokForcePriorityServiceTier(body []byte, account *Account) ([]byte, error) {
	if len(body) == 0 || !grokForcePriorityServiceTierEnabled(account) {
		return body, nil
	}
	parsed := gjson.ParseBytes(body)
	if !parsed.IsObject() {
		return body, nil
	}
	if parsed.Get("service_tier").String() == OpenAIFastTierPriority {
		return body, nil
	}
	return sjson.SetBytes(body, "service_tier", OpenAIFastTierPriority)
}

func normalizeGrokForcePriorityServiceTierExtra(platform string, extra map[string]any) (map[string]any, error) {
	if extra == nil {
		return extra, nil
	}
	raw, exists := extra[GrokForcePriorityServiceTierExtraKey]
	if !exists {
		return extra, nil
	}
	normalized := shallowCopyMap(extra)
	if platform != PlatformGrok || raw == nil {
		delete(normalized, GrokForcePriorityServiceTierExtraKey)
		return normalized, nil
	}
	if _, ok := raw.(bool); !ok {
		return nil, infraerrors.BadRequest("GROK_FORCE_PRIORITY_INVALID", "grok_force_priority_service_tier must be a boolean or null")
	}
	return normalized, nil
}

func normalizeGrokForcePriorityServiceTierUpdateExtra(account *Account, input *UpdateAccountInput, normalized map[string]any) (map[string]any, error) {
	if account == nil || account.Platform != PlatformGrok {
		if normalized == nil {
			return normalized, nil
		}
		out := shallowCopyMap(normalized)
		delete(out, GrokForcePriorityServiceTierExtraKey)
		return out, nil
	}
	if input == nil {
		return nil, infraerrors.BadRequest("INVALID_ACCOUNT_INPUT", "account update input is required")
	}
	if err := ValidateGrokForcePriorityServiceTierExtra(account.Platform, input.Extra); err != nil {
		return nil, err
	}
	if normalized == nil {
		normalized = make(map[string]any)
	} else {
		normalized = shallowCopyMap(normalized)
	}
	raw, provided := input.Extra[GrokForcePriorityServiceTierExtraKey]
	if provided {
		if raw == nil {
			delete(normalized, GrokForcePriorityServiceTierExtraKey)
		}
		return normalized, nil
	}
	if current, ok := account.Extra[GrokForcePriorityServiceTierExtraKey].(bool); ok {
		normalized[GrokForcePriorityServiceTierExtraKey] = current
	}
	return normalized, nil
}

func ValidateGrokForcePriorityServiceTierExtra(platform string, extra map[string]any) error {
	if platform != PlatformGrok || extra == nil {
		return nil
	}
	raw, exists := extra[GrokForcePriorityServiceTierExtraKey]
	if !exists || raw == nil {
		return nil
	}
	if _, ok := raw.(bool); !ok {
		return infraerrors.BadRequest("GROK_FORCE_PRIORITY_INVALID", "grok_force_priority_service_tier must be a boolean or null")
	}
	return nil
}

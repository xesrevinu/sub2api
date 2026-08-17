package handler

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type openAIRelayRouteKind int

const (
	openAIRelayRouteResponses openAIRelayRouteKind = iota
	openAIRelayRouteChatCompletions
	openAIRelayRouteRawPassthrough
)

// RelayOpenAI handles /api/relay/openai and rewrites supported OpenAI relay
// paths onto the existing OpenAI handlers so the request still flows through
// the standard scheduling and usage pipeline.
func (h *OpenAIGatewayHandler) RelayOpenAI(c *gin.Context) {
	targetPath, routeKind, err := normalizeOpenAIRelayPath(c.Param("subpath"))
	if err != nil {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}

	if c.Request != nil {
		if c.Request.URL != nil {
			c.Request.URL.Path = targetPath
			c.Request.URL.RawPath = targetPath
			if c.Request.URL.RawQuery != "" {
				c.Request.RequestURI = targetPath + "?" + c.Request.URL.RawQuery
			} else {
				c.Request.RequestURI = targetPath
			}
		}
	}

	c.Set(ctxKeyInboundEndpoint, NormalizeInboundEndpoint(targetPath))
	// Relay requests still go through the normal scheduler/billing pipeline, but
	// they must keep the original OpenAI-compatible upstream path. Some upstreams
	// behind our "openai-compact" group expose /v1/chat/completions only and
	// return 404 if we normalize everything to /v1/responses.
	service.SetOpenAIForcePassthrough(c)

	switch routeKind {
	case openAIRelayRouteResponses:
		switch c.Request.Method {
		case http.MethodGet:
			h.ResponsesWebSocket(c)
		case http.MethodPost:
			h.Responses(c)
		default:
			h.errorResponse(c, http.StatusMethodNotAllowed, "invalid_request_error", "Only GET and POST are supported for /api/relay/openai responses relay")
		}
	case openAIRelayRouteChatCompletions:
		if c.Request.Method != http.MethodPost {
			h.errorResponse(c, http.StatusMethodNotAllowed, "invalid_request_error", "Only POST is supported for /api/relay/openai chat completions relay")
			return
		}
		h.ChatCompletions(c)
	case openAIRelayRouteRawPassthrough:
		h.RelayOpenAIRawPassthrough(c)
	default:
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Unsupported relay route")
	}
}

func normalizeOpenAIRelayPath(rawSubpath string) (string, openAIRelayRouteKind, error) {
	subpath := strings.TrimSpace(rawSubpath)
	if subpath == "" || subpath == "/" {
		return EndpointResponses, openAIRelayRouteResponses, nil
	}
	if !strings.HasPrefix(subpath, "/") {
		subpath = "/" + subpath
	}

	switch {
	case strings.HasPrefix(subpath, "/openai/"):
		return strings.TrimPrefix(subpath, "/openai"), openAIRelayRouteRawPassthrough, nil
	case subpath == "/responses", subpath == "/v1/responses":
		return subpath, openAIRelayRouteResponses, nil
	case strings.HasPrefix(subpath, "/responses/"), strings.HasPrefix(subpath, "/v1/responses/"):
		return subpath, openAIRelayRouteResponses, nil
	case subpath == "/chat/completions":
		return EndpointChatCompletions, openAIRelayRouteChatCompletions, nil
	case subpath == EndpointChatCompletions:
		return EndpointChatCompletions, openAIRelayRouteChatCompletions, nil
	default:
		return "", 0, fmt.Errorf("unsupported relay path: %s (expected /responses, /v1/responses, /chat/completions, /v1/chat/completions, or /openai/*)", subpath)
	}
}

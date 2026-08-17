package handler

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeOpenAIRelayPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		want     string
		wantKind openAIRelayRouteKind
		wantErr  string
	}{
		{name: "empty defaults to v1 responses", input: "", want: EndpointResponses, wantKind: openAIRelayRouteResponses},
		{name: "slash defaults to v1 responses", input: "/", want: EndpointResponses, wantKind: openAIRelayRouteResponses},
		{name: "responses alias", input: "/responses", want: "/responses", wantKind: openAIRelayRouteResponses},
		{name: "responses compact alias", input: "/responses/compact", want: "/responses/compact", wantKind: openAIRelayRouteResponses},
		{name: "v1 responses", input: "/v1/responses", want: "/v1/responses", wantKind: openAIRelayRouteResponses},
		{name: "v1 responses compact", input: "/v1/responses/compact", want: "/v1/responses/compact", wantKind: openAIRelayRouteResponses},
		{name: "chat completions alias", input: "/chat/completions", want: EndpointChatCompletions, wantKind: openAIRelayRouteChatCompletions},
		{name: "v1 chat completions", input: "/v1/chat/completions", want: EndpointChatCompletions, wantKind: openAIRelayRouteChatCompletions},
		{name: "raw openai passthrough", input: "/openai/v1/models", want: "/v1/models", wantKind: openAIRelayRouteRawPassthrough},
		{name: "unsupported path", input: "/v1/assistants", wantErr: "unsupported relay path"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, gotKind, err := normalizeOpenAIRelayPath(tc.input)
			if tc.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
			require.Equal(t, tc.wantKind, gotKind)
		})
	}
}

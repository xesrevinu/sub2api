package service

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenAIForcePassthroughContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest("POST", "/api/relay/openai/v1/chat/completions", nil)

	require.False(t, IsOpenAIForcePassthrough(c))
	SetOpenAIForcePassthrough(c)
	require.True(t, IsOpenAIForcePassthrough(c))
	require.True(t, IsOpenAIForcePassthroughContext(c.Request.Context()))
}

func TestWithOpenAIForcePassthrough(t *testing.T) {
	ctx := WithOpenAIForcePassthrough(context.Background())
	require.True(t, IsOpenAIForcePassthroughContext(ctx))
	require.False(t, IsOpenAIForcePassthroughContext(context.Background()))
}

func TestWithOpenAIRequestGroup(t *testing.T) {
	ctx := WithOpenAIRequestGroup(context.Background(), &Group{
		ID:       2,
		Name:     "openai",
		Platform: PlatformOpenAI,
	})
	group, ok := OpenAIRequestGroupFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, int64(2), group.ID)
	require.Equal(t, "openai", group.Name)
	require.Equal(t, PlatformOpenAI, group.Platform)
}

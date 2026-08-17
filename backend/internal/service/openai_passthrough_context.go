package service

import (
	"context"

	"github.com/gin-gonic/gin"
)

const openAIForcePassthroughContextKey = "openai_force_passthrough"

type openAIForcePassthroughRequestContextKey struct{}
type openAIRequestGroupContextKey struct{}

type openAIRequestGroupContextValue struct {
	ID       int64
	Name     string
	Platform string
}

func WithOpenAIForcePassthrough(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, openAIForcePassthroughRequestContextKey{}, true)
}

func SetOpenAIForcePassthrough(c *gin.Context) {
	if c == nil {
		return
	}
	c.Set(openAIForcePassthroughContextKey, true)
	if c.Request != nil {
		c.Request = c.Request.WithContext(WithOpenAIForcePassthrough(c.Request.Context()))
	}
}

func IsOpenAIForcePassthrough(c *gin.Context) bool {
	if c == nil {
		return false
	}
	value, ok := c.Get(openAIForcePassthroughContextKey)
	if ok {
		enabled, _ := value.(bool)
		if enabled {
			return true
		}
	}
	if c.Request == nil {
		return false
	}
	return IsOpenAIForcePassthroughContext(c.Request.Context())
}

func IsOpenAIForcePassthroughContext(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	enabled, _ := ctx.Value(openAIForcePassthroughRequestContextKey{}).(bool)
	return enabled
}

func WithOpenAIRequestGroup(ctx context.Context, group *Group) context.Context {
	if group == nil {
		return ctx
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, openAIRequestGroupContextKey{}, openAIRequestGroupContextValue{
		ID:       group.ID,
		Name:     group.Name,
		Platform: group.Platform,
	})
}

func BindOpenAIRequestGroup(c *gin.Context, group *Group) {
	if c == nil || c.Request == nil {
		return
	}
	c.Request = c.Request.WithContext(WithOpenAIRequestGroup(c.Request.Context(), group))
}

func OpenAIRequestGroupFromContext(ctx context.Context) (*Group, bool) {
	if ctx == nil {
		return nil, false
	}
	value, _ := ctx.Value(openAIRequestGroupContextKey{}).(openAIRequestGroupContextValue)
	if value.ID <= 0 && value.Name == "" && value.Platform == "" {
		return nil, false
	}
	return &Group{
		ID:       value.ID,
		Name:     value.Name,
		Platform: value.Platform,
		Hydrated: true,
	}, true
}

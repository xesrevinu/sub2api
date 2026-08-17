package handler

import (
	"context"
	"net/http"
	"strconv"
	"time"

	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

// RelayOpenAIRawPassthrough handles arbitrary OpenAI-compatible relay paths
// that arrive via /api/relay/openai/openai/* and preserves the original
// method/path/query for API-key upstreams.
func (h *OpenAIGatewayHandler) RelayOpenAIRawPassthrough(c *gin.Context) {
	streamStarted := false
	defer h.recoverResponsesPanic(c, &streamStarted)

	requestStart := time.Now()

	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}

	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusInternalServerError, "api_error", "User context not found")
		return
	}

	reqLog := requestLogger(
		c,
		"handler.openai_gateway.relay_passthrough",
		zap.Int64("user_id", subject.UserID),
		zap.Int64("api_key_id", apiKey.ID),
		zap.Any("group_id", apiKey.GroupID),
	)
	if !h.ensureResponsesDependencies(c, reqLog) {
		return
	}
	service.BindOpenAIRequestGroup(c, apiKey.Group)

	body, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
	if err != nil {
		if maxErr, ok := extractMaxBytesError(err); ok {
			h.errorResponse(c, http.StatusRequestEntityTooLarge, "invalid_request_error", buildBodyTooLargeMessage(maxErr.Limit))
			return
		}
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
		return
	}

	reqModel := ""
	reqStream := false
	if len(body) > 0 && gjson.ValidBytes(body) {
		if model := gjson.GetBytes(body, "model"); model.Exists() && model.Type == gjson.String {
			reqModel = model.String()
		}
		reqStream = gjson.GetBytes(body, "stream").Bool()
	}
	reqLog = reqLog.With(
		zap.String("path", c.Request.URL.Path),
		zap.String("method", c.Request.Method),
		zap.String("model", reqModel),
		zap.Bool("stream", reqStream),
	)

	setOpsRequestContext(c, reqModel, reqStream)
	if h.errorPassthroughService != nil {
		service.BindErrorPassthroughService(c, h.errorPassthroughService)
	}

	subscription, _ := middleware2.GetSubscriptionFromContext(c)
	service.SetOpsLatencyMs(c, service.OpsAuthLatencyMsKey, time.Since(requestStart).Milliseconds())
	routingStart := time.Now()

	userReleaseFunc, acquired := h.acquireResponsesUserSlot(c, subject.UserID, subject.Concurrency, reqStream, &streamStarted, reqLog)
	if !acquired {
		return
	}
	if userReleaseFunc != nil {
		defer userReleaseFunc()
	}

	if err := h.billingCacheService.CheckBillingEligibility(c.Request.Context(), apiKey.User, apiKey, apiKey.Group, subscription, service.QuotaPlatform(c.Request.Context(), apiKey)); err != nil {
		reqLog.Info("openai_relay_passthrough.billing_eligibility_check_failed", zap.Error(err))
		status, code, message, retryAfter := billingErrorDetails(err)
		if retryAfter > 0 {
			c.Header("Retry-After", strconv.Itoa(retryAfter))
		}
		h.handleStreamingAwareError(c, status, code, message, streamStarted)
		return
	}

	sessionHash := h.gatewayService.GenerateSessionHash(c, body)
	maxAccountSwitches := h.maxAccountSwitches
	switchCount := 0
	failedAccountIDs := make(map[int64]struct{})

	for {
		selection, scheduleDecision, err := h.gatewayService.SelectAccountWithScheduler(
			c.Request.Context(),
			apiKey.GroupID,
			"",
			sessionHash,
			reqModel,
			failedAccountIDs,
			service.OpenAIUpstreamTransportAny,
			false,
		)
		if err != nil {
			reqLog.Warn("openai_relay_passthrough.account_select_failed",
				zap.Error(err),
				zap.Int("excluded_account_count", len(failedAccountIDs)),
			)
			h.handleStreamingAwareError(c, http.StatusServiceUnavailable, "api_error", "Service temporarily unavailable", streamStarted)
			return
		}
		if selection == nil || selection.Account == nil {
			h.handleStreamingAwareError(c, http.StatusServiceUnavailable, "api_error", "No available accounts", streamStarted)
			return
		}

		account := selection.Account
		_ = scheduleDecision
		setOpsSelectedAccount(c, account.ID, account.Platform)

		// Raw /openai/* relay must target API-key style upstreams. OAuth OpenAI
		// accounts only expose the ChatGPT internal Responses surface and cannot
		// safely proxy arbitrary OpenAI-compatible endpoints.
		if account.Type == service.AccountTypeOAuth {
			failedAccountIDs[account.ID] = struct{}{}
			switchCount++
			if switchCount > maxAccountSwitches {
				h.handleStreamingAwareError(c, http.StatusServiceUnavailable, "api_error", "No API-key passthrough accounts available", streamStarted)
				return
			}
			reqLog.Warn("openai_relay_passthrough.skip_oauth_account_for_raw_path",
				zap.Int64("account_id", account.ID),
				zap.String("account_name", account.Name),
				zap.Int("switch_count", switchCount),
			)
			continue
		}

		accountReleaseFunc, slotResult := h.acquireResponsesAccountSlot(c, apiKey.GroupID, sessionHash, selection, reqStream, &streamStarted, reqLog)
		if slotResult != openAISlotAcquireOK {
			return
		}

		service.SetOpsLatencyMs(c, service.OpsRoutingLatencyMsKey, time.Since(routingStart).Milliseconds())
		forwardStart := time.Now()
		result, err := h.gatewayService.ForwardPassthrough(c.Request.Context(), c, account, body, reqModel, reqStream, forwardStart)
		forwardDurationMs := time.Since(forwardStart).Milliseconds()
		if accountReleaseFunc != nil {
			accountReleaseFunc()
		}
		upstreamLatencyMs, _ := getContextInt64(c, service.OpsUpstreamLatencyMsKey)
		responseLatencyMs := forwardDurationMs
		if upstreamLatencyMs > 0 && forwardDurationMs > upstreamLatencyMs {
			responseLatencyMs = forwardDurationMs - upstreamLatencyMs
		}
		service.SetOpsLatencyMs(c, service.OpsResponseLatencyMsKey, responseLatencyMs)
		if err == nil && result != nil && result.FirstTokenMs != nil {
			service.SetOpsLatencyMs(c, service.OpsTimeToFirstTokenMsKey, int64(*result.FirstTokenMs))
		}
		if err != nil {
			h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, account.GetMappedModel(reqModel), false, nil)
			wroteFallback := h.ensureForwardErrorResponse(c, streamStarted)
			reqLog.Warn("openai_relay_passthrough.forward_failed",
				zap.Int64("account_id", account.ID),
				zap.Bool("fallback_error_response_written", wroteFallback),
				zap.Error(err),
			)
			return
		}
		if result != nil {
			h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, account.GetMappedModel(reqModel), true, result.FirstTokenMs)
		} else {
			h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, account.GetMappedModel(reqModel), true, nil)
		}

		userAgent := c.GetHeader("User-Agent")
		clientIP := ip.GetClientIP(c)

		h.submitUsageRecordTask(c.Request.Context(), func(ctx context.Context) {
			if err := h.gatewayService.RecordUsage(ctx, &service.OpenAIRecordUsageInput{
				Result:           result,
				APIKey:           apiKey,
				User:             apiKey.User,
				Account:          account,
				Subscription:     subscription,
				InboundEndpoint:  GetInboundEndpoint(c),
				UpstreamEndpoint: GetUpstreamEndpoint(c, account.Platform),
				UserAgent:        userAgent,
				IPAddress:        clientIP,
				APIKeyService:    h.apiKeyService,
			}); err != nil {
				logger.L().With(
					zap.String("component", "handler.openai_gateway.relay_passthrough"),
					zap.Int64("user_id", subject.UserID),
					zap.Int64("api_key_id", apiKey.ID),
					zap.Any("group_id", apiKey.GroupID),
					zap.String("model", reqModel),
					zap.Int64("account_id", account.ID),
				).Error("openai_relay_passthrough.record_usage_failed", zap.Error(err))
			}
		})
		reqLog.Debug("openai_relay_passthrough.request_completed", zap.Int64("account_id", account.ID))
		return
	}
}

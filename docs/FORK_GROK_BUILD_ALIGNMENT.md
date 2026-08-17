# Fork: Grok Build 身份/TLS 对齐记录

> 本文记录本地 fork 相对上游 `Wei-Shaw/sub2api` 在 Grok 上游请求上做的定制，以及为什么这样做。后续同步上游、解决冲突或回退时，先读这一份。

## 1. 背景

本地使用 Grok 订阅 OAuth 流量，请求打到 `cli-chat-proxy.grok.com`。该代理会按 Grok Build 客户端身份做版本门、鉴权和归因；上游默认声明的是 `xai-grok-workspace/0.2.114`，与本机实际 `grok` CLI 不一致。

fork 目标：尽量贴近本机 macOS Grok Build 1.0.3 的真实流量，降低被代理识别为第三方转发的概率。

## 2. 固定身份

- 版本：`1.0.3`
- 可用 `XAI_GROK_CLI_VERSION` 覆盖，但不得低于 `1.0.3`
- User-Agent：`grok-pager/1.0.3 grok-shell/1.0.3 (macos; aarch64)`
- `x-grok-client-identifier: grok-pager`
- `x-grok-client-mode: interactive`
- `x-authenticateresponse: authenticate-response`
- `X-XAI-Token-Auth: xai-grok-cli`
- `Accept-Encoding: gzip, br, deflate`

说明：本地 `grok -p` 是 `grok-shell + headless`，但我们选择模拟本机默认 TUI，即 `grok-pager + interactive`。这是刻意保留的差异。

API Key 流量走 `api.x.ai`，故意不加 CLI 身份头；`api.x.ai` 回退时也会剥掉 CLI 身份头。

## 3. TLS / HTTP2 指纹

- 从本机 `~/.grok/bin/grok` 1.0.3 抓取 rustls 0.23 ClientHello，固化到 `backend/internal/pkg/tlsfingerprint/grok_build.go`
- 密码套件、曲线、签名算法、扩展顺序、ALPN `h2, http/1.1` 均按本机抓包对齐
- Grok 官方域必须直接用 `http2.Transport`：Go 的 `net/http` 只识别 `*tls.Conn` 的 ALPN，`utls.UConn` 会被误当 HTTP/1
- `Do()` 对 `cli-chat-proxy.grok.com`、`api.x.ai`、`*.api.x.ai` 自动启用 Grok profile

## 4. 推理请求头

- 带 `x-grok-model-override`、`x-grok-req-id`、`x-grok-conv-id`、`x-grok-session-id`、`x-grok-agent-id`、`x-grok-turn-idx`、`x-grok-doom-loop-check`
- 不带 `x-email` / `x-userid`，但带账号里的 `x-grok-user-id`
- `Accept` 按是否 stream 区分：stream `text/event-stream`，非 stream `application/json`

## 5. 主要改动文件

同步时最容易冲突的位置：

- `backend/internal/pkg/xai/cli_identity.go`
- `backend/internal/pkg/xai/billing.go`
- `backend/internal/pkg/xai/billing_test.go`
- `backend/internal/pkg/tlsfingerprint/grok_build.go`
- `backend/internal/repository/http_upstream.go`
- `backend/internal/repository/http_upstream_test.go`
- `backend/internal/service/grok_upstream_headers.go`
- `backend/internal/service/openai_gateway_grok.go`
- `backend/internal/service/gateway_service.go`
- `backend/internal/service/grok_media.go`
- `backend/internal/service/grok_audio.go`
- `backend/internal/service/upstream_models.go`
- `backend/internal/service/grok_quota_service.go`

## 6. 同步后回归检查

```bash
cd backend && go test ./internal/pkg/xai ./internal/pkg/tlsfingerprint ./internal/repository ./internal/service -run 'Grok|CLI|UpstreamHeaders|BuildGrok'
```

线上验证：

- Grok OAuth 账号请求必须走 `cli-chat-proxy.grok.com`
- 日志应出现 `profile: "Grok Build (rustls 0.23)"` 和 `alpn: "h2"`
- 真实 `POST /v1/responses` 应返回 `grok-4.5-build` 等订阅模型

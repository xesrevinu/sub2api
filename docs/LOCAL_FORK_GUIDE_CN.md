# 本地 Fork 维护说明

> 本文是本地 fork 的唯一说明：相对上游 `Wei-Shaw/sub2api` 的定制、同步升级、验证和重建本地镜像都写在这里。

## 1. 背景

当前 fork 主要是为了补齐一条上游暂时没有的能力：

- 增加 `/api/relay/openai` 透传入口
- 让这条 relay 请求继续走现有的鉴权、账号调度、计费、`usage_logs` 记录链路
- 支持旧的 OpenAI 兼容路径 `/v1/chat/completions`
- 对 OpenAI-compatible 上游保留原始请求路径，不强行归一到 `/v1/responses`
- 在选择转发账号时，不只按 `platform=openai` 选，还会优先选择 `openai-compact` 分组

这样做的目标是：

- 兼容 OpenAI-like API 提供商
- 继续复用当前项目已有的 token 统计和计费基础设施
- 减少未来与上游同步时的理解成本

## 2. 当前 fork 的核心行为

### 2.1 Relay 入口

新增并支持：

- `GET /api/relay/openai`
- `POST /api/relay/openai`
- `GET /api/relay/openai/*subpath`
- `POST /api/relay/openai/*subpath`

当前已支持的 relay 子路径：

- `/responses`
- `/v1/responses`
- `/responses/...`
- `/v1/responses/...`
- `/chat/completions`
- `/v1/chat/completions`

### 2.2 为什么 `chat/completions` 不能统一改写到 `/v1/responses`

这是本 fork 最重要的行为差异之一。

对于 OpenAI 官方风格的 API，`/v1/chat/completions` 和 `/v1/responses` 有时可以做兼容转换；但对于一些 OpenAI-compatible 上游，这个假设不成立。

我们在本地验证中已经确认：

- 某些通过 `tuzi openai` 账号访问的模型可以正常处理 `/v1/chat/completions`
- 同样的模型如果被强制改写到 `/v1/responses`，会返回 `404`

所以本 fork 的策略是：

- relay 请求仍然复用当前项目的调度、日志、计费逻辑
- 但对 passthrough 请求，保留原始 OpenAI-compatible 上游路径

### 2.3 分组选择策略

当前 relay passthrough 请求的 OpenAI 账号选择策略是：

1. 仅在 OpenAI relay passthrough 场景触发特殊逻辑
2. 如果当前 API key 绑定组是 `openai`
3. 优先查找 `openai-compact`
4. 若 `openai-compact` 无可用账号，再回退到原始 `openai` 组

如果当前 API key 本身已经在 `openai-compact` 组，则仍只使用该组。

这样做的原因是：

- `openai-compact` 后续会承载更多 OpenAI-compatible 的其他家模型
- 这类模型通常更适合保持“原样透传”的行为

### 2.4 Usage 记录

为了让 relay 的非流式请求也能正确计入 token，用量解析同时支持两种返回格式：

- Responses API 风格：
  - `usage.input_tokens`
  - `usage.output_tokens`
- Chat Completions 风格：
  - `usage.prompt_tokens`
  - `usage.completion_tokens`

因此，relay 经过 `/v1/chat/completions` 返回时，`usage_logs` 也能继续记录 token。

## 3. 主要改动文件

后续和上游同步时，这些文件是最需要重点关注的冲突点：

- `backend/internal/handler/openai_relay_handler.go`
- `backend/internal/handler/openai_chat_completions.go`
- `backend/internal/handler/openai_gateway_handler.go`
- `backend/internal/handler/endpoint.go`
- `backend/internal/service/openai_gateway_service.go`
- `backend/internal/service/openai_passthrough_context.go`
- `backend/internal/service/cursor_pricing.go`
- `backend/internal/service/cursor_pricing_test.go`
- `backend/internal/service/billing_service.go`
- `backend/internal/service/openai_model_alias.go`
- `backend/internal/service/openai_gateway_service_test.go`
- `backend/internal/service/openai_passthrough_context_test.go`
- `backend/internal/handler/openai_relay_handler_test.go`
- `backend/internal/handler/endpoint_test.go`
- `backend/internal/server/routes/gateway.go`
- `backend/internal/server/routes/gateway_test.go`

本地开发环境相关文件：

- `flake.nix`
- `flake.lock`
- `.envrc`
- `deploy/docker-compose.build.local.yml`
- `deploy/docker-compose.local.yml`

## 4. 建议优先阅读的代码入口

如果后续要继续演进这条 fork，建议按这个顺序读：

1. `backend/internal/handler/openai_relay_handler.go`
2. `backend/internal/handler/openai_chat_completions.go`
3. `backend/internal/service/openai_passthrough_context.go`
4. `backend/internal/service/openai_gateway_service.go`
5. `backend/internal/handler/endpoint.go`

理解顺序大致是：

- relay 如何进入现有 handler
- passthrough 标记如何传到 service 层
- service 如何选择账号与构造上游 URL
- usage/logging 如何记录原始 endpoint

## 5. 本地开发环境

当前仓库已经补了 Nix + direnv 支持。

### 5.1 初始化

```bash
direnv allow
nix develop
```

或直接执行：

```bash
nix develop "path:/Users/kee/Workspace/github.com/xesrevinu/sub2api"
```

### 5.2 常用测试命令

针对 relay / passthrough 相关改动，建议优先跑定向测试：

```bash
nix develop "path:/Users/kee/Workspace/github.com/xesrevinu/sub2api" -c sh -lc '
  cd backend &&
  env GODEBUG=http2client=0 go test ./internal/service ./internal/handler -run "TestOpenAI"
'
```

如果只改了极少数文件，也可以只跑更小的测试集合。

### 5.3 Flake 和 Go 版本

`flake.nix` 当前 dev shell 用的是 `go_1_26`，不一定等于上游 `go.mod` 要求的精确版本。

例如：

- `backend/go.mod` 要求 `go 1.27.0`
- 本机 Nix `go_1_26` 低于该版本时，直接跑 `go test` 会触发自动下载 `go1.27.0`
- `GOTOOLCHAIN=local` 会因为版本不足失败

检查方式：

```bash
nix develop --command bash -lc 'go version && go env GOTOOLCHAIN GOVERSION'
sed -n '1,5p' backend/go.mod
```

如果本机 Go 验证被工具链下载卡住，优先用 Docker 构建验证；本地 Compose 的 `GOLANG_IMAGE` 必须与当前 `backend/go.mod` 的 Go 版本一致：

```yaml
GOLANG_IMAGE: public.ecr.aws/docker/library/golang:1.27.0-alpine
```

不要为了升级临时改 `go.mod`。

## 6. 本地重建与重启

当前本地运行使用的是：

- 运行编排：`deploy/docker-compose.local.yml`
- 构建编排：`deploy/docker-compose.build.local.yml`
- 运行镜像 tag：`weishaw/sub2api:latest`

推荐流程：

### 6.1 重建镜像

```bash
docker compose -f deploy/docker-compose.build.local.yml build sub2api
```

构建完成后，实际生成的 tag 可能是：

- `sub2api-local:local-<git-sha>`

可以先查看：

```bash
docker images --format '{{.Repository}}:{{.Tag}} {{.ID}}' | rg '^sub2api-local:'
```

### 6.2 重新打到本地运行 tag

```bash
docker tag sub2api-local:local-<git-sha> weishaw/sub2api:latest
```

### 6.3 重启运行容器

```bash
docker compose -f deploy/docker-compose.local.yml up -d --no-deps --force-recreate sub2api
docker compose -f deploy/docker-compose.local.yml ps sub2api
```

也可以一条命令构建并重启（数据库和 Redis 不会被重建）：

```bash
SUB2API_BUILD_VERSION=local-$(git rev-parse --short HEAD) \
SUB2API_BUILD_COMMIT=$(git rev-parse --short HEAD) \
docker compose -f deploy/docker-compose.local.yml up -d --build sub2api
```

期望 `sub2api` 状态是 `Up ... (healthy)`，端口仍是 `127.0.0.1:8088->8080/tcp`。

## 7. 本地验证方法

### 7.1 直接请求 relay 接口

```bash
curl --max-time 30 -sS -D - \
  -X POST 'http://127.0.0.1:8088/api/relay/openai/v1/chat/completions' \
  -H 'Authorization: Bearer <local-api-key>' \
  -H 'Content-Type: application/json' \
  --data '{
    "model":"deepseek-v3",
    "messages":[{"role":"user","content":"reply with ok"}],
    "stream":false
  }'
```

期望：

- HTTP `200`
- 返回 chat completion JSON

### 7.2 检查 usage_logs 是否落库

```bash
docker exec sub2api-postgres psql -U sub2api -d sub2api -c "
select id, created_at, account_id, requested_model, inbound_endpoint, upstream_endpoint
from usage_logs
order by id desc
limit 5;
"
```

期望看到：

- `requested_model = deepseek-v3`
- `inbound_endpoint = /v1/chat/completions`
- `upstream_endpoint = /v1/chat/completions`
- `account_id` 落到预期的 OpenAI-compatible 账号

### 7.3 检查是否优先落到 `openai-compact`

如果你已经确认：

- `openai` 组存在
- `openai-compact` 组存在
- `openai-compact` 下绑定了 OpenAI-compatible 账号

那么 relay `chat/completions` 的最新 `usage_logs.account_id` 应优先落到 `openai-compact` 对应账号，而不是普通 `openai` 组中的账号。

## 8. 与上游同步的建议流程

推荐保留一个明确的上游 remote：

```bash
git remote add upstream https://github.com/Wei-Shaw/sub2api.git
git fetch upstream
```

升级前先看工作区：

```bash
git remote -v
git status --short --branch
git log --oneline --decorate --left-right origin/main...main
```

有未提交改动时先确认是否属于本次升级，不要直接覆盖。

### 8.1 推荐更新方式

如果这是团队共享分支，建议优先使用 `merge`，避免频繁改写历史：

```bash
git checkout main
git fetch upstream
git merge upstream/main
```

如果你们内部明确使用线性历史，也可以：

```bash
git checkout main
git fetch upstream
git rebase upstream/main
```

当前约定若 `origin` 就是 `Wei-Shaw/sub2api`，则用：

```bash
git fetch origin
git rebase origin/main
```

冲突时：

```bash
git status --short
rg -n '<<<<<<<|=======|>>>>>>>|\|\|\|\|\|\|\|' .
gofmt -w <changed-go-files>
git diff --check
git add <resolved-files>
git rebase --continue
```

放弃本次同步：`git rebase --abort`。

### 8.2 同步时重点检查的冲突点

最常见冲突集中在：

- OpenAI handler 路由
- OpenAI passthrough / upstream URL 构造
- endpoint 归一化
- usage 解析
- gateway route 注册

建议每次同步后，至少手动确认下面几点没有被上游改回去：

1. relay 是否仍支持 `/api/relay/openai/v1/chat/completions`
2. relay passthrough 是否仍保留原始 upstream path
3. relay passthrough 是否仍优先 `openai-compact`
4. `usage_logs` 是否仍正确记录 `/v1/chat/completions`
5. Grok 身份 / TLS / 账号级 Priority 是否仍在（见第 11 节）
6. Grok 长上下文 / `cost_in_usd_ticks` 是否仍在（见第 16 节）

同步后建议确认这些符号还在：

```bash
rg -n 'ForwardPassthrough|IsOpenAIForcePassthrough|openAICompactRelayGroupName|openAIPassthroughRequestPath' \
  backend/internal/handler backend/internal/service

rg -n 'GrokBuildProfileName|ApplyCLIIdentityHeaders|applyGrokOAuthInferenceHeaders|grok-pager/1.0.3|applyGrokForcePriorityServiceTier|grok_force_priority_service_tier' \
  backend/internal/pkg backend/internal/repository backend/internal/service frontend/src/components/account
```

## 9. 建议的同步后回归检查

每次合上游后，至少执行一次：

```bash
nix develop "path:/Users/kee/Workspace/github.com/xesrevinu/sub2api" -c sh -lc '
  gofmt -w \
    backend/internal/handler/openai_relay_handler.go \
    backend/internal/handler/openai_chat_completions.go \
    backend/internal/handler/openai_gateway_handler.go \
    backend/internal/handler/endpoint.go \
    backend/internal/service/openai_passthrough_context.go \
    backend/internal/service/openai_gateway_service.go \
    backend/internal/service/openai_gateway_service_test.go \
    backend/internal/service/openai_passthrough_context_test.go &&
  cd backend &&
  env GODEBUG=http2client=0 go test ./internal/service ./internal/handler
'
```

然后做一次 live 验证：

- rebuild image
- restart `sub2api`
- curl 测 `/api/relay/openai/v1/chat/completions`
- 查 `usage_logs`

## 10. 后续继续推进时的建议

后续如果继续扩展这条 fork，建议保持下面的原则：

- 尽量把 fork 的行为差异限制在 OpenAI relay / passthrough 相关路径
- 每一处行为差异都补注释，说明“为什么不能按上游默认逻辑处理”
- 新增能力优先配测试，再做 live 验证
- 不要把 OpenAI-compatible 上游和 OpenAI 官方 OAuth 路径混成一套逻辑

简单说：

- OpenAI 官方 OAuth 仍然更偏 Responses-only
- OpenAI-compatible API Key 上游必须允许保留原始兼容路径

这条边界如果后面被模糊掉，很容易再次出现 `404 /v1/responses` 这一类回归问题

## 11. Grok Build 身份/TLS 对齐（2026-08-13）

### 11.1 为什么改

本地使用 Grok 订阅 OAuth 流量打到 `cli-chat-proxy.grok.com`，该代理会按 Grok Build 客户端身份做版本门、鉴权和归因。上游 Sub2API 默认声明的是 `xai-grok-workspace/0.2.114`，与本机实际 `grok` CLI 不一致。fork 目标是尽量贴近本机 macOS Grok Build 1.0.3 的真实流量，降低被代理识别为第三方转发的概率。

### 11.2 固定身份

- 版本：`1.0.3`，可用 `XAI_GROK_CLI_VERSION` 覆盖，但不得低于 1.0.3
- User-Agent：`grok-pager/1.0.3 grok-shell/1.0.3 (macos; aarch64)`
- `x-grok-client-identifier: grok-pager`
- `x-grok-client-mode: interactive`
- `x-authenticateresponse: authenticate-response`
- `X-XAI-Token-Auth: xai-grok-cli`
- `Accept-Encoding: gzip, br, deflate`

API Key 流量走 `api.x.ai`，故意不加 CLI 身份头；`api.x.ai` 回退时也会剥掉 CLI 身份头。

### 11.3 TLS / HTTP2 指纹

- 从本机 `~/.grok/bin/grok` 1.0.3 抓取 rustls 0.23 ClientHello，固化到 `backend/internal/pkg/tlsfingerprint/grok_build.go`
- 密码套件、曲线、签名算法、扩展顺序、ALPN `h2, http/1.1` 均按本机抓包对齐
- Grok 官方域必须直接用 `http2.Transport`：Go 的 `net/http` 只识别 `*tls.Conn` 的 ALPN，`utls.UConn` 会被误当 HTTP/1
- `Do()` 对 `cli-chat-proxy.grok.com`、`api.x.ai`、`*.api.x.ai` 自动启用 Grok profile

### 11.4 推理请求头差异

- 使用 `grok-pager + interactive`，而不是 `grok -p` 的 `grok-shell + headless`，这是刻意选择
- 推理请求带 `x-grok-model-override`、`x-grok-req-id`、`x-grok-conv-id`、`x-grok-session-id`、`x-grok-agent-id`、`x-grok-turn-idx`、`x-grok-doom-loop-check`
- 推理请求不带 `x-email` / `x-userid`，但带账号里的 `x-grok-user-id`
- `Accept` 按是否 stream 区分：stream `text/event-stream`，非 stream `application/json`

### 11.5 同步时的冲突点

以下文件是 Grok 定制最容易与上游冲突的位置：

- `backend/internal/pkg/xai/cli_identity.go`
- `backend/internal/pkg/xai/billing.go`
- `backend/internal/pkg/xai/billing_test.go`
- `backend/internal/service/grok_upstream_cost.go`
- `backend/internal/pkg/tlsfingerprint/grok_build.go`
- `backend/internal/repository/http_upstream.go`
- `backend/internal/repository/http_upstream_test.go`
- `backend/internal/service/grok_upstream_headers.go`
- `backend/internal/service/openai_gateway_grok.go`
- `backend/internal/service/grok_force_priority.go`
- `backend/internal/service/openai_gateway_chat_completions_raw.go`
- `backend/internal/service/account.go`
- `backend/internal/service/admin_account.go`
- `frontend/src/components/account/EditAccountModal.vue`
- `backend/internal/service/gateway_service.go`
- `backend/internal/service/grok_media.go`
- `backend/internal/service/grok_audio.go`
- `backend/internal/service/upstream_models.go`
- `backend/internal/service/grok_quota_service.go`

### 11.6 同步后回归检查

每次同步上游后，确认以下行为没有丢：

```bash
cd backend && go test ./internal/pkg/xai ./internal/pkg/tlsfingerprint ./internal/repository ./internal/service -run 'Grok|CLI|UpstreamHeaders|BuildGrok|ForcePriority'
```

并做一次线上验证：

- Grok OAuth 账号请求必须走 `cli-chat-proxy.grok.com`
- 日志应出现 `profile: "Grok Build (rustls 0.23)"` 和 `alpn: "h2"`
- 真实 `POST /v1/responses` 应返回 `grok-4.5-build` 等订阅模型
- 未改 extra 的 Grok OAuth 文本请求应带 `service_tier=priority`；API Key 默认不带；账号编辑页可单独开关

### 11.7 账号级强制 Priority

Grok CLI 没有 `service_tier` 开关，也不会发这个字段。fork 把开关放在账号 `extra.grok_force_priority_service_tier`，不是 yaml 或系统设置。

- 显式 `true` / `false` 以账号为准
- 未设置：OAuth 默认开（CLI 补不了这个字段），API Key 默认关
- 非法值按关处理
- 只注入文本 Responses / Chat Completions；图像 / 视频 / 音频不变
- API Key 只有上游回显 `priority` 才按 2x 计费
- 管理后台编辑任意 Grok 账号可见「强制 Priority」；保存时写回明确布尔值，避免下次被默认值盖掉

## 12. 升级常见问题

### 12.1 Docker 构建时 Go module 下载失败

可能看到 `goproxy.cn ... unexpected EOF`。通常是代理或网络临时问题，先直接重试同一条 Docker build 命令。

### 12.2 `SelectAccountWithScheduler` 参数数量不匹配

上游可能给调度函数新增参数。合并本地 passthrough 调用时，按上游签名补齐。普通 passthrough relay 调用使用：

```go
service.OpenAIUpstreamTransportAny,
false,
```

compact 专用路径再按实际需要传 `true`。

### 12.3 默认模型 helper 被上游删除

如果出现 `undefined: resolveOpenAIForwardDefaultMappedModel`，直接读 API key 绑定组的默认映射模型：

```go
defaultMappedModel := ""
if apiKey.Group != nil {
    defaultMappedModel = apiKey.Group.DefaultMappedModel
}
```

然后继续传给 `ForwardAsChatCompletions(..., defaultMappedModel)`。

### 12.4 Flake Go 版本低于 `go.mod`

如果看到 `go.mod requires go >= 1.27.0 (running go 1.26.1; GOTOOLCHAIN=local)`，不要改 `go.mod`，用 Docker 的 `golang:1.27.0-alpine` 构建验证。

## 13. 升级后检查清单

每次升级完成后至少确认：

- `git status --short --branch` 显示没有未解决冲突
- Docker build 后端编译通过
- 前端 build 通过
- `sub2api` 已重启且 healthy
- `ForwardPassthrough` / `IsOpenAIForcePassthrough` / `openAICompactRelayGroupName` 仍存在
- `/api/relay/openai/v1/chat/completions` 相关逻辑没有被上游改回 `/v1/responses`
- Grok 身份仍是 `grok-pager/1.0.3`，Grok 官方域仍走 `Grok Build (rustls 0.23)` + HTTP/2
- Grok OAuth 推理请求仍不带 `x-email` / `x-userid`
- Grok 强制 Priority 仍是账号 extra（`grok_force_priority_service_tier`），OAuth 默认开、API Key 默认关
- Grok 文本 usage 仍解析 `cost_in_usd_ticks`，Grok 账号仍用 ticks 覆盖总额

最终状态检查：

```bash
git status --short --branch
docker ps --filter name=sub2api \
  --format 'table {{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}'
docker logs --tail 80 sub2api
```

后续每次升级可以按这个格式记录：

```text
日期：
上游：
本地合并提交：
冲突文件：
保留的本地功能：
构建镜像：
Docker 状态：
验证结果：
遗留问题：
```

## 14. Cursor 价目（`cursor-*` 客户端 id）

Cursor 走本机/集群 `cursor-api-proxy` 时，客户端目录会给系列 id 加 `cursor-` 前缀（`kimi-k3` → `cursor-kimi-k3`）。proxy 自己会剥客户端前缀；Grok 原生 id（`cursor-grok-4.6`）不能剥。

计费必须认**原始** `cursor-*` 名称，不能落到 Grok Build / Moonshot 卡上：

- `GetModelPricing` / `HasIdentifiedTokenPricing` 对 `cursor-` 前缀走 Cursor 官网价目（https://cursor.com/docs/models-and-pricing）
- Fast 是模型 id 后缀 `-fast`，不是 OpenAI `service_tier`；Composer Fast 是 6x，Grok 4.5 Fast 的 output 是 3x，其余未单独标价的 Fast 默认 2x
- 未加前缀的 `composer-2.5` / `grok-composer-2.5-fast` 仍走 xAI Grok Build 卡
- 计费候选会把 `cursor-*` 排到前面，避免映射剥前缀后先命中 Kimi/Grok fallback
- 账号 `model_mapping` 支持捕获改写：`cursor-*` → `*` 会把 `cursor-kimi-k3` 变成 `kimi-k3`；更长的 `cursor-grok-*` → `cursor-grok-*` 优先。当前线上 Cursor 账号用 **passthrough + 恒等 mapping**（不改写，只给 `/v1/models` 挂目录）

回归：

```bash
cd backend && go test -tags unit ./internal/service -run 'CursorPrefixed|UnprefixedGrokComposer|StripsCursor|PrefersCursor|GrokCatalogFallbacks|MatchWildcardMappingResult'
```

## 15. 升级记录

```text
日期：2026-08-31
上游：Wei-Shaw/sub2api origin/main @ 52374af94 (v0.1.184)
本地合并提交：rebase origin/main（backup/pre-upstream-rebase-20260831）
        214116c00 feat(billing): add Cursor list prices for cursor-* client ids
        9ccc4ef95 fix(grok): pass account into service_tier billing resolution tests
冲突文件：
  - billing_service_test.go（保留上游 DeepSeek flash 兜底用例）
  - openai_gateway_chat_completions_raw.go（serviceTier 延后到 Grok force-priority 之后提取）
  - openai_gateway_grok_test.go（补 ApplyOpenAIServiceTierBillingResolution 的 account 参数）
保留的本地功能：
  - /api/relay/openai passthrough + openai-compact 优先
  - Grok Build 身份/TLS + 账号级 force Priority
  - Cursor cursor-* 价目 / 通配符捕获 mapping / 计费候选优先
构建镜像：未构建（等确认后再更新 k8s）
Docker 状态：未重启
验证结果：
  go test -tags unit ./internal/service -run 'CursorPrefixed|UnprefixedGrokComposer|StripsCursor|PrefersCursor|GrokCatalogFallbacks|MatchWildcardMappingResult|ForcePriority|ApplyOpenAIServiceTier' 通过
遗留问题：k8s 镜像仍是 rebase 前的 c93703cae，需确认后再部署
```

```text
日期：2026-09-01
上游：Wei-Shaw/sub2api origin/main @ a2fb09260 (v0.1.185)
本地合并提交：rebase origin/main
冲突文件：
  - billing_service.go（上游已删 GPT-5.4 长上下文常量；保留 fork 的 cnyToUSDFallbackExchangeRate）
保留的本地功能：
  - /api/relay/openai passthrough + openai-compact 优先
  - Grok Build 身份/TLS + 账号级 force Priority
  - Cursor cursor-* 价目 / 通配符捕获 mapping / 计费候选优先
  - Grok 上游 cost_in_usd_ticks 覆盖 TotalCost/ActualCost（见第 16 节）
构建镜像：未构建（等确认后再更新 k8s）
Docker 状态：未重启
验证结果：
  go test -tags unit ./internal/pkg/xai ./internal/service -run 'CostUSDFromTicks|OpenAIUsageFromGJSONParsesCostInUsdTicks|ApplyGrokUpstreamReportedCost|Grok46|XAIThresholdInclusive' 通过
遗留问题：k8s 仍跑 rebase 前镜像 sub2api-local:23276556c-amd64，需部署后 LiteLLM above_200k 折算和 ticks 才会进线上 usage_logs
```

```text
日期：2026-09-07
上游：Wei-Shaw/sub2api origin/main @ ab99d56e9 (v0.2.1)
本地合并提交：rebase origin/main（backup/pre-upstream-rebase-20260907）
        53ae8869d feat(grok): bill long-context cache and xAI cost ticks
冲突文件：
  - openai_gateway_grok.go（保留上游 UpstreamHeaders；保留 fork 的 outbound ServiceTier / UpstreamResponseServiceTier，经 resolvedOpenAIUpstreamServiceTier 写入）
保留的本地功能：
  - /api/relay/openai passthrough + openai-compact 优先
  - Grok Build 身份/TLS + 账号级 force Priority
  - Cursor cursor-* 价目 / 通配符捕获 mapping / 计费候选优先
  - Grok 上游 cost_in_usd_ticks 覆盖 TotalCost/ActualCost（见第 16 节）
构建镜像：未构建（等确认后再更新 k8s）
Docker 状态：未重启
验证结果：
  go test ./internal/pkg/xai ./internal/pkg/tlsfingerprint ./internal/repository ./internal/service ./internal/handler -run 'Grok|CLI|UpstreamHeaders|BuildGrok|ForcePriority|TestOpenAI|CursorPrefixed|UnprefixedGrokComposer|StripsCursor|PrefersCursor|GrokCatalogFallbacks|MatchWildcardMappingResult|CostUSDFromTicks|OpenAIUsageFromGJSONParsesCostInUsdTicks|ApplyGrokUpstreamReportedCost|StampsCustomAPIKeySessionID' 通过
  go test ./internal/service ./internal/handler 通过
遗留问题：
  - 工作区仍有未提交的 stampCustomOpenAIAPIKeySessionID（自定义 OpenAI api_key 的 session_id 粘性）
  - k8s / 本地 Docker 镜像仍是 rebase 前版本，需确认后再部署
```

```text
日期：2026-09-17
上游：Wei-Shaw/sub2api origin/main @ efe9aab1e (v0.2.5)
本地合并提交：rebase origin/main（backup/pre-upstream-rebase-20260917）
冲突文件：
  - gateway.go（保留 fork 的 /api/relay/openai Any 注册；embeddings 改用上游 rootRoute）
  - billing_service.go（保留上游 Gemini 3.7/3.8 Flash 兜底 + fork 的 GPT-5.4 等 relay fallback）
  - openai_gateway_chat_completions_raw.go（sanitizeGrokUnsupportedFields 之后再 applyGrokForcePriority；Ollama clamp 之后再 extract service_tier）
  - openai_gateway_usage.go（Cursor 用 inputTokensForBilling；保留上游 ImageCacheReadTokens 拆分）
  - openai_gateway_service.go（保留 ImageCacheReadTokens + CostInUsdTicks）
  - EditAccountModal.vue / en|zh accounts.ts（保留上游 grokMediaEligibility + fork 的 grokForcePriority）
保留的本地功能：
  - /api/relay/openai passthrough + openai-compact 优先
  - Grok Build 身份/TLS + 账号级 force Priority
  - Cursor cursor-* 价目 / 通配符捕获 mapping / 计费候选优先
  - Grok 上游 cost_in_usd_ticks 覆盖 TotalCost/ActualCost（见第 16 节）
构建镜像：未构建（等确认后再更新 k8s）
Docker 状态：未重启
验证结果：
  go test ./internal/pkg/xai ./internal/pkg/tlsfingerprint 通过
  go test ./internal/repository ./internal/service 通过
  go test ./internal/handler 通过
  go test ./internal/service ./internal/handler -run 'Grok|CLI|UpstreamHeaders|BuildGrok|ForcePriority|TestOpenAI|CursorPrefixed|UnprefixedGrokComposer|StripsCursor|PrefersCursor|GrokCatalogFallbacks|MatchWildcardMappingResult|CostUSDFromTicks|OpenAIUsageFromGJSONParsesCostInUsdTicks|ApplyGrokUpstreamReportedCost|StampsCustomAPIKeySessionID' 通过
遗留问题：
  - k8s / 本地 Docker 镜像仍是 rebase 前版本，需确认后再部署
```

## 16. Grok Build 自用额度（2026-09-01）

Grok OAuth 走 `cli-chat-proxy` 的 **GrokBuild 周 credits**，不是向用户售卖。官方价卡（https://docs.x.ai/developers/pricing）：

- grok-4.6：prompt < 200k 时 $2 / $0.50 cached / $6；含 cache 的 prompt ≥200k 时整单 2x
- cache 单独按 cached input 计价，不是免单
- 响应 `usage.cost_in_usd_ticks`（1 USD = 10^10 ticks）是单次真实扣费

上游 v0.1.185 已把 LiteLLM 的 `*_above_200k_tokens` 折成 `long_context_*` 阈值+倍率，xAI 阈值语义为达到即进高档。本 fork 在此之上：

- 解析并合并 `cost_in_usd_ticks`
- Grok 账号记账时用 ticks 覆盖 `TotalCost`/`ActualCost`（分项仍是本地估算，便于核对 cache）
- 无 ticks 的历史日志按「uncached + cache ≥ 200k 则 actual_cost×2」回算

相关文件：

- `backend/internal/service/pricing_service.go`（上游目录折算）
- `backend/internal/pkg/xai/billing.go`（`USDTicksPerDollar`）
- `backend/internal/service/grok_upstream_cost.go`
- `backend/internal/service/openai_gateway_usage.go`
- `backend/internal/service/openai_gateway_response_handling.go`


# 本地 Fork 升级运行手册

> 适用于本地 fork 同步上游 `Wei-Shaw/sub2api` 后，保留本地 OpenAI relay / passthrough / compact 定制功能，并重新构建 Docker 服务。

## 1. 升级目标

每次升级要同时满足三件事：

- 拉取并以 rebase 方式同步上游 `Wei-Shaw/sub2api` 最新 `main`
- 解决冲突时保留本地 fork 功能
- 构建并重启本地 Docker 服务，确认 `sub2api` 健康

当前本地运行服务：

- Compose 运行与构建文件：`deploy/docker-compose.local.yml`
- 应用容器：`sub2api`
- 本地访问：`http://127.0.0.1:8088`

## 2. 升级前检查

先确认远端、分支和工作区状态：

```bash
git remote -v
git status --short --branch
git branch -vv
```

当前约定：

- `origin` 指向 `https://github.com/Wei-Shaw/sub2api`
- 本地工作分支是 `main`
- 如果工作区有未提交改动，先确认是否属于本次升级；不要直接覆盖

建议先看本地 fork 相比上游的提交：

```bash
git log --oneline --decorate --left-right origin/main...main
git diff --stat origin/main...main
```

## 3. 拉取并 rebase 上游

```bash
git fetch origin
git rebase origin/main
```

如果发生冲突，先列出冲突文件：

```bash
git status --short
rg -n '<<<<<<<|=======|>>>>>>>|\|\|\|\|\|\|\|' .
```

解决完成后：

```bash
gofmt -w <changed-go-files>
git diff --check
git add <resolved-files>
git rebase --continue
```

如果上游远端弄错，或需要放弃本次同步：

```bash
git rebase --abort
```
```

## 4. 冲突处理原则

本地 fork 的核心功能不能被上游合并抹掉：

- `/api/relay/openai` relay 入口
- `service.IsOpenAIForcePassthrough(c)` 分支
- `ForwardPassthrough(...)`
- `openAIPassthroughRequestPath(c)` 保留原始上游 path
- `openAICompactRelayGroupName = "openai-compact"`
- Chat Completions passthrough usage 解析
- raw `/v1/chat/completions` upstream endpoint 记录

合并时常见冲突点：

- `backend/internal/handler/openai_chat_completions.go`
- `backend/internal/handler/openai_relay_passthrough.go`
- `backend/internal/service/openai_gateway_service.go`
- `backend/internal/service/openai_codex_transform.go`
- `backend/internal/service/billing_service.go`
- `backend/internal/service/openai_gateway_service_test.go`

Grok Build 定制的完整记录见 `docs/FORK_GROK_BUILD_ALIGNMENT.md`，核心文件包括：

- `backend/internal/pkg/xai/cli_identity.go`
- `backend/internal/pkg/tlsfingerprint/grok_build.go`
- `backend/internal/repository/http_upstream.go`
- `backend/internal/service/grok_upstream_headers.go`
- `backend/internal/service/openai_gateway_grok.go`

同步后建议确认这些符号还在：

```bash
rg -n 'ForwardPassthrough|IsOpenAIForcePassthrough|openAICompactRelayGroupName|openAIPassthroughRequestPath' \
  backend/internal/handler backend/internal/service

rg -n 'GrokBuildProfileName|ApplyCLIIdentityHeaders|applyGrokOAuthInferenceHeaders|grok-pager/1.0.3' \
  backend/internal/pkg backend/internal/repository backend/internal/service
```

## 5. Flake 和 Go 版本注意点

仓库包含 `flake.nix`，当前 dev shell 配置：

```nix
packages = with pkgs; [
  go_1_26
  gopls
  gotools
  gcc
  git
  gnumake
  pkg-config
];
```

但要注意：`go_1_26` 不一定等于上游 `go.mod` 要求的精确 patch 版本。

例如本次升级后：

- `backend/go.mod` 要求 `go 1.26.6`
- 本机 Go 版本低于该 patch 时，直接跑 `go test` 会触发自动下载 `go1.26.6`
- `GOTOOLCHAIN=local` 会因为版本不足失败

检查方式：

```bash
nix develop --command bash -lc 'go version && go env GOTOOLCHAIN GOVERSION'
sed -n '1,5p' backend/go.mod
```

如果本机 Go 验证被工具链下载卡住，优先用 Docker 构建验证；本地 Compose 的 `GOLANG_IMAGE` 必须与当前 `backend/go.mod` 的 Go 版本一致：

```yaml
GOLANG_IMAGE: public.ecr.aws/docker/library/golang:1.26.6-alpine
```

## 6. 推荐验证命令

如果 Nix 提供的 Go 版本满足 `go.mod`：

```bash
nix develop --command bash -lc '
  cd backend &&
  go test -run "^$" ./internal/service ./internal/handler
'
```

如果 Nix Go patch 版本低于 `go.mod`，用 Docker build 做实际编译验证。

## 7. Docker 重建与重启

推荐使用当前 commit 作为本地镜像 tag：

```bash
SUB2API_BUILD_VERSION=local-$(git rev-parse --short HEAD) \
SUB2API_BUILD_COMMIT=$(git rev-parse --short HEAD) \
docker compose \
  -f deploy/docker-compose.local.yml \
  up -d --build sub2api
```

构建成功后会自动重建并启动 `sub2api`，数据库和 Redis 不会被重建。

检查容器状态：

```bash
docker ps --filter name=sub2api \
  --format 'table {{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}'
```

期望：

- `sub2api` 状态是 `Up ... (healthy)`
- 镜像类似 `sub2api-local:local-<git-sha>`
- 端口仍是 `127.0.0.1:8088->8080/tcp`

查看启动日志：

```bash
docker logs --tail 100 sub2api
```

## 8. 常见问题

### 8.1 Docker 构建时 Go module 下载失败

可能看到：

```text
go mod download
Get "https://goproxy.cn/...": unexpected EOF
```

这通常是代理或网络临时问题。先直接重试同一条 Docker build 命令。

### 8.2 `SelectAccountWithScheduler` 参数数量不匹配

上游可能给调度函数新增参数。合并本地 passthrough 调用时，按上游签名补齐。

这次对应新增参数是 `requireCompact bool`，普通 passthrough relay 调用使用：

```go
service.OpenAIUpstreamTransportAny,
false,
```

compact 专用路径再按实际需要传 `true`。

### 8.3 默认模型 helper 被上游删除

如果出现：

```text
undefined: resolveOpenAIForwardDefaultMappedModel
```

保守做法是直接读取 API key 绑定组的默认映射模型：

```go
defaultMappedModel := ""
if apiKey.Group != nil {
    defaultMappedModel = apiKey.Group.DefaultMappedModel
}
```

然后继续传给：

```go
ForwardAsChatCompletions(..., defaultMappedModel)
```

### 8.4 Flake Go 版本低于 `go.mod`

如果看到：

```text
go.mod requires go >= 1.26.6 (running go 1.26.1; GOTOOLCHAIN=local)
```

说明 Nix 当前 `go_1_26` 落后于 `go.mod` patch 版本。不要为了升级临时改 `go.mod`，优先用 Docker 的 `golang:1.26.6-alpine` 构建验证。

## 9. 升级后检查清单

每次升级完成后至少确认：

- `git status --short --branch` 显示没有未解决冲突
- Docker build 后端编译通过
- 前端 build 通过
- `sub2api` 已重启且 healthy
- `ForwardPassthrough` / `IsOpenAIForcePassthrough` / `openAICompactRelayGroupName` 仍存在
- `/api/relay/openai/v1/chat/completions` 相关逻辑没有被上游改回 `/v1/responses`
- Grok 身份仍是 `grok-pager/1.0.3`，Grok 官方域仍走 `Grok Build (rustls 0.23)` + HTTP/2
- Grok OAuth 推理请求仍不带 `x-email` / `x-userid`

最终状态检查：

```bash
git status --short --branch
docker ps --filter name=sub2api \
  --format 'table {{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}'
docker logs --tail 80 sub2api
```

## 10. 本次升级记录模板

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

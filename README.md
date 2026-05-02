# ai-mini-gateway

`ai-mini-gateway` 是一个独立运行、独立发布的本地 AI gateway runtime。

当前实现目标：

1. 满足 `third-party-gateway-contract-spec.md` 中的 required runtime contract
2. 以单 binary 方式启动
3. 通过稳定 HTTP contract 暴露 OpenAI-compatible / Anthropic-compatible 接口
4. 独立维护 runtime 数据目录与凭据文件

## Build

```bash
go build -o ./bin/ai-mini-gateway ./cmd/gateway
```

发布构建建议注入版本信息：

```bash
go build \
  -ldflags "-X main.version=0.1.0 -X main.commit=$(git rev-parse --short HEAD)" \
  -o ./bin/ai-mini-gateway \
  ./cmd/gateway
```

其中：

1. `version` 对应 Git tag / GitHub Release 版本
2. `contract_version` 对应对外 HTTP contract 兼容版本，当前固定为 `v1`
3. `contract_version` 当前仅从 `/health` 暴露，作为调用方读取兼容版本的单一入口

## Run

```bash
go run ./cmd/gateway --host 127.0.0.1 --port 3457 --data-dir ./data
```

```bash
./bin/ai-mini-gateway --host 127.0.0.1 --port 3457 --data-dir ./data --models-cache-ttl 15s
```

也支持 embedded adapter 传入的环境变量，flags 优先级高于 env：

```bash
LOCAL_GATEWAY_RUNTIME_HOST=127.0.0.1 \
LOCAL_GATEWAY_RUNTIME_PORT=3457 \
CORE_DATA_DIR=./data \
./bin/ai-mini-gateway --models-cache-ttl 15s
```

## Implemented Endpoints

1. `GET /health`
2. `GET /v1/models`
3. `POST /v1/chat/completions`
4. `POST /v1/responses`
5. `POST /v1/messages`
6. `POST /v1/messages/count_tokens`
7. `GET/POST/PUT/DELETE /admin/model-sources`
8. `PUT /admin/model-sources/order`
9. `GET/PUT /admin/selected-models`
10. `GET /capabilities`
11. `GET /admin/model-sources/capabilities`
12. `PUT /admin/runtime/sync`
13. `POST /admin/model-sources/:id/healthcheck`
14. `GET /runtime/status`

## Embedded Validation

最小 embedded / external 启动验证脚本：

```bash
./scripts/validate-embedded.sh
```

它会：

1. 构建本地 binary
2. 使用 embedded 环境变量和独立 data dir 启动 runtime
3. 检查 `/health`
4. 检查 `/capabilities`
5. 退出并清理子进程

## Web Console

独立前端管理页面位于：

```bash
web/
```

它不由当前 Go runtime 内嵌提供，而是作为单独的 React + Vite + TailwindCSS 项目存在，通过 API 访问 gateway。

开发态推荐：

1. 单独启动 gateway
2. 在 `web/` 目录启动前端 dev server
3. 通过 Vite proxy 或 `VITE_API_BASE_URL` 访问 gateway API

详细说明见 [web/README.md](web/README.md)。

## Release Notes

发布时建议产物：

1. 单个可执行文件 `ai-mini-gateway`
2. 对应版本号与 commit id
3. 一份最小启动参数说明
4. 一份 capability / admin endpoint 列表

可参考 [docs/RELEASE.md](docs/RELEASE.md)。

## Current Notes

1. runtime 状态持久化使用 SQLite，credentials 保持独立 JSON 文件
2. 当前 provider 执行链路已支持基于 compatible HTTP contract 的真实上游转发
3. provider 抽象已覆盖认证头、path、header forwarding、请求校验、错误归一和基础 capability 状态
4. `/health` 会返回 `runtime_kind`、`version`、`commit`、`contract_version`
5. `/capabilities` 会返回增强能力字段，例如 `supports_atomic_source_sync` 和 `supports_runtime_version`
6. `POST /admin/model-sources/:id/healthcheck` 可显式校验单条 source 的可达性
7. `GET /runtime/status` 会返回 `last_applied_at`、`sync_in_progress`、`last_sync_error` 等稳定运行态信息

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

## Run

```bash
go run ./cmd/gateway --host 127.0.0.1 --port 3457 --data-dir ./data
```

```bash
./bin/ai-mini-gateway --host 127.0.0.1 --port 3457 --data-dir ./data --models-cache-ttl 15s
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

## Embedded Validation

最小 embedded / external 启动验证脚本：

```bash
./scripts/validate-embedded.sh
```

它会：

1. 构建本地 binary
2. 使用独立 data dir 启动 runtime
3. 检查 `/health`
4. 检查 `/capabilities`
5. 退出并清理子进程

## Release Notes

发布时建议产物：

1. 单个可执行文件 `ai-mini-gateway`
2. 对应版本号与 commit id
3. 一份最小启动参数说明
4. 一份 capability / admin endpoint 列表

可参考 [RELEASE.md](RELEASE.md)。

## Current Notes

1. runtime 状态持久化使用 SQLite，credentials 保持独立 JSON 文件
2. 当前 provider 执行链路已支持基于 compatible HTTP contract 的真实上游转发
3. provider 抽象已覆盖认证头、path、header forwarding、请求校验、错误归一和基础 capability 状态

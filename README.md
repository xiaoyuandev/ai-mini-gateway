# ai-mini-gateway

`ai-mini-gateway` 是一个独立运行、独立发布的本地 AI gateway runtime。

当前实现目标：

1. 满足 `third-party-gateway-contract-spec.md` 中的 required runtime contract
2. 以单 binary 方式启动
3. 通过稳定 HTTP contract 暴露 OpenAI-compatible / Anthropic-compatible 接口
4. 独立维护 runtime 数据目录与凭据文件

## Run

```bash
go run ./cmd/gateway --host 127.0.0.1 --port 3457 --data-dir ./data
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

## Current Notes

1. 当前推理执行器是 contract-first 的本地 echo 实现，用于先打通 runtime API、stream 语义和管理面
2. runtime 状态持久化使用 SQLite，credentials 保持独立 JSON 文件
3. 当前 provider 执行链路仍是本地 stub，下一步需要补真实 upstream 转发与 source 级路由

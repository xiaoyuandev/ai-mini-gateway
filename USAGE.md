# Usage Guide

`ai-mini-gateway` 是一个独立运行的本地 AI gateway runtime。

## Build

```bash
go build -o ./bin/ai-mini-gateway ./cmd/gateway
```

## Start

直接运行源码：

```bash
go run ./cmd/gateway \
  --host 127.0.0.1 \
  --port 3457 \
  --data-dir ./data \
  --models-cache-ttl 15s
```

运行编译后的 binary：

```bash
./bin/ai-mini-gateway \
  --host 127.0.0.1 \
  --port 3457 \
  --data-dir ./data \
  --models-cache-ttl 15s
```

参数说明：

1. `--host`：监听地址，默认 `127.0.0.1`
2. `--port`：监听端口，默认 `3457`
3. `--data-dir`：runtime 数据目录，包含 SQLite 和 credentials 文件
4. `--models-cache-ttl`：上游 models / capability 状态缓存 TTL，默认 `15s`

## Stop

前台运行时：

1. `Ctrl+C`
2. 或发送 `SIGTERM`

后台运行时：

```bash
pkill -f ai-mini-gateway
```

也可以按 PID 停止：

```bash
kill <PID>
```

服务会走标准 `http.Server.Shutdown`，默认等待 10 秒优雅退出。

## Smoke Checks

健康检查：

```bash
curl -s http://127.0.0.1:3457/health
```

能力检查：

```bash
curl -s http://127.0.0.1:3457/capabilities
```

模型检查：

```bash
curl -s http://127.0.0.1:3457/v1/models
```

## Management API

### 1. List Model Sources

```bash
curl -s http://127.0.0.1:3457/admin/model-sources
```

### 2. Create Model Source

OpenAI-compatible:

```bash
curl -s -X POST http://127.0.0.1:3457/admin/model-sources \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "OpenAI",
    "base_url": "https://api.openai.com/v1",
    "provider_type": "openai-compatible",
    "default_model_id": "gpt-4.1",
    "exposed_model_ids": ["gpt-4.1-mini"],
    "enabled": true,
    "api_key": "sk-xxx"
  }'
```

Anthropic-compatible:

```bash
curl -s -X POST http://127.0.0.1:3457/admin/model-sources \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "Anthropic",
    "base_url": "https://api.anthropic.com/v1",
    "provider_type": "anthropic-compatible",
    "default_model_id": "claude-3-7-sonnet",
    "exposed_model_ids": ["claude-3-haiku"],
    "enabled": true,
    "api_key": "sk-ant-xxx"
  }'
```

### 3. Update Model Source

```bash
curl -s -X PUT http://127.0.0.1:3457/admin/model-sources/<SOURCE_ID> \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "OpenAI Primary",
    "base_url": "https://api.openai.com/v1",
    "provider_type": "openai-compatible",
    "default_model_id": "gpt-4.1",
    "exposed_model_ids": ["gpt-4.1-mini", "gpt-4.1-nano"],
    "enabled": true,
    "api_key": "sk-xxx"
  }'
```

### 4. Delete Model Source

```bash
curl -i -X DELETE http://127.0.0.1:3457/admin/model-sources/<SOURCE_ID>
```

### 5. Reorder Model Sources

```bash
curl -s -X PUT http://127.0.0.1:3457/admin/model-sources/order \
  -H 'Content-Type: application/json' \
  -d '[
    {"id": "src_a", "position": 0},
    {"id": "src_b", "position": 1}
  ]'
```

### 6. List Source Capabilities

```bash
curl -s http://127.0.0.1:3457/admin/model-sources/capabilities
```

返回会包含：

1. `supports_models_api`
2. `models_api_status`
3. `supports_openai_chat_completions`
4. `supports_openai_responses`
5. `supports_anthropic_messages`
6. `supports_anthropic_count_tokens`
7. `supports_stream`

### 7. List Selected Models

```bash
curl -s http://127.0.0.1:3457/admin/selected-models
```

### 8. Update Selected Models

```bash
curl -s -X PUT http://127.0.0.1:3457/admin/selected-models \
  -H 'Content-Type: application/json' \
  -d '[
    {"model_id": "claude-3-7-sonnet", "position": 0},
    {"model_id": "gpt-4.1-mini", "position": 1}
  ]'
```

## Inference Examples

### OpenAI Chat Completions

```bash
curl -s http://127.0.0.1:3457/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "gpt-4.1-mini",
    "messages": [{"role": "user", "content": "hello"}]
  }'
```

### OpenAI Responses

```bash
curl -s http://127.0.0.1:3457/v1/responses \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "gpt-4.1-mini",
    "input": "hello"
  }'
```

### Anthropic Messages

```bash
curl -s http://127.0.0.1:3457/v1/messages \
  -H 'Content-Type: application/json' \
  -H 'anthropic-version: 2023-06-01' \
  -d '{
    "model": "claude-3-7-sonnet",
    "max_tokens": 128,
    "messages": [{"role": "user", "content": "hello"}]
  }'
```

### Anthropic Count Tokens

```bash
curl -s http://127.0.0.1:3457/v1/messages/count_tokens \
  -H 'Content-Type: application/json' \
  -H 'anthropic-version: 2023-06-01' \
  -d '{
    "model": "claude-3-7-sonnet",
    "messages": [{"role": "user", "content": "hello"}]
  }'
```

## Validation Script

```bash
./scripts/validate-embedded.sh
```

脚本会自动：

1. 构建 binary
2. 启动 runtime
3. 验证 `/health`
4. 验证 `/capabilities`
5. 退出并清理进程

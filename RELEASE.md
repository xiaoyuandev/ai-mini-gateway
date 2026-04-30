# Release Guide

`ai-mini-gateway` 以单 binary 方式发布。

## Build

```bash
go build -o ./bin/ai-mini-gateway ./cmd/gateway
```

## Required Runtime Arguments

```bash
./bin/ai-mini-gateway \
  --host 127.0.0.1 \
  --port 3457 \
  --data-dir ./data \
  --models-cache-ttl 15s
```

## Expected Deliverables

1. `ai-mini-gateway` 可执行文件
2. 对应版本号
3. 对应 git commit id
4. 一份启动参数说明
5. 一份对外 endpoint / capability 列表

## Smoke Checks

发布前建议至少检查：

1. `GET /health`
2. `GET /capabilities`
3. `GET /v1/models`
4. `GET /admin/model-sources`

## Embedded / External Validation

可直接运行：

```bash
./scripts/validate-embedded.sh
```

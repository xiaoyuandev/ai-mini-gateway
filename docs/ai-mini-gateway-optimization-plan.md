# ai-mini-gateway Optimization Plan

## 1. 目的

本文档用于沉淀 `ai-mini-gateway` 配合 `Local Gateway` 整改的 runtime 侧优化计划。

目标是：

1. 对齐 [local-gateway-review-and-optimization.md](./local-gateway-review-and-optimization.md) 中需要 runtime 配合的整改建议
2. 保持当前已上线 P0 contract 不被破坏
3. 为后续每个阶段的开发、测试和验收提供统一清单

## 2. 适用范围

本文档只覆盖 `ai-mini-gateway` 自身需要新增或增强的运行时能力，不扩展到 `clash-for-ai` 产品层实现。

当前边界保持不变：

1. `clash-for-ai` 继续持有产品主数据
2. `ai-mini-gateway` 继续作为本地 runtime 执行层
3. 当前已存在的 health / capabilities / admin / inference contract 不应被破坏
4. 新能力应尽量复用现有 `state`、`admin`、`executor`、`handler` 模块边界

## 3. 输入依据

本计划主要依据以下文档：

1. [docs/local-gateway-review-and-optimization.md](./local-gateway-review-and-optimization.md)
2. [docs/ai-mini-gateway-integration-contract.md](./ai-mini-gateway-integration-contract.md)

其中与 `ai-mini-gateway` 直接相关的重点建议是：

1. 原子化全量 sync / replace
2. runtime 版本和 commit 信息
3. source 显式 validate / healthcheck 能力
4. 更稳定的 runtime 状态接口

## 4. 总体实施原则

后续开发必须遵守以下原则：

1. 不破坏当前已实现的 P0 endpoint
2. 新增能力优先通过增量 endpoint 和增量字段提供
3. 所有 admin 类写操作都必须返回稳定 JSON 错误结构
4. 涉及运行态整体替换的操作必须具备明确的一致性语义
5. 新能力应优先服务于 adapter 降复杂度，而不是把产品主数据重新下沉到 runtime

## 5. 当前结论

当前 `ai-mini-gateway` 已具备：

1. `GET /health`
2. `GET /capabilities`
3. `GET /admin/model-sources`
4. `GET /admin/model-sources/capabilities`
5. `GET/PUT /admin/selected-models`
6. 基础 source CRUD / reorder
7. OpenAI-compatible / Anthropic-compatible inference endpoints

当前已完成：

1. 原子化全量 sync 接口
2. runtime 版本 / commit 暴露

当前仍缺少的 runtime 协作能力：

1. 显式 source healthcheck / validate
2. 更稳定的 runtime status 输出

## 6. 阶段计划

### 阶段 A: 原子化全量 Sync

Status: [x] Completed

#### 目标

新增 `PUT /admin/runtime/sync`，支持一次性提交全部 `sources` 和 `selected_models`，成功前不破坏当前已生效配置。

#### 设计要求

1. 接口采用全量替换语义
2. 成功后 runtime 内部状态必须与请求一致
3. 失败时整体失败，不允许部分静默成功
4. 同一时刻只允许一个 sync 执行
5. 并发 sync 请求应返回 `409 conflict`

#### 建议请求结构

```json
{
  "sources": [
    {
      "external_id": "local-source-1",
      "name": "OpenAI",
      "base_url": "https://api.openai.com/v1",
      "api_key": "sk-xxx",
      "provider_type": "openai-compatible",
      "default_model_id": "gpt-4.1",
      "exposed_model_ids": ["gpt-4.1-mini"],
      "enabled": true,
      "position": 0
    }
  ],
  "selected_models": [
    { "model_id": "gpt-4.1", "position": 0 }
  ]
}
```

#### 建议响应结构

```json
{
  "applied_sources": 1,
  "applied_selected_models": 1,
  "last_synced_at": "2026-05-01T15:23:46Z"
}
```

#### 实现拆分

1. 在 `internal/runtime/admin` 增加 `PUT /admin/runtime/sync`
2. 在 `internal/runtime/state` 增加事务化全量替换能力
3. 在 `state` 层复用现有 source / selected model 校验
4. 在 runtime 内部增加 sync 锁和最后一次成功同步时间
5. sync 成功后统一失效 models cache

#### 外部标识建议

为了降低 adapter 复杂度，建议同步支持 source 的稳定 `external_id` 字段。

本阶段已在 source 结构中预留 `external_id` 字段，但当前阶段主能力仍以原子化全量 sync 为核心。

#### 验收标准

1. 错误输入不会清空原有可用 runtime 配置
2. 并发 sync 时只有一个请求进入执行阶段
3. sync 成功后 `GET /admin/model-sources` 与 `GET /admin/selected-models` 立即反映新状态
4. sync 失败时错误结构稳定，且 runtime 保持旧配置

#### 测试清单

1. 正常 sync 成功
2. 非法 source 输入导致 sync 失败
3. 非法 selected model 导致 sync 失败
4. 并发 sync 返回冲突错误
5. sync 成功后 cache 被正确失效

### 阶段 B: Runtime 版本信息与能力声明

Status: [x] Completed

#### 目标

让产品层能稳定获取 runtime 的 `version`、`commit` 和增强能力开关。

#### 设计要求

1. 在不破坏现有兼容性的前提下扩展 `/health`
2. 在 `/capabilities` 中补充增强能力字段
3. `version` 和 `commit` 应通过 build metadata 注入

#### 建议字段

`GET /health` 建议返回：

```json
{
  "status": "ok",
  "version": "0.1.0",
  "commit": "abcdef1",
  "runtime_kind": "ai-mini-gateway"
}
```

`GET /capabilities` 建议新增：

```json
{
  "supports_source_capabilities": true,
  "supports_atomic_source_sync": true,
  "supports_runtime_version": true,
  "supports_explicit_source_health": false
}
```

#### 实现拆分

1. 在 `cmd/gateway` 注入 `version` / `commit`
2. 将 build metadata 传入 `health` / `capability` handler
3. 扩展现有响应结构与测试

#### 验收标准

1. `/health` 仍保持 JSON 响应
2. `/capabilities` 现有必需字段不缺失
3. 新增字段值稳定且可预测

#### 测试清单

1. 默认 build metadata 下返回预期占位值
2. 注入 `version` / `commit` 后输出正确
3. 旧有 contract 测试全部通过

### 阶段 C: Source 显式 Healthcheck / Validate

Status: [ ] Pending

#### 目标

提供一个显式来源校验接口，避免产品层将“观测能力状态”和“主动校验动作”混用。

#### 接口策略

优先实现：

1. `POST /admin/model-sources/:id/healthcheck`

备选实现：

1. `POST /admin/model-sources/validate`

#### 建议响应结构

```json
{
  "status": "ok",
  "status_code": 200,
  "latency_ms": 321,
  "summary": "HTTP 200",
  "checked_at": "2026-05-01T15:30:00Z"
}
```

#### 设计要求

1. 校验动作应是显式触发，而不是读取 capabilities 时隐式触发
2. 单条 source 校验失败不应影响其他 source 状态
3. 错误返回应保持稳定 JSON 结构
4. 校验逻辑应尽量轻量

#### 探测策略建议

1. 优先探测 `/models`
2. 若 provider 明确不支持 `/models`，则回退到 provider 的最小推理路径
3. 记录状态码、耗时和摘要信息

#### 实现拆分

1. 在 `executor` 增加最小探测能力
2. 在 `admin` 增加 healthcheck endpoint
3. 在 provider 抽象中补充必要的探测策略

#### 验收标准

1. 正常 source 能得到明确成功结果
2. 错误 base URL / 错误 key / 不支持接口能得到明确失败结果
3. 不需要依赖 inference 请求也能做来源校验

#### 测试清单

1. 正常探测成功
2. 上游 401 / 404 / 5xx 返回时结果明确
3. 不支持 `/models` 的 provider 能走降级路径

### 阶段 D: 更稳定的 Runtime Status

Status: [ ] Pending

#### 目标

新增一个比 `/health` 更完整的 runtime 状态接口，减少产品层只靠“进程 + health”推断运行态。

#### 建议 Endpoint

1. `GET /runtime/status`

#### 建议响应结构

```json
{
  "runtime_kind": "ai-mini-gateway",
  "status": "ok",
  "version": "0.1.0",
  "commit": "abcdef1",
  "host": "127.0.0.1",
  "port": 3457,
  "data_dir": "./data",
  "last_applied_at": "2026-05-01T15:23:46Z",
  "sync_in_progress": false,
  "last_sync_error": ""
}
```

#### 设计要求

1. 输出应偏稳定状态而不是调试日志
2. 需要能反映最近一次成功 sync 时间
3. 若上一轮 sync 失败，应能暴露最近失败摘要

#### 实现拆分

1. 在 `state` 层增加 runtime metadata 持久化
2. 记录 `last_applied_at`
3. 记录 `last_sync_error`
4. 新增 `runtime/status` handler

#### 验收标准

1. runtime status 能反映基础运行态
2. sync 成功 / 失败后状态能更新
3. 不破坏 `/health` 的最小兼容用途

#### 测试清单

1. 冷启动状态输出正确
2. sync 成功后 `last_applied_at` 更新
3. sync 失败后 `last_sync_error` 更新

## 7. 推荐实施顺序

建议按以下顺序推进：

1. 阶段 A: 原子化全量 Sync
2. 阶段 B: Runtime 版本信息与能力声明
3. 阶段 C: Source 显式 Healthcheck / Validate
4. 阶段 D: 更稳定的 Runtime Status

原因：

1. 阶段 A 直接解决当前最核心的一致性风险
2. 阶段 B 为 adapter 做能力判断和升级判断提供基础
3. 阶段 C 降低 source 配置错误的排查成本
4. 阶段 D 完善产品层运行态感知

## 8. 开发阶段核对清单

每个阶段开始前都应确认：

1. 当前阶段是否新增了 endpoint、字段或持久化结构
2. 是否需要更新 contract 测试
3. 是否需要更新 `README.md`、`docs/USAGE.md`、`docs/RELEASE.md`
4. 是否会影响现有 P0 endpoint 行为

每个阶段完成后都应确认：

1. 单元测试已覆盖新增核心语义
2. 现有 router / store / provider / executor 测试未回归
3. 文档和实际实现一致
4. 错误响应保持稳定 JSON 结构

## 9. 联调验收清单

完成全部阶段后，至少应通过以下联调：

1. 使用真实 binary 启动 runtime
2. 验证 `/health`
3. 验证 `/capabilities`
4. 验证 `/admin/runtime/sync`
5. 验证 `/admin/model-sources/capabilities`
6. 验证 source healthcheck / validate
7. 验证 `/runtime/status`
8. 验证 OpenAI-compatible / Anthropic-compatible 推理链路未被破坏

## 10. 后续说明

后续每一阶段实现时，均以本文档为直接开发清单。

如阶段设计发生变化，应优先更新本文档，再进入实现。

# web

`web/` 是 `ai-mini-gateway` 的独立前端管理端原型。

技术栈：

1. React
2. TypeScript
3. Vite
4. TailwindCSS

目标：

1. 前后端目录解耦
2. 通过 HTTP API 调用 gateway
3. 后续可直接拆成独立仓库

## Development

先启动 gateway：

```bash
go run ./cmd/gateway --host 127.0.0.1 --port 3457 --data-dir ./data
```

再启动前端：

```bash
cd web
pnpm install
pnpm dev
```

默认开发模式通过 Vite proxy 把下面这些路径转发到 `http://127.0.0.1:3457`：

1. `/health`
2. `/capabilities`
3. `/v1/*`
4. `/admin/*`

如果你后续把前端部署为独立站点，可以设置：

```bash
VITE_API_BASE_URL=https://your-gateway-host
```

## Current Scope

当前页面覆盖：

1. runtime 健康检查与 capability 探测
2. model source 列表、创建、更新、删除、排序
3. selected models 管理
4. OpenAI / Anthropic compatible 请求 playground

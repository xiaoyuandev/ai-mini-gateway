import { useEffect, useState } from "react"

import { PlaygroundCard } from "./components/PlaygroundCard"
import { ResponsePanel } from "./components/ResponsePanel"
import { SectionCard } from "./components/SectionCard"
import { SelectedModelsEditor } from "./components/SelectedModelsEditor"
import { SourceCapabilitiesTable } from "./components/SourceCapabilitiesTable"
import { SourceForm } from "./components/SourceForm"
import { SourceList } from "./components/SourceList"
import { StatusBadge } from "./components/StatusBadge"
import { gatewayRequest } from "./lib/api"
import type {
  ApiResult,
  GatewayCapabilities,
  GatewayHealth,
  ModelItem,
  ModelSource,
  ModelSourceCapabilityRow,
  ModelSourceInput,
  ModelsResponse,
  PlaygroundDefinition,
  SelectedModel,
  SourceFormState
} from "./lib/types"

const initialSourceForm: SourceFormState = {
  name: "",
  base_url: "",
  provider_type: "openai-compatible",
  default_model_id: "",
  exposed_model_ids_text: "",
  enabled: true,
  api_key: ""
}

const playgroundDefinitions: PlaygroundDefinition[] = [
  {
    id: "openai-chat",
    title: "OpenAI Chat Completions",
    method: "POST",
    path: "/v1/chat/completions",
    provider: "openai-compatible",
    defaultHeaders: {
      "x-trace-id": "trace-openai"
    },
    buildPayload(model) {
      return {
        model,
        messages: [
          { role: "system", content: "You are a helpful tester." },
          { role: "user", content: "Say hello from ai-mini-gateway." }
        ]
      }
    }
  },
  {
    id: "openai-responses",
    title: "OpenAI Responses",
    method: "POST",
    path: "/v1/responses",
    provider: "openai-compatible",
    defaultHeaders: {
      "x-trace-id": "trace-openai"
    },
    buildPayload(model) {
      return {
        model,
        input: "Summarize the runtime state in one sentence."
      }
    }
  },
  {
    id: "anthropic-messages",
    title: "Anthropic Messages",
    method: "POST",
    path: "/v1/messages",
    provider: "anthropic-compatible",
    defaultHeaders: {
      "anthropic-version": "2023-06-01",
      "x-request-id": "trace-anthropic"
    },
    buildPayload(model) {
      return {
        model,
        max_tokens: 128,
        messages: [{ role: "user", content: "Say hello from ai-mini-gateway." }]
      }
    }
  },
  {
    id: "anthropic-count-tokens",
    title: "Anthropic Count Tokens",
    method: "POST",
    path: "/v1/messages/count_tokens",
    provider: "anthropic-compatible",
    defaultHeaders: {
      "anthropic-version": "2023-06-01"
    },
    buildPayload(model) {
      return {
        model,
        messages: [{ role: "user", content: "Count my input tokens." }]
      }
    }
  }
]

function parseModelCSV(value: string) {
  return value
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean)
}

function sortSources(items: ModelSource[]) {
  return items.slice().sort((a, b) => a.position - b.position)
}

function createClientResult(method: string, path: string, statusText: string, message: string): ApiResult {
  return {
    ok: false,
    status: 400,
    statusText,
    contentType: "application/json",
    data: { error: statusText, message },
    rawText: message,
    displayBody: JSON.stringify({ error: statusText, message }, null, 2),
    method,
    path
  }
}

function capabilityEntries(capabilities: GatewayCapabilities | null) {
  if (!capabilities) {
    return []
  }

  return Object.entries(capabilities).map(([key, value]) => ({
    key,
    value: value ? "true" : "false"
  }))
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value)
}

export default function App() {
  const [response, setResponse] = useState<ApiResult | null>(null)
  const [health, setHealth] = useState<GatewayHealth | null>(null)
  const [capabilities, setCapabilities] = useState<GatewayCapabilities | null>(null)
  const [sources, setSources] = useState<ModelSource[]>([])
  const [sourceCapabilities, setSourceCapabilities] = useState<ModelSourceCapabilityRow[]>([])
  const [selectedModels, setSelectedModels] = useState<SelectedModel[]>([])
  const [models, setModels] = useState<ModelItem[]>([])
  const [sourceForm, setSourceForm] = useState<SourceFormState>(initialSourceForm)
  const [editingSourceID, setEditingSourceID] = useState<string | null>(null)
  const [sourceOrderDirty, setSourceOrderDirty] = useState(false)
  const [pendingKey, setPendingKey] = useState<string | null>("bootstrap")

  const editingSource = sources.find((item) => item.id === editingSourceID) || null
  const sourceKeyNote = editingSource
    ? `当前密钥状态：${editingSource.api_key_masked || "未设置"}。更新 source 时如果不重新填写 API key，后端会把原值清空。`
    : "新建 source 时会直接保存 API key。"

  async function performRequest<T>(
    method: string,
    path: string,
    options?: { headers?: Record<string, string>; body?: unknown; recordResponse?: boolean }
  ) {
    const result = await gatewayRequest<T>(method, path, {
      headers: options?.headers,
      body: options?.body
    })
    if (options?.recordResponse !== false) {
      setResponse(result)
    }
    return result
  }

  async function reloadHealth(recordResponse = false) {
    const result = await performRequest<GatewayHealth>("GET", "/health", { recordResponse })
    if (result.ok && result.data && typeof result.data === "object" && "status" in result.data) {
      setHealth(result.data as GatewayHealth)
    }
    return result
  }

  async function reloadCapabilities(recordResponse = false) {
    const result = await performRequest<GatewayCapabilities>("GET", "/capabilities", { recordResponse })
    if (result.ok && result.data && typeof result.data === "object" && "supports_admin_api" in result.data) {
      setCapabilities(result.data as GatewayCapabilities)
    }
    return result
  }

  async function reloadSources(recordResponse = false) {
    const result = await performRequest<ModelSource[]>("GET", "/admin/model-sources", { recordResponse })
    if (result.ok && Array.isArray(result.data)) {
      setSources(sortSources(result.data as ModelSource[]))
      setSourceOrderDirty(false)
    }
    return result
  }

  async function reloadSourceCapabilities(recordResponse = false) {
    const result = await performRequest<ModelSourceCapabilityRow[]>("GET", "/admin/model-sources/capabilities", { recordResponse })
    if (result.ok && Array.isArray(result.data)) {
      setSourceCapabilities(result.data as ModelSourceCapabilityRow[])
    }
    return result
  }

  async function reloadSelectedModels(recordResponse = false) {
    const result = await performRequest<SelectedModel[]>("GET", "/admin/selected-models", { recordResponse })
    if (result.ok && Array.isArray(result.data)) {
      setSelectedModels(result.data as SelectedModel[])
    }
    return result
  }

  async function reloadModels(recordResponse = false) {
    const result = await performRequest<ModelsResponse>("GET", "/v1/models", { recordResponse })
    if (
      result.ok &&
      result.data &&
      typeof result.data === "object" &&
      "data" in result.data &&
      Array.isArray((result.data as ModelsResponse).data)
    ) {
      setModels((result.data as ModelsResponse).data)
    }
    return result
  }

  async function bootstrap() {
    setPendingKey("bootstrap")
    try {
      await Promise.allSettled([
        reloadHealth(false),
        reloadCapabilities(false),
        reloadSources(false),
        reloadSourceCapabilities(false),
        reloadSelectedModels(false),
        reloadModels(false)
      ])
    } finally {
      setPendingKey(null)
    }
  }

  useEffect(() => {
    void bootstrap()
  }, [])

  function updateSourceForm<K extends keyof SourceFormState>(field: K, value: SourceFormState[K]) {
    setSourceForm((current) => ({
      ...current,
      [field]: value
    }) as SourceFormState)
  }

  function resetSourceForm() {
    setEditingSourceID(null)
    setSourceForm(initialSourceForm)
  }

  function editSource(source: ModelSource) {
    setEditingSourceID(source.id)
    setSourceForm({
      name: source.name,
      base_url: source.base_url,
      provider_type: source.provider_type,
      default_model_id: source.default_model_id,
      exposed_model_ids_text: source.exposed_model_ids.join(", "),
      enabled: source.enabled,
      api_key: ""
    })
  }

  function moveSource(index: number, delta: number) {
    const nextIndex = index + delta
    if (nextIndex < 0 || nextIndex >= sources.length) {
      return
    }

    const next = sources.slice()
    const current = next[index]
    next[index] = next[nextIndex]
    next[nextIndex] = current
    setSources(next.map((source, position) => ({ ...source, position })))
    setSourceOrderDirty(true)
  }

  async function submitSource() {
    const payload: ModelSourceInput = {
      name: sourceForm.name.trim(),
      base_url: sourceForm.base_url.trim(),
      provider_type: sourceForm.provider_type,
      default_model_id: sourceForm.default_model_id.trim(),
      exposed_model_ids: parseModelCSV(sourceForm.exposed_model_ids_text),
      enabled: sourceForm.enabled,
      api_key: sourceForm.api_key.trim()
    }

    setPendingKey("source-submit")
    try {
      const path = editingSourceID ? `/admin/model-sources/${editingSourceID}` : "/admin/model-sources"
      const method = editingSourceID ? "PUT" : "POST"
      const result = await performRequest<ModelSource>(method, path, { body: payload })
      if (!result.ok) {
        return
      }
      resetSourceForm()
      await Promise.allSettled([reloadSources(false), reloadSourceCapabilities(false), reloadModels(false), reloadSelectedModels(false)])
    } finally {
      setPendingKey(null)
    }
  }

  async function deleteSource(source: ModelSource) {
    const confirmed = window.confirm(`删除 source "${source.name}"？`)
    if (!confirmed) {
      return
    }

    setPendingKey(`delete-${source.id}`)
    try {
      const result = await performRequest("DELETE", `/admin/model-sources/${source.id}`)
      if (!result.ok) {
        return
      }
      if (editingSourceID === source.id) {
        resetSourceForm()
      }
      await Promise.allSettled([reloadSources(false), reloadSourceCapabilities(false), reloadModels(false), reloadSelectedModels(false)])
    } finally {
      setPendingKey(null)
    }
  }

  async function saveSourceOrder() {
    setPendingKey("source-order")
    try {
      const payload = sources.map((source, position) => ({
        id: source.id,
        position
      }))
      const result = await performRequest("PUT", "/admin/model-sources/order", { body: payload })
      if (!result.ok) {
        return
      }
      await Promise.allSettled([reloadSources(false), reloadSourceCapabilities(false)])
    } finally {
      setPendingKey(null)
    }
  }

  function changeSelectedModel(index: number, modelID: string) {
    setSelectedModels((current) =>
      current.map((item, itemIndex) => (itemIndex === index ? { ...item, model_id: modelID } : item))
    )
  }

  function moveSelectedModel(index: number, delta: number) {
    const nextIndex = index + delta
    if (nextIndex < 0 || nextIndex >= selectedModels.length) {
      return
    }

    const next = selectedModels.slice()
    const current = next[index]
    next[index] = next[nextIndex]
    next[nextIndex] = current
    setSelectedModels(next)
  }

  function removeSelectedModel(index: number) {
    setSelectedModels((current) => current.filter((_, itemIndex) => itemIndex !== index))
  }

  function addSelectedModel() {
    if (models.length === 0) {
      return
    }
    setSelectedModels((current) => [...current, { model_id: models[0].id, position: current.length }])
  }

  async function saveSelectedModels() {
    setPendingKey("selected-save")
    try {
      const payload = selectedModels.map((item, position) => ({
        model_id: item.model_id,
        position
      }))
      const result = await performRequest("PUT", "/admin/selected-models", { body: payload })
      if (!result.ok) {
        return
      }
      await Promise.allSettled([reloadSelectedModels(false), reloadModels(false)])
    } finally {
      setPendingKey(null)
    }
  }

  async function runProbe(path: string) {
    setPendingKey(`probe-${path}`)
    try {
      if (path === "/health") {
        await reloadHealth(true)
        return
      }
      if (path === "/capabilities") {
        await reloadCapabilities(true)
        return
      }
      if (path === "/v1/models") {
        await reloadModels(true)
        return
      }
      if (path === "/admin/model-sources") {
        await reloadSources(true)
        return
      }
      if (path === "/admin/model-sources/capabilities") {
        await reloadSourceCapabilities(true)
        return
      }
      await reloadSelectedModels(true)
    } finally {
      setPendingKey(null)
    }
  }

  async function sendPlaygroundRequest(
    definition: PlaygroundDefinition,
    model: string,
    headersText: string,
    bodyText: string
  ) {
    let headers: Record<string, string>
    let body: Record<string, unknown>

    try {
      const parsedHeaders = headersText.trim() ? JSON.parse(headersText) : {}
      if (!isRecord(parsedHeaders)) {
        setResponse(createClientResult(definition.method, definition.path, "invalid_headers_json", "headers must be a JSON object"))
        return
      }
      headers = Object.fromEntries(Object.entries(parsedHeaders).map(([key, value]) => [key, String(value)]))
    } catch (error) {
      setResponse(createClientResult(definition.method, definition.path, "invalid_headers_json", String(error)))
      return
    }

    try {
      const parsedBody = bodyText.trim() ? JSON.parse(bodyText) : {}
      if (!isRecord(parsedBody)) {
        setResponse(createClientResult(definition.method, definition.path, "invalid_body_json", "body must be a JSON object"))
        return
      }
      body = parsedBody
    } catch (error) {
      setResponse(createClientResult(definition.method, definition.path, "invalid_body_json", String(error)))
      return
    }

    body.model = model

    setPendingKey(`playground-${definition.id}`)
    try {
      await performRequest(definition.method, definition.path, {
        headers,
        body
      })
    } finally {
      setPendingKey(null)
    }
  }

  const stats = [
    { label: "Model Sources", value: String(sources.length) },
    { label: "Selected Models", value: String(selectedModels.length) },
    { label: "Visible Models", value: String(models.length) }
  ]

  return (
    <div className="app-shell">
      <header className="hero-card">
        <div className="max-w-4xl">
          <p className="section-eyebrow text-rust-700">AI Mini Gateway</p>
          <h1 className="font-display text-4xl leading-none text-stone-950 sm:text-5xl lg:text-6xl">Standalone Web Console</h1>
          <p className="mt-5 max-w-3xl text-base leading-7 text-stone-600 sm:text-lg">
            这个管理端不再和 Go runtime 的静态资源耦合，目录独立、通过 API 调用，后续可以直接拆成一个单独的前端项目。
          </p>
        </div>

        <div className="mt-8 flex flex-wrap gap-3">
          <button type="button" className="btn-primary" onClick={() => void bootstrap()} disabled={pendingKey === "bootstrap"}>
            刷新全部
          </button>
          <a className="btn-secondary no-underline" href="#sources">
            管理 Model Sources
          </a>
          <a className="btn-secondary no-underline" href="#playground">
            打开 Playground
          </a>
        </div>
      </header>

      <div className="mt-6 grid gap-6 xl:grid-cols-[minmax(0,1.65fr)_minmax(360px,0.95fr)]">
        <div className="space-y-6">
          <SectionCard
            id="runtime"
            eyebrow="Runtime"
            title="运行态探测"
            description="快速确认当前 gateway 是否可用，并查看 capability、模型列表和管理接口响应。"
            actions={
              <>
                <button type="button" className="btn-secondary" onClick={() => void runProbe("/health")} disabled={pendingKey === "probe-/health"}>
                  GET /health
                </button>
                <button
                  type="button"
                  className="btn-secondary"
                  onClick={() => void runProbe("/capabilities")}
                  disabled={pendingKey === "probe-/capabilities"}
                >
                  GET /capabilities
                </button>
                <button type="button" className="btn-secondary" onClick={() => void runProbe("/v1/models")} disabled={pendingKey === "probe-/v1/models"}>
                  GET /v1/models
                </button>
              </>
            }
          >
            <div className="grid gap-4 lg:grid-cols-[minmax(0,1.1fr)_minmax(0,0.9fr)]">
              <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
                <div className="stat-card">
                  <span className="stat-label">Health</span>
                  <div className="mt-2">
                    <StatusBadge tone={health?.status === "ok" ? "success" : "warning"}>{health?.status || "unknown"}</StatusBadge>
                  </div>
                  <p className="mt-3 text-sm leading-6 text-stone-500">{health?.message || "当前未读取到 health 数据。"}</p>
                </div>
                {stats.map((item) => (
                  <div key={item.label} className="stat-card">
                    <span className="stat-label">{item.label}</span>
                    <strong className="mt-2 block text-3xl font-semibold text-stone-900">{item.value}</strong>
                  </div>
                ))}
              </div>

              <div className="rounded-[1.5rem] border border-stone-200/80 bg-white/80 p-5">
                <h3 className="font-display text-xl text-stone-900">Capabilities</h3>
                <div className="mt-4 flex flex-wrap gap-2">
                  {capabilityEntries(capabilities).length > 0 ? (
                    capabilityEntries(capabilities).map((entry) => (
                      <StatusBadge key={entry.key} tone={entry.value === "true" ? "success" : "danger"}>
                        {`${entry.key}: ${entry.value}`}
                      </StatusBadge>
                    ))
                  ) : (
                    <p className="text-sm leading-6 text-stone-500">尚未读取 capability 数据。</p>
                  )}
                </div>
              </div>
            </div>
          </SectionCard>

          <SectionCard
            id="sources"
            eyebrow="Admin API"
            title="Model Source 管理"
            description="支持创建、更新、删除和排序。排序保存会调用 `/admin/model-sources/order`。"
            actions={
              <>
                <button type="button" className="btn-secondary" onClick={() => void reloadSources(true)}>
                  刷新列表
                </button>
                <button
                  type="button"
                  className="btn-primary"
                  onClick={() => void saveSourceOrder()}
                  disabled={!sourceOrderDirty || pendingKey === "source-order"}
                >
                  保存排序
                </button>
              </>
            }
          >
            <div className="grid gap-6 xl:grid-cols-[minmax(320px,0.9fr)_minmax(0,1.1fr)]">
              <div className="rounded-[1.5rem] border border-stone-200/80 bg-white/85 p-5">
                <SourceForm
                  form={sourceForm}
                  isEditing={Boolean(editingSource)}
                  apiKeyNote={sourceKeyNote}
                  submitting={pendingKey === "source-submit"}
                  onFieldChange={updateSourceForm}
                  onSubmit={() => void submitSource()}
                  onReset={resetSourceForm}
                />
              </div>
              <SourceList sources={sources} onEdit={editSource} onDelete={(source) => void deleteSource(source)} onMove={moveSource} />
            </div>
          </SectionCard>

          <SectionCard
            eyebrow="Capability Matrix"
            title="各 Source 能力视图"
            description="这部分读取 `/admin/model-sources/capabilities`，用来观察 models/chat/responses/stream 的动态状态。"
            actions={
              <button type="button" className="btn-secondary" onClick={() => void reloadSourceCapabilities(true)}>
                刷新 capability
              </button>
            }
          >
            <SourceCapabilitiesTable rows={sourceCapabilities} />
          </SectionCard>

          <SectionCard
            id="selected-models"
            eyebrow="Selection"
            title="Selected Models"
            description="这里对应 `/admin/selected-models`，顺序就是 position 顺序。"
          >
            <SelectedModelsEditor
              items={selectedModels}
              models={models}
              onChangeModel={changeSelectedModel}
              onMove={moveSelectedModel}
              onRemove={removeSelectedModel}
              onAdd={addSelectedModel}
              onReload={() => void reloadSelectedModels(true)}
              onSave={() => void saveSelectedModels()}
              saving={pendingKey === "selected-save"}
            />
          </SectionCard>

          <SectionCard
            id="playground"
            eyebrow="Playground"
            title="OpenAI / Anthropic 请求调试"
            description="每张卡直接调用真实 API。模型选项来自 `/v1/models`，headers 和 body 都可以手动改。"
            actions={
              <button type="button" className="btn-secondary" onClick={() => void reloadModels(true)}>
                刷新模型
              </button>
            }
          >
            <div className="grid gap-5 2xl:grid-cols-2">
              {playgroundDefinitions.map((definition) => (
                <PlaygroundCard
                  key={definition.id}
                  definition={definition}
                  models={models}
                  pending={pendingKey === `playground-${definition.id}`}
                  onSend={(nextDefinition, model, headersText, bodyText) =>
                    void sendPlaygroundRequest(nextDefinition, model, headersText, bodyText)
                  }
                />
              ))}
            </div>
          </SectionCard>
        </div>

        <ResponsePanel response={response} />
      </div>
    </div>
  )
}

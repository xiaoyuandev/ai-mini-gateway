import { useEffect, useState } from "react"

import type { ModelItem, PlaygroundDefinition } from "../lib/types"

interface PlaygroundCardProps {
  definition: PlaygroundDefinition
  models: ModelItem[]
  pending: boolean
  onSend: (definition: PlaygroundDefinition, model: string, headersText: string, bodyText: string) => void
}

function prettyJSON(value: unknown) {
  return JSON.stringify(value, null, 2)
}

export function PlaygroundCard(props: PlaygroundCardProps) {
  const providerModels = props.models.filter((model) => model.owned_by === props.definition.provider)
  const availableModels = providerModels.length > 0 ? providerModels : props.models
  const availableModelIDs = availableModels.map((item) => item.id).join("|")
  const preferredModel = availableModels[0]?.id || ""

  const [model, setModel] = useState(preferredModel)
  const [headersText, setHeadersText] = useState(prettyJSON(props.definition.defaultHeaders))
  const [bodyText, setBodyText] = useState(prettyJSON(props.definition.buildPayload(preferredModel)))

  useEffect(() => {
    const nextModel = availableModels.some((item) => item.id === model) ? model : preferredModel
    setModel(nextModel)

    setBodyText((current) => {
      try {
        const parsed = JSON.parse(current) as Record<string, unknown>
        parsed.model = nextModel
        return prettyJSON(parsed)
      } catch {
        return prettyJSON(props.definition.buildPayload(nextModel))
      }
    })
  }, [availableModelIDs, preferredModel, props.definition])

  function handleReset() {
    setHeadersText(prettyJSON(props.definition.defaultHeaders))
    setBodyText(prettyJSON(props.definition.buildPayload(model)))
  }

  return (
    <article className="rounded-[1.75rem] border border-stone-200/80 bg-white/85 p-5 shadow-sm">
      <div className="mb-4">
        <h3 className="font-display text-xl text-stone-900">{props.definition.title}</h3>
        <p className="mt-1 text-sm text-stone-500">
          <code className="font-mono text-xs">{`${props.definition.method} ${props.definition.path}`}</code>
        </p>
      </div>

      <div className="space-y-4">
        <label className="field">
          <span>模型</span>
          <select
            className="input"
            value={model}
            onChange={(event) => {
              const nextModel = event.target.value
              setModel(nextModel)
              setBodyText((current) => {
                try {
                  const parsed = JSON.parse(current) as Record<string, unknown>
                  parsed.model = nextModel
                  return prettyJSON(parsed)
                } catch {
                  return prettyJSON(props.definition.buildPayload(nextModel))
                }
              })
            }}
          >
            {availableModels.length > 0 ? (
              availableModels.map((item) => (
                <option key={item.id} value={item.id}>
                  {item.id}
                </option>
              ))
            ) : (
              <option value="">暂无可用模型</option>
            )}
          </select>
        </label>

        <label className="field">
          <span>Headers JSON</span>
          <textarea className="textarea" value={headersText} onChange={(event) => setHeadersText(event.target.value)} />
        </label>

        <label className="field">
          <span>Body JSON</span>
          <textarea className="textarea min-h-[220px]" value={bodyText} onChange={(event) => setBodyText(event.target.value)} />
        </label>

        <div className="flex flex-wrap gap-3">
          <button
            type="button"
            className="btn-primary"
            onClick={() => props.onSend(props.definition, model, headersText, bodyText)}
            disabled={props.pending}
          >
            发送请求
          </button>
          <button type="button" className="btn-secondary" onClick={handleReset}>
            重置示例
          </button>
        </div>
      </div>
    </article>
  )
}

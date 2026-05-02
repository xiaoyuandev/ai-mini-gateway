import type { ModelSource } from "../lib/types"
import { StatusBadge } from "./StatusBadge"

interface SourceListProps {
  sources: ModelSource[]
  onEdit: (source: ModelSource) => void
  onDelete: (source: ModelSource) => void
  onMove: (index: number, delta: number) => void
}

export function SourceList(props: SourceListProps) {
  if (props.sources.length === 0) {
    return <div className="empty-state">还没有 model source。</div>
  }

  return (
    <div className="space-y-4">
      {props.sources.map((source, index) => (
        <article key={source.id} className="rounded-[1.5rem] border border-stone-200/80 bg-white/85 p-5 shadow-sm">
          <div className="mb-4 flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between">
            <div>
              <div className="mb-3 flex flex-wrap items-center gap-2">
                <h3 className="font-display text-xl text-stone-900">{source.name}</h3>
                <StatusBadge tone={source.enabled ? "success" : "danger"}>{source.enabled ? "enabled" : "disabled"}</StatusBadge>
                <StatusBadge>{source.provider_type}</StatusBadge>
                <StatusBadge>{`position ${source.position}`}</StatusBadge>
              </div>
              <div className="space-y-1 text-sm leading-6 text-stone-600">
                <p>
                  <strong className="text-stone-900">Base URL:</strong> {source.base_url}
                </p>
                <p>
                  <strong className="text-stone-900">Default Model:</strong> {source.default_model_id}
                </p>
                <p>
                  <strong className="text-stone-900">API Key:</strong> {source.api_key_masked || "未设置"}
                </p>
              </div>
            </div>

            <div className="flex flex-wrap gap-2">
              <button type="button" className="btn-secondary" onClick={() => props.onMove(index, -1)} disabled={index === 0}>
                上移
              </button>
              <button
                type="button"
                className="btn-secondary"
                onClick={() => props.onMove(index, 1)}
                disabled={index === props.sources.length - 1}
              >
                下移
              </button>
              <button type="button" className="btn-secondary" onClick={() => props.onEdit(source)}>
                编辑
              </button>
              <button type="button" className="btn-danger" onClick={() => props.onDelete(source)}>
                删除
              </button>
            </div>
          </div>

          <div className="flex flex-wrap gap-2">
            {source.exposed_model_ids.length > 0 ? (
              source.exposed_model_ids.map((model) => (
                <span key={model} className="rounded-full bg-stone-100 px-3 py-1 text-xs font-semibold text-stone-600">
                  {model}
                </span>
              ))
            ) : (
              <span className="rounded-full bg-stone-100 px-3 py-1 text-xs font-semibold text-stone-500">无 exposed models</span>
            )}
          </div>
        </article>
      ))}
    </div>
  )
}

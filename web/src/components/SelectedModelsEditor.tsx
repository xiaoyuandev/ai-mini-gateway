import type { ModelItem, SelectedModel } from "../lib/types"

interface SelectedModelsEditorProps {
  items: SelectedModel[]
  models: ModelItem[]
  onChangeModel: (index: number, modelID: string) => void
  onMove: (index: number, delta: number) => void
  onRemove: (index: number) => void
  onAdd: () => void
  onReload: () => void
  onSave: () => void
  saving: boolean
}

export function SelectedModelsEditor(props: SelectedModelsEditorProps) {
  const options = props.models.map((model) => (
    <option key={model.id} value={model.id}>
      {model.id} · {model.owned_by}
    </option>
  ))

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap gap-3">
        <button type="button" className="btn-secondary" onClick={props.onReload}>
          重新加载
        </button>
        <button type="button" className="btn-secondary" onClick={props.onAdd}>
          新增一行
        </button>
        <button type="button" className="btn-primary" onClick={props.onSave} disabled={props.saving}>
          保存 selected models
        </button>
      </div>

      {props.items.length === 0 ? (
        <div className="empty-state">当前没有 selected models。</div>
      ) : (
        <div className="space-y-4">
          {props.items.map((item, index) => (
            <article key={`${item.model_id}-${index}`} className="rounded-[1.5rem] border border-stone-200/80 bg-white/85 p-5">
              <div className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
                <div className="flex-1">
                  <p className="mb-2 text-sm font-semibold uppercase tracking-[0.16em] text-amber-700">Position {index}</p>
                  <select
                    className="input"
                    value={item.model_id}
                    onChange={(event) => props.onChangeModel(index, event.target.value)}
                  >
                    {options}
                  </select>
                </div>
                <div className="flex flex-wrap gap-2">
                  <button type="button" className="btn-secondary" onClick={() => props.onMove(index, -1)} disabled={index === 0}>
                    上移
                  </button>
                  <button
                    type="button"
                    className="btn-secondary"
                    onClick={() => props.onMove(index, 1)}
                    disabled={index === props.items.length - 1}
                  >
                    下移
                  </button>
                  <button type="button" className="btn-danger" onClick={() => props.onRemove(index)}>
                    删除
                  </button>
                </div>
              </div>
            </article>
          ))}
        </div>
      )}
    </div>
  )
}

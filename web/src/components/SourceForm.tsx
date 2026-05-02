import type { SourceFormState } from "../lib/types"

interface SourceFormProps {
  form: SourceFormState
  isEditing: boolean
  apiKeyNote: string
  submitting: boolean
  onFieldChange: <K extends keyof SourceFormState>(field: K, value: SourceFormState[K]) => void
  onSubmit: () => void
  onReset: () => void
}

export function SourceForm(props: SourceFormProps) {
  return (
    <div className="space-y-4">
      <div className="grid gap-4 md:grid-cols-2">
        <label className="field">
          <span>名称</span>
          <input className="input" value={props.form.name} onChange={(event) => props.onFieldChange("name", event.target.value)} placeholder="OpenAI US" />
        </label>
        <label className="field">
          <span>Provider Type</span>
          <select
            className="input"
            value={props.form.provider_type}
            onChange={(event) => props.onFieldChange("provider_type", event.target.value as SourceFormState["provider_type"])}
          >
            <option value="openai-compatible">openai-compatible</option>
            <option value="anthropic-compatible">anthropic-compatible</option>
          </select>
        </label>
        <label className="field">
          <span>Base URL</span>
          <input
            className="input"
            value={props.form.base_url}
            onChange={(event) => props.onFieldChange("base_url", event.target.value)}
            placeholder="https://api.openai.com/v1"
          />
        </label>
        <label className="field">
          <span>Default Model</span>
          <input
            className="input"
            value={props.form.default_model_id}
            onChange={(event) => props.onFieldChange("default_model_id", event.target.value)}
            placeholder="gpt-4.1-mini"
          />
        </label>
      </div>

      <label className="field">
        <span>Exposed Model IDs</span>
        <input
          className="input"
          value={props.form.exposed_model_ids_text}
          onChange={(event) => props.onFieldChange("exposed_model_ids_text", event.target.value)}
          placeholder="gpt-4.1, gpt-4.1-mini"
        />
      </label>

      <label className="field">
        <span>API Key</span>
        <input
          className="input"
          value={props.form.api_key}
          onChange={(event) => props.onFieldChange("api_key", event.target.value)}
          placeholder="sk-..."
          autoComplete="off"
        />
      </label>

      <label className="toggle-row">
        <input type="checkbox" checked={props.form.enabled} onChange={(event) => props.onFieldChange("enabled", event.target.checked)} />
        <span>启用当前 model source</span>
      </label>

      <p className="text-sm leading-6 text-stone-500">{props.apiKeyNote}</p>

      <div className="flex flex-wrap gap-3">
        <button type="button" className="btn-primary" onClick={props.onSubmit} disabled={props.submitting}>
          {props.isEditing ? "更新 source" : "创建 source"}
        </button>
        <button type="button" className="btn-secondary" onClick={props.onReset}>
          清空表单
        </button>
      </div>
    </div>
  )
}

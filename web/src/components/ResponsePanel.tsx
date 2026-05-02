import type { ApiResult } from "../lib/types"
import { StatusBadge } from "./StatusBadge"

interface ResponsePanelProps {
  response: ApiResult | null
}

function statusTone(status: number) {
  if (status >= 500) {
    return "danger" as const
  }
  if (status >= 400) {
    return "warning" as const
  }
  return "success" as const
}

export function ResponsePanel({ response }: ResponsePanelProps) {
  return (
    <div className="panel lg:sticky lg:top-6">
      <div className="mb-5 flex flex-wrap items-center justify-between gap-3">
        <div>
          <p className="section-eyebrow">Request Echo</p>
          <h2 className="section-title">最后一次响应</h2>
          <p className="section-copy">所有 probe、管理动作和 playground 请求都会在这里回显。</p>
        </div>
        {response ? <StatusBadge tone={statusTone(response.status)}>{`${response.status} ${response.statusText}`}</StatusBadge> : null}
      </div>

      <div className="mb-5 grid gap-3 sm:grid-cols-3">
        <div className="stat-card">
          <span className="stat-label">Method</span>
          <strong className="stat-value">{response?.method || "-"}</strong>
        </div>
        <div className="stat-card">
          <span className="stat-label">Path</span>
          <strong className="stat-value break-all">{response?.path || "-"}</strong>
        </div>
        <div className="stat-card">
          <span className="stat-label">Content Type</span>
          <strong className="stat-value break-all">{response?.contentType || "-"}</strong>
        </div>
      </div>

      <pre className="response-block">{response?.displayBody || "等待请求..."}</pre>
    </div>
  )
}

import type { ModelSourceCapabilityRow } from "../lib/types"
import { StatusBadge } from "./StatusBadge"

interface SourceCapabilitiesTableProps {
  rows: ModelSourceCapabilityRow[]
}

function badgeTone(supported: boolean, status: string) {
  if (!supported || status === "unsupported") {
    return "danger" as const
  }
  if (status.includes("configured")) {
    return "warning" as const
  }
  return "success" as const
}

export function SourceCapabilitiesTable(props: SourceCapabilitiesTableProps) {
  if (props.rows.length === 0) {
    return <div className="empty-state">还没有 capability 数据。</div>
  }

  return (
    <div className="overflow-hidden rounded-[1.5rem] border border-stone-200/80">
      <div className="overflow-x-auto">
        <table className="min-w-full divide-y divide-stone-200 bg-white/80 text-sm">
          <thead className="bg-stone-100/80 text-left text-stone-500">
            <tr>
              <th className="px-4 py-3 font-semibold">Source</th>
              <th className="px-4 py-3 font-semibold">Models API</th>
              <th className="px-4 py-3 font-semibold">OpenAI Chat</th>
              <th className="px-4 py-3 font-semibold">Responses</th>
              <th className="px-4 py-3 font-semibold">Anthropic Messages</th>
              <th className="px-4 py-3 font-semibold">Count Tokens</th>
              <th className="px-4 py-3 font-semibold">Stream</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-stone-200/70">
            {props.rows.map((row) => (
              <tr key={row.id}>
                <td className="px-4 py-4 align-top">
                  <div className="font-semibold text-stone-900">{row.name}</div>
                  <div className="mt-1 text-xs text-stone-500">{row.provider_type}</div>
                </td>
                <td className="px-4 py-4 align-top">
                  <StatusBadge tone={badgeTone(row.supports_models_api, row.models_api_status)}>{row.models_api_status}</StatusBadge>
                </td>
                <td className="px-4 py-4 align-top">
                  <StatusBadge tone={badgeTone(row.supports_openai_chat_completions, row.openai_chat_completions_status)}>
                    {row.openai_chat_completions_status}
                  </StatusBadge>
                </td>
                <td className="px-4 py-4 align-top">
                  <StatusBadge tone={badgeTone(row.supports_openai_responses, row.openai_responses_status)}>{row.openai_responses_status}</StatusBadge>
                </td>
                <td className="px-4 py-4 align-top">
                  <StatusBadge tone={badgeTone(row.supports_anthropic_messages, row.anthropic_messages_status)}>
                    {row.anthropic_messages_status}
                  </StatusBadge>
                </td>
                <td className="px-4 py-4 align-top">
                  <StatusBadge tone={badgeTone(row.supports_anthropic_count_tokens, row.anthropic_count_tokens_status)}>
                    {row.anthropic_count_tokens_status}
                  </StatusBadge>
                </td>
                <td className="px-4 py-4 align-top">
                  <StatusBadge tone={badgeTone(row.supports_stream, row.stream_status)}>{row.stream_status}</StatusBadge>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}

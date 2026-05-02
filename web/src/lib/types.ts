export type ProviderType = "openai-compatible" | "anthropic-compatible"

export interface GatewayHealth {
  status: string
  message?: string
}

export interface GatewayCapabilities {
  supports_openai_compatible: boolean
  supports_anthropic_compatible: boolean
  supports_models_api: boolean
  supports_stream: boolean
  supports_admin_api: boolean
  supports_model_source_admin: boolean
  supports_selected_model_admin: boolean
}

export interface ModelSource {
  id: string
  name: string
  base_url: string
  provider_type: ProviderType
  default_model_id: string
  exposed_model_ids: string[]
  enabled: boolean
  position: number
  api_key: string
  api_key_masked: string
}

export interface ModelSourceInput {
  name: string
  base_url: string
  provider_type: ProviderType
  default_model_id: string
  exposed_model_ids: string[]
  enabled: boolean
  api_key: string
}

export interface ModelSourceCapabilityRow {
  id: string
  name: string
  provider_type: ProviderType
  supports_models_api: boolean
  models_api_status: string
  supports_openai_chat_completions: boolean
  openai_chat_completions_status: string
  supports_openai_responses: boolean
  openai_responses_status: string
  supports_anthropic_messages: boolean
  anthropic_messages_status: string
  supports_anthropic_count_tokens: boolean
  anthropic_count_tokens_status: string
  supports_stream: boolean
  stream_status: string
}

export interface SelectedModel {
  model_id: string
  position: number
}

export interface ModelItem {
  id: string
  object: string
  owned_by: ProviderType
}

export interface ModelsResponse {
  data: ModelItem[]
}

export interface ApiResult<T = unknown> {
  ok: boolean
  status: number
  statusText: string
  contentType: string
  data: T | string | null
  rawText: string
  displayBody: string
  method: string
  path: string
}

export interface SourceFormState {
  name: string
  base_url: string
  provider_type: ProviderType
  default_model_id: string
  exposed_model_ids_text: string
  enabled: boolean
  api_key: string
}

export interface PlaygroundDefinition {
  id: string
  title: string
  method: "POST"
  path: string
  provider: ProviderType
  defaultHeaders: Record<string, string>
  buildPayload: (model: string) => Record<string, unknown>
}

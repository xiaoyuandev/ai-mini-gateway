package providers

import (
	"net/http"
	"strings"

	"github.com/yuanjunliang/ai-mini-gateway/internal/runtime/state"
)

type Capabilities struct {
	SupportsModelsAPI             bool   `json:"supports_models_api"`
	ModelsAPIStatus               string `json:"models_api_status"`
	SupportsOpenAIChatCompletions bool   `json:"supports_openai_chat_completions"`
	OpenAIChatCompletionsStatus   string `json:"openai_chat_completions_status"`
	SupportsOpenAIResponses       bool   `json:"supports_openai_responses"`
	OpenAIResponsesStatus         string `json:"openai_responses_status"`
	SupportsAnthropicMessages     bool   `json:"supports_anthropic_messages"`
	AnthropicMessagesStatus       string `json:"anthropic_messages_status"`
	SupportsAnthropicCountTokens  bool   `json:"supports_anthropic_count_tokens"`
	AnthropicCountTokensStatus    string `json:"anthropic_count_tokens_status"`
	SupportsStream                bool   `json:"supports_stream"`
	StreamStatus                  string `json:"stream_status"`
}

type Operation string

const (
	OperationModels                Operation = "models"
	OperationOpenAIChatCompletions Operation = "openai_chat_completions"
	OperationOpenAIResponses       Operation = "openai_responses"
	OperationAnthropicMessages     Operation = "anthropic_messages"
	OperationAnthropicCountTokens  Operation = "anthropic_count_tokens"
)

type Provider interface {
	ApplyAuthHeader(header http.Header, source state.ModelSource)
	DefaultCapabilities() Capabilities
	PathForOperation(operation Operation) string
	ShouldForwardHeader(key string) bool
	ValidateRequest(operation Operation, header http.Header) *ValidationError
	NormalizeUpstreamError(operation Operation, status int, payload map[string]any) ErrorResponse
	ClassifyResponse(operation Operation, resp *http.Response) ResponseClassification
	IsOperationUnsupported(operation Operation, status int) bool
}

type ValidationError struct {
	Status  int
	Code    string
	Message string
}

type ErrorResponse struct {
	Code    string
	Message string
}

type ResponseClassification struct {
	IsErrorJSON bool
	IsStream    bool
}

func ForSource(source state.ModelSource) Provider {
	switch source.ProviderType {
	case "anthropic-compatible":
		return anthropicProvider{}
	case "openai-compatible":
		fallthrough
	default:
		return openAIProvider{}
	}
}

func shouldForwardDefaultHeader(key string) bool {
	switch strings.ToLower(key) {
	case "host",
		"content-length",
		"authorization",
		"x-api-key",
		"connection",
		"keep-alive",
		"proxy-authenticate",
		"proxy-authorization",
		"te",
		"trailer",
		"transfer-encoding",
		"upgrade":
		return false
	default:
		return true
	}
}

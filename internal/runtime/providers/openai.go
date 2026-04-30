package providers

import (
	"net/http"
	"strings"

	"github.com/yuanjunliang/ai-mini-gateway/internal/runtime/state"
)

type openAIProvider struct{}

func (openAIProvider) ApplyAuthHeader(header http.Header, source state.ModelSource) {
	if source.APIKey == "" || header.Get("Authorization") != "" {
		return
	}
	header.Set("Authorization", "Bearer "+source.APIKey)
}

func (openAIProvider) DefaultCapabilities() Capabilities {
	return Capabilities{
		SupportsModelsAPI:             true,
		ModelsAPIStatus:               "unknown",
		SupportsOpenAIChatCompletions: true,
		OpenAIChatCompletionsStatus:   "configured_supported",
		SupportsOpenAIResponses:       true,
		OpenAIResponsesStatus:         "configured_supported",
		SupportsAnthropicMessages:     false,
		AnthropicMessagesStatus:       "unsupported",
		SupportsAnthropicCountTokens:  false,
		AnthropicCountTokensStatus:    "unsupported",
		SupportsStream:                true,
		StreamStatus:                  "configured_supported",
	}
}

func (openAIProvider) PathForOperation(operation Operation) string {
	switch operation {
	case OperationModels:
		return "/models"
	case OperationOpenAIResponses:
		return "/responses"
	case OperationOpenAIChatCompletions:
		fallthrough
	default:
		return "/chat/completions"
	}
}

func (openAIProvider) ShouldForwardHeader(key string) bool {
	return shouldForwardDefaultHeader(key)
}

func (openAIProvider) ValidateRequest(operation Operation, header http.Header) *ValidationError {
	return nil
}

func (openAIProvider) NormalizeUpstreamError(operation Operation, status int, payload map[string]any) ErrorResponse {
	code := "openai_upstream_request_failed"
	switch operation {
	case OperationOpenAIResponses:
		code = "openai_responses_upstream_request_failed"
	case OperationOpenAIChatCompletions:
		code = "openai_chat_completions_upstream_request_failed"
	case OperationModels:
		code = "openai_models_upstream_request_failed"
	}

	message := http.StatusText(status)
	if value, ok := payload["error"].(string); ok && strings.TrimSpace(value) != "" {
		code = value
	}
	if value, ok := payload["message"].(string); ok && strings.TrimSpace(value) != "" {
		message = value
	}
	return ErrorResponse{Code: code, Message: message}
}

func (openAIProvider) ClassifyResponse(operation Operation, resp *http.Response) ResponseClassification {
	contentType := resp.Header.Get("Content-Type")
	return ResponseClassification{
		IsErrorJSON: resp.StatusCode >= 400 && strings.Contains(contentType, "application/json"),
		IsStream:    strings.Contains(contentType, "text/event-stream"),
	}
}

func (openAIProvider) IsOperationUnsupported(operation Operation, status int) bool {
	switch status {
	case http.StatusNotFound, http.StatusMethodNotAllowed, http.StatusNotImplemented:
		return true
	default:
		return false
	}
}

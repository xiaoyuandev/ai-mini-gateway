package providers

import (
	"net/http"
	"strings"

	"github.com/yuanjunliang/ai-mini-gateway/internal/runtime/state"
)

type anthropicProvider struct{}

func (anthropicProvider) ApplyAuthHeader(header http.Header, source state.ModelSource) {
	if source.APIKey == "" || header.Get("x-api-key") != "" {
		return
	}
	header.Set("x-api-key", source.APIKey)
}

func (anthropicProvider) DefaultCapabilities() Capabilities {
	return Capabilities{
		SupportsModelsAPI:             true,
		ModelsAPIStatus:               "unknown",
		SupportsOpenAIChatCompletions: false,
		OpenAIChatCompletionsStatus:   "unsupported",
		SupportsOpenAIResponses:       false,
		OpenAIResponsesStatus:         "unsupported",
		SupportsAnthropicMessages:     true,
		AnthropicMessagesStatus:       "configured_supported",
		SupportsAnthropicCountTokens:  true,
		AnthropicCountTokensStatus:    "configured_supported",
		SupportsStream:                true,
		StreamStatus:                  "configured_supported",
	}
}

func (anthropicProvider) PathForOperation(operation Operation) string {
	switch operation {
	case OperationModels:
		return "/models"
	case OperationAnthropicCountTokens:
		return "/messages/count_tokens"
	case OperationAnthropicMessages:
		fallthrough
	default:
		return "/messages"
	}
}

func (anthropicProvider) ShouldForwardHeader(key string) bool {
	switch strings.ToLower(key) {
	case "host", "content-length":
		return false
	default:
		return true
	}
}

func (anthropicProvider) ValidateRequest(operation Operation, header http.Header) *ValidationError {
	switch operation {
	case OperationAnthropicMessages, OperationAnthropicCountTokens:
		if header.Get("anthropic-version") == "" {
			return &ValidationError{
				Status:  http.StatusBadRequest,
				Code:    "missing_header",
				Message: "anthropic-version header is required",
			}
		}
	}
	return nil
}

func (anthropicProvider) NormalizeUpstreamError(operation Operation, status int, payload map[string]any) ErrorResponse {
	code := "anthropic_upstream_request_failed"
	switch operation {
	case OperationAnthropicMessages:
		code = "anthropic_messages_upstream_request_failed"
	case OperationAnthropicCountTokens:
		code = "anthropic_count_tokens_upstream_request_failed"
	case OperationModels:
		code = "anthropic_models_upstream_request_failed"
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

func (anthropicProvider) ClassifyResponse(operation Operation, resp *http.Response) ResponseClassification {
	contentType := resp.Header.Get("Content-Type")
	return ResponseClassification{
		IsErrorJSON: resp.StatusCode >= 400 && strings.Contains(contentType, "application/json"),
		IsStream:    strings.Contains(contentType, "text/event-stream"),
	}
}

func (anthropicProvider) IsOperationUnsupported(operation Operation, status int) bool {
	switch status {
	case http.StatusNotFound, http.StatusMethodNotAllowed, http.StatusNotImplemented:
		return true
	default:
		return false
	}
}

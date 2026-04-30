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
	switch strings.ToLower(key) {
	case "host", "content-length":
		return false
	default:
		return true
	}
}

func (openAIProvider) ValidateRequest(operation Operation, header http.Header) *ValidationError {
	return nil
}

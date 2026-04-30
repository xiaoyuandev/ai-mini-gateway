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

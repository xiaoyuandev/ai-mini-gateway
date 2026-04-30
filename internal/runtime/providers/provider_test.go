package providers

import (
	"net/http"
	"testing"

	"github.com/yuanjunliang/ai-mini-gateway/internal/runtime/state"
)

func TestForSource(t *testing.T) {
	tests := []struct {
		name         string
		providerType string
		wantType     string
	}{
		{name: "openai", providerType: "openai-compatible", wantType: "providers.openAIProvider"},
		{name: "anthropic", providerType: "anthropic-compatible", wantType: "providers.anthropicProvider"},
		{name: "fallback", providerType: "unknown", wantType: "providers.openAIProvider"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ForSource(state.ModelSource{ProviderType: tt.providerType})
			if actual := typeName(got); actual != tt.wantType {
				t.Fatalf("unexpected provider type: %s", actual)
			}
		})
	}
}

func TestOpenAIProvider(t *testing.T) {
	provider := openAIProvider{}
	source := state.ModelSource{APIKey: "sk-openai"}

	t.Run("apply auth header", func(t *testing.T) {
		header := http.Header{}
		provider.ApplyAuthHeader(header, source)
		if got := header.Get("Authorization"); got != "Bearer sk-openai" {
			t.Fatalf("unexpected authorization header: %q", got)
		}
	})

	t.Run("preserve existing auth header", func(t *testing.T) {
		header := http.Header{}
		header.Set("Authorization", "Bearer existing")
		provider.ApplyAuthHeader(header, source)
		if got := header.Get("Authorization"); got != "Bearer existing" {
			t.Fatalf("unexpected authorization header: %q", got)
		}
	})

	t.Run("paths", func(t *testing.T) {
		if got := provider.PathForOperation(OperationModels); got != "/models" {
			t.Fatalf("unexpected models path: %q", got)
		}
		if got := provider.PathForOperation(OperationOpenAIChatCompletions); got != "/chat/completions" {
			t.Fatalf("unexpected chat path: %q", got)
		}
		if got := provider.PathForOperation(OperationOpenAIResponses); got != "/responses" {
			t.Fatalf("unexpected responses path: %q", got)
		}
	})

	t.Run("header forwarding", func(t *testing.T) {
		if provider.ShouldForwardHeader("Host") {
			t.Fatal("host should not be forwarded")
		}
		if provider.ShouldForwardHeader("Content-Length") {
			t.Fatal("content-length should not be forwarded")
		}
		if provider.ShouldForwardHeader("Authorization") {
			t.Fatal("authorization should not be forwarded")
		}
		if provider.ShouldForwardHeader("Connection") {
			t.Fatal("connection should not be forwarded")
		}
		if !provider.ShouldForwardHeader("x-trace-id") {
			t.Fatal("custom headers should be forwarded")
		}
	})

	t.Run("capabilities", func(t *testing.T) {
		caps := provider.DefaultCapabilities()
		if !caps.SupportsOpenAIChatCompletions || !caps.SupportsOpenAIResponses {
			t.Fatalf("unexpected openai capabilities: %+v", caps)
		}
		if caps.SupportsAnthropicMessages || caps.SupportsAnthropicCountTokens {
			t.Fatalf("unexpected anthropic capabilities: %+v", caps)
		}
		if !caps.SupportsStream || caps.StreamStatus != "configured_supported" {
			t.Fatalf("unexpected stream capabilities: %+v", caps)
		}
	})

	t.Run("validate request", func(t *testing.T) {
		if err := provider.ValidateRequest(OperationOpenAIChatCompletions, http.Header{}); err != nil {
			t.Fatalf("unexpected validation error: %+v", err)
		}
	})

	t.Run("normalize upstream error", func(t *testing.T) {
		got := provider.NormalizeUpstreamError(OperationOpenAIResponses, http.StatusBadGateway, map[string]any{})
		if got.Code != "openai_responses_upstream_request_failed" {
			t.Fatalf("unexpected code: %+v", got)
		}
		if got.Message != http.StatusText(http.StatusBadGateway) {
			t.Fatalf("unexpected message: %+v", got)
		}
	})

	t.Run("classify response", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		}
		classification := provider.ClassifyResponse(OperationOpenAIChatCompletions, resp)
		if !classification.IsStream || classification.IsErrorJSON {
			t.Fatalf("unexpected classification: %+v", classification)
		}
	})

	t.Run("unsupported status", func(t *testing.T) {
		if !provider.IsOperationUnsupported(OperationOpenAIChatCompletions, http.StatusNotFound) {
			t.Fatal("expected 404 to be unsupported")
		}
		if provider.IsOperationUnsupported(OperationOpenAIChatCompletions, http.StatusBadGateway) {
			t.Fatal("expected 502 not to be unsupported")
		}
	})
}

func TestAnthropicProvider(t *testing.T) {
	provider := anthropicProvider{}
	source := state.ModelSource{APIKey: "sk-anthropic"}

	t.Run("apply auth header", func(t *testing.T) {
		header := http.Header{}
		provider.ApplyAuthHeader(header, source)
		if got := header.Get("x-api-key"); got != "sk-anthropic" {
			t.Fatalf("unexpected x-api-key header: %q", got)
		}
	})

	t.Run("preserve existing api key header", func(t *testing.T) {
		header := http.Header{}
		header.Set("x-api-key", "existing")
		provider.ApplyAuthHeader(header, source)
		if got := header.Get("x-api-key"); got != "existing" {
			t.Fatalf("unexpected x-api-key header: %q", got)
		}
	})

	t.Run("paths", func(t *testing.T) {
		if got := provider.PathForOperation(OperationModels); got != "/models" {
			t.Fatalf("unexpected models path: %q", got)
		}
		if got := provider.PathForOperation(OperationAnthropicMessages); got != "/messages" {
			t.Fatalf("unexpected messages path: %q", got)
		}
		if got := provider.PathForOperation(OperationAnthropicCountTokens); got != "/messages/count_tokens" {
			t.Fatalf("unexpected count tokens path: %q", got)
		}
	})

	t.Run("header forwarding", func(t *testing.T) {
		if provider.ShouldForwardHeader("Host") {
			t.Fatal("host should not be forwarded")
		}
		if provider.ShouldForwardHeader("Content-Length") {
			t.Fatal("content-length should not be forwarded")
		}
		if provider.ShouldForwardHeader("x-api-key") {
			t.Fatal("x-api-key should not be forwarded")
		}
		if provider.ShouldForwardHeader("Transfer-Encoding") {
			t.Fatal("transfer-encoding should not be forwarded")
		}
		if !provider.ShouldForwardHeader("x-request-id") {
			t.Fatal("custom headers should be forwarded")
		}
	})

	t.Run("capabilities", func(t *testing.T) {
		caps := provider.DefaultCapabilities()
		if !caps.SupportsAnthropicMessages || !caps.SupportsAnthropicCountTokens {
			t.Fatalf("unexpected anthropic capabilities: %+v", caps)
		}
		if caps.SupportsOpenAIChatCompletions || caps.SupportsOpenAIResponses {
			t.Fatalf("unexpected openai capabilities: %+v", caps)
		}
		if !caps.SupportsStream || caps.StreamStatus != "configured_supported" {
			t.Fatalf("unexpected stream capabilities: %+v", caps)
		}
	})

	t.Run("validate request", func(t *testing.T) {
		if err := provider.ValidateRequest(OperationAnthropicMessages, http.Header{}); err == nil {
			t.Fatal("expected missing anthropic-version validation error")
		}

		header := http.Header{}
		header.Set("anthropic-version", "2023-06-01")
		if err := provider.ValidateRequest(OperationAnthropicMessages, header); err != nil {
			t.Fatalf("unexpected validation error: %+v", err)
		}
		if err := provider.ValidateRequest(OperationAnthropicCountTokens, header); err != nil {
			t.Fatalf("unexpected validation error: %+v", err)
		}
	})

	t.Run("normalize upstream error", func(t *testing.T) {
		got := provider.NormalizeUpstreamError(OperationAnthropicMessages, http.StatusTooManyRequests, map[string]any{
			"error":   "rate_limited",
			"message": "slow down",
		})
		if got.Code != "rate_limited" || got.Message != "slow down" {
			t.Fatalf("unexpected error normalization: %+v", got)
		}
	})

	t.Run("classify response", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: http.StatusBadRequest,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}
		classification := provider.ClassifyResponse(OperationAnthropicMessages, resp)
		if classification.IsStream || !classification.IsErrorJSON {
			t.Fatalf("unexpected classification: %+v", classification)
		}
	})

	t.Run("unsupported status", func(t *testing.T) {
		if !provider.IsOperationUnsupported(OperationAnthropicMessages, http.StatusMethodNotAllowed) {
			t.Fatal("expected 405 to be unsupported")
		}
		if provider.IsOperationUnsupported(OperationAnthropicMessages, http.StatusTooManyRequests) {
			t.Fatal("expected 429 not to be unsupported")
		}
	})
}

func typeName(v any) string {
	return "providers." + map[bool]string{
		true:  "anthropicProvider",
		false: "openAIProvider",
	}[isAnthropic(v)]
}

func isAnthropic(v any) bool {
	_, ok := v.(anthropicProvider)
	return ok
}

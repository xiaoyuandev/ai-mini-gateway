package executor

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/yuanjunliang/ai-mini-gateway/internal/runtime/providers"
	"github.com/yuanjunliang/ai-mini-gateway/internal/runtime/state"
)

func TestProxyHealthcheckRejectsInvalidModelsPayload(t *testing.T) {
	source := state.ModelSource{
		ID:             "src_openai_invalid",
		BaseURL:        "https://openai.example/v1",
		ProviderType:   "openai-compatible",
		DefaultModelID: "gpt-4.1",
		Enabled:        true,
		APIKey:         "sk-openai",
	}

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			rec := httptest.NewRecorder()
			rec.WriteHeader(http.StatusOK)
			_, _ = rec.WriteString("not-json")
			return rec.Result(), nil
		}),
	}

	proxy := NewProxyWithClientAndTTL(client, 30*time.Second)
	result := proxy.HealthcheckSource(context.Background(), source)

	if result.Status != "error" {
		t.Fatalf("expected error status, got %+v", result)
	}
	if result.StatusCode != http.StatusOK {
		t.Fatalf("expected status code 200 for invalid payload, got %+v", result)
	}
	if result.Summary == "" {
		t.Fatalf("expected summary to explain failure, got %+v", result)
	}
}

func TestProxyConcurrentFetchModels(t *testing.T) {
	source := state.ModelSource{
		ID:             "src_openai",
		BaseURL:        "https://openai.example/v1",
		ProviderType:   "openai-compatible",
		DefaultModelID: "gpt-4.1",
		Enabled:        true,
		APIKey:         "sk-openai",
	}

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			rec := httptest.NewRecorder()
			_ = json.NewEncoder(rec).Encode(map[string]any{
				"data": []map[string]any{
					{"id": "gpt-4.1", "object": "model", "owned_by": "openai-compatible"},
				},
			})
			return rec.Result(), nil
		}),
	}

	proxy := NewProxyWithClientAndTTL(client, 30*time.Second)

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			models, err := proxy.FetchModels(context.Background(), source)
			if err != nil {
				t.Errorf("fetch models: %v", err)
				return
			}
			if len(models) != 1 || models[0].ID != "gpt-4.1" {
				t.Errorf("unexpected models: %+v", models)
			}
		}()
	}
	wg.Wait()

	caps := proxy.GetSourceCapabilities(source)
	if caps.ModelsAPIStatus != "supported" {
		t.Fatalf("unexpected capability status: %+v", caps)
	}
}

func TestProxyConcurrentObservedStatuses(t *testing.T) {
	source := state.ModelSource{
		ID:             "src_anthropic",
		BaseURL:        "https://anthropic.example/v1",
		ProviderType:   "anthropic-compatible",
		DefaultModelID: "claude-3-7-sonnet",
		Enabled:        true,
		APIKey:         "sk-anthropic",
	}

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			rec := httptest.NewRecorder()
			rec.Header().Set("Content-Type", "text/event-stream")
			rec.WriteHeader(http.StatusOK)
			return rec.Result(), nil
		}),
	}

	proxy := NewProxyWithClientAndTTL(client, 30*time.Second)

	body := []byte(`{"stream":true}`)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := proxy.ForwardOperation(context.Background(), source, providers.OperationAnthropicMessages, http.Header{
				"anthropic-version": []string{"2023-06-01"},
			}, body)
			if err != nil {
				t.Errorf("forward operation: %v", err)
				return
			}
			resp.Body.Close()
		}()
	}
	wg.Wait()

	caps := proxy.GetSourceCapabilities(source)
	if caps.AnthropicMessagesStatus != "supported" || caps.StreamStatus != "supported" {
		t.Fatalf("unexpected observed capabilities: %+v", caps)
	}
}

func TestUpstreamCandidateURLs(t *testing.T) {
	testCases := []struct {
		name    string
		baseURL string
		path    string
		want    []string
	}{
		{
			name:    "anthropic nested root prefers versioned path",
			baseURL: "https://api.minimaxi.com/anthropic",
			path:    "/messages",
			want: []string{
				"https://api.minimaxi.com/anthropic/v1/messages",
				"https://api.minimaxi.com/anthropic/messages",
			},
		},
		{
			name:    "existing version path is preserved",
			baseURL: "https://api.deepseek.com/anthropic/v1",
			path:    "/messages",
			want: []string{
				"https://api.deepseek.com/anthropic/v1/messages",
			},
		},
		{
			name:    "root base url gets versioned then legacy candidate",
			baseURL: "https://openrouter.example",
			path:    "/models",
			want: []string{
				"https://openrouter.example/v1/models",
				"https://openrouter.example/models",
			},
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			got := upstreamCandidateURLs(testCase.baseURL, testCase.path)
			if !slices.Equal(got, testCase.want) {
				t.Fatalf("upstreamCandidateURLs(%q, %q) = %#v, want %#v", testCase.baseURL, testCase.path, got, testCase.want)
			}
		})
	}
}

func TestProxyFetchModelsFallsBackToLegacyPath(t *testing.T) {
	source := state.ModelSource{
		ID:             "src_models_fallback",
		BaseURL:        "https://legacy-models.example",
		ProviderType:   "openai-compatible",
		DefaultModelID: "legacy-model",
		Enabled:        true,
		APIKey:         "sk-legacy",
	}

	var hits []string
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			hits = append(hits, req.URL.String())
			rec := httptest.NewRecorder()
			switch req.URL.Path {
			case "/v1/models":
				rec.WriteHeader(http.StatusNotFound)
			case "/models":
				_ = json.NewEncoder(rec).Encode(map[string]any{
					"data": []map[string]any{
						{"id": "legacy-model", "object": "model", "owned_by": "openai-compatible"},
					},
				})
			default:
				t.Fatalf("unexpected path: %s", req.URL.Path)
			}
			return rec.Result(), nil
		}),
	}

	proxy := NewProxyWithClientAndTTL(client, 30*time.Second)
	models, err := proxy.FetchModels(context.Background(), source)
	if err != nil {
		t.Fatalf("fetch models: %v", err)
	}

	if len(models) != 1 || models[0].ID != "legacy-model" {
		t.Fatalf("unexpected models: %+v", models)
	}
	if !slices.Equal(hits, []string{
		"https://legacy-models.example/v1/models",
		"https://legacy-models.example/models",
	}) {
		t.Fatalf("unexpected request sequence: %#v", hits)
	}
}

func TestProxyForwardAnthropicPrefersVersionedNestedRoot(t *testing.T) {
	source := state.ModelSource{
		ID:             "src_anthropic_nested",
		BaseURL:        "https://api.minimaxi.com/anthropic",
		ProviderType:   "anthropic-compatible",
		DefaultModelID: "MiniMax-M2.7",
		Enabled:        true,
		APIKey:         "sk-minimax",
	}

	var hits []string
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			hits = append(hits, req.URL.String())
			rec := httptest.NewRecorder()
			if req.URL.Path != "/anthropic/v1/messages" {
				t.Fatalf("unexpected path: %s", req.URL.Path)
			}
			rec.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(rec).Encode(map[string]any{"id": "msg_1"})
			return rec.Result(), nil
		}),
	}

	proxy := NewProxyWithClientAndTTL(client, 30*time.Second)
	resp, err := proxy.ForwardOperation(
		context.Background(),
		source,
		providers.OperationAnthropicMessages,
		http.Header{"anthropic-version": []string{"2023-06-01"}},
		[]byte(`{"model":"MiniMax-M2.7","messages":[{"role":"user","content":"hello"}]}`),
	)
	if err != nil {
		t.Fatalf("forward operation: %v", err)
	}
	defer resp.Body.Close()

	if !slices.Equal(hits, []string{"https://api.minimaxi.com/anthropic/v1/messages"}) {
		t.Fatalf("unexpected request sequence: %#v", hits)
	}
}

func TestProxyHealthcheckAnthropicFallsBackToMessagesBeforeCountTokens(t *testing.T) {
	source := state.ModelSource{
		ID:             "src_anthropic_healthcheck",
		BaseURL:        "https://api.deepseek.com/anthropic",
		ProviderType:   "anthropic-compatible",
		DefaultModelID: "deepseek-v4-flash",
		Enabled:        true,
		APIKey:         "sk-deepseek",
	}

	var hits []string
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			hits = append(hits, req.URL.String())
			rec := httptest.NewRecorder()
			switch req.URL.Path {
			case "/anthropic/v1/models":
				rec.WriteHeader(http.StatusNotFound)
			case "/anthropic/models":
				rec.WriteHeader(http.StatusNotFound)
			case "/anthropic/v1/messages":
				rec.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(rec).Encode(map[string]any{"id": "msg_healthcheck"})
			default:
				t.Fatalf("unexpected path: %s", req.URL.Path)
			}
			return rec.Result(), nil
		}),
	}

	proxy := NewProxyWithClientAndTTL(client, 30*time.Second)
	result := proxy.HealthcheckSource(context.Background(), source)

	if result.Status != "ok" || result.StatusCode != http.StatusOK {
		t.Fatalf("unexpected healthcheck result: %+v", result)
	}
	if !slices.Equal(hits, []string{
		"https://api.deepseek.com/anthropic/v1/models",
		"https://api.deepseek.com/anthropic/models",
		"https://api.deepseek.com/anthropic/v1/messages",
	}) {
		t.Fatalf("unexpected request sequence: %#v", hits)
	}
}

func TestProxyHealthcheckAnthropicUsesCountTokensAsLastFallback(t *testing.T) {
	source := state.ModelSource{
		ID:             "src_anthropic_count_tokens_fallback",
		BaseURL:        "https://anthropic-fallback.example/anthropic",
		ProviderType:   "anthropic-compatible",
		DefaultModelID: "claude-sonnet",
		Enabled:        true,
		APIKey:         "sk-anthropic",
	}

	var hits []string
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			hits = append(hits, req.URL.String())
			rec := httptest.NewRecorder()
			switch req.URL.Path {
			case "/anthropic/v1/models", "/anthropic/models":
				rec.WriteHeader(http.StatusNotFound)
			case "/anthropic/v1/messages":
				rec.WriteHeader(http.StatusMethodNotAllowed)
			case "/anthropic/messages":
				rec.WriteHeader(http.StatusMethodNotAllowed)
			case "/anthropic/v1/messages/count_tokens":
				_ = json.NewEncoder(rec).Encode(map[string]any{"input_tokens": 3})
			default:
				t.Fatalf("unexpected path: %s", req.URL.Path)
			}
			return rec.Result(), nil
		}),
	}

	proxy := NewProxyWithClientAndTTL(client, 30*time.Second)
	result := proxy.HealthcheckSource(context.Background(), source)

	if result.Status != "ok" || result.StatusCode != http.StatusOK {
		t.Fatalf("unexpected healthcheck result: %+v", result)
	}
	if !slices.Equal(hits, []string{
		"https://anthropic-fallback.example/anthropic/v1/models",
		"https://anthropic-fallback.example/anthropic/models",
		"https://anthropic-fallback.example/anthropic/v1/messages",
		"https://anthropic-fallback.example/anthropic/messages",
		"https://anthropic-fallback.example/anthropic/v1/messages/count_tokens",
	}) {
		t.Fatalf("unexpected request sequence: %#v", hits)
	}
}

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

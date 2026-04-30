package executor

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/yuanjunliang/ai-mini-gateway/internal/runtime/providers"
	"github.com/yuanjunliang/ai-mini-gateway/internal/runtime/state"
)

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

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

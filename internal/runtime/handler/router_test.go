package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yuanjunliang/ai-mini-gateway/internal/runtime/executor"
	"github.com/yuanjunliang/ai-mini-gateway/internal/runtime/state"
)

func TestRuntimeContract(t *testing.T) {
	dir := t.TempDir()
	store, err := state.NewStore(dir)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	_, err = store.CreateModelSource(t.Context(), state.ModelSourceUpsertRequest{
		Name:           "OpenAI",
		BaseURL:        "https://openai.example/v1",
		ProviderType:   "openai-compatible",
		DefaultModelID: "gpt-4.1",
		Enabled:        true,
		APIKey:         "sk-test",
	})
	if err != nil {
		t.Fatalf("create model source: %v", err)
	}
	_, err = store.CreateModelSource(t.Context(), state.ModelSourceUpsertRequest{
		Name:           "Anthropic",
		BaseURL:        "https://anthropic.example/v1",
		ProviderType:   "anthropic-compatible",
		DefaultModelID: "claude-3-7-sonnet",
		Enabled:        true,
		APIKey:         "sk-ant-test",
	})
	if err != nil {
		t.Fatalf("create anthropic source: %v", err)
	}

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			rec := httptest.NewRecorder()

			switch req.URL.String() {
			case "https://openai.example/v1/chat/completions":
				_ = json.NewEncoder(rec).Encode(map[string]any{"id": "chatcmpl-test", "object": "chat.completion"})
			case "https://openai.example/v1/responses":
				_ = json.NewEncoder(rec).Encode(map[string]any{"id": "resp-test", "object": "response"})
			case "https://anthropic.example/v1/messages":
				_ = json.NewEncoder(rec).Encode(map[string]any{"id": "msg_test", "type": "message"})
			case "https://anthropic.example/v1/messages/count_tokens":
				_ = json.NewEncoder(rec).Encode(map[string]any{"input_tokens": 3})
			default:
				rec.WriteHeader(http.StatusNotFound)
			}

			return rec.Result(), nil
		}),
	}

	router := NewRouterWithProxy(store, executor.NewProxyWithClient(client))

	t.Run("health", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d", rec.Code)
		}
	})

	t.Run("models", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d", rec.Code)
		}
	})

	t.Run("chat completions", func(t *testing.T) {
		body := []byte(`{"model":"gpt-4.1","messages":[{"role":"user","content":"hello"}]}`)
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("anthropic", func(t *testing.T) {
		body := []byte(`{"model":"claude-3-7-sonnet","messages":[{"role":"user","content":"hello"}],"max_tokens":128}`)
		req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
		req.Header.Set("anthropic-version", "2023-06-01")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("anthropic count tokens", func(t *testing.T) {
		body := []byte(`{"model":"claude-3-7-sonnet","messages":[{"role":"user","content":"hello"}]}`)
		req := httptest.NewRequest(http.MethodPost, "/v1/messages/count_tokens", bytes.NewReader(body))
		req.Header.Set("anthropic-version", "2023-06-01")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
		}
	})
}

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body != nil {
		_, _ = io.ReadAll(req.Body)
	}
	return f(req)
}

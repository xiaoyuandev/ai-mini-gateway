package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

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
		BaseURL:        "https://api.openai.com/v1",
		ProviderType:   "openai-compatible",
		DefaultModelID: "gpt-4.1",
		Enabled:        true,
		APIKey:         "sk-test",
	})
	if err != nil {
		t.Fatalf("create model source: %v", err)
	}

	router := NewRouter(store)

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
		body := []byte(`{"model":"gpt-4.1","messages":[{"role":"user","content":"hello"}],"max_tokens":128}`)
		req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
		req.Header.Set("anthropic-version", "2023-06-01")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
		}
	})
}

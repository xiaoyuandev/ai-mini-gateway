package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yuanjunliang/ai-mini-gateway/internal/runtime/buildinfo"
	"github.com/yuanjunliang/ai-mini-gateway/internal/runtime/executor"
	"github.com/yuanjunliang/ai-mini-gateway/internal/runtime/state"
)

func TestRuntimeContract(t *testing.T) {
	modelsHits := map[string]int{}

	dir := t.TempDir()
	store, err := state.NewStore(dir)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	_, err = store.CreateModelSource(t.Context(), state.ModelSourceUpsertRequest{
		Name:            "OpenAI",
		BaseURL:         "https://openai.example/v1",
		ProviderType:    "openai-compatible",
		DefaultModelID:  "gpt-4.1",
		ExposedModelIDs: []string{"gpt-4.1-mini", "gpt-upstream-error"},
		Enabled:         true,
		APIKey:          "sk-test",
	})
	if err != nil {
		t.Fatalf("create model source: %v", err)
	}
	_, err = store.CreateModelSource(t.Context(), state.ModelSourceUpsertRequest{
		Name:            "Anthropic",
		BaseURL:         "https://anthropic.example/v1",
		ProviderType:    "anthropic-compatible",
		DefaultModelID:  "claude-3-7-sonnet",
		ExposedModelIDs: []string{"claude-3-haiku"},
		Enabled:         true,
		APIKey:          "sk-ant-test",
	})
	if err != nil {
		t.Fatalf("create anthropic source: %v", err)
	}
	if err := store.ReplaceSelectedModels(t.Context(), []state.SelectedModel{
		{ModelID: "claude-3-7-sonnet", Position: 0},
		{ModelID: "claude-3-haiku", Position: 1},
		{ModelID: "gpt-4.1-mini", Position: 2},
		{ModelID: "gpt-upstream-error", Position: 3},
	}); err != nil {
		t.Fatalf("replace selected models: %v", err)
	}

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			rec := httptest.NewRecorder()

			switch req.URL.String() {
			case "https://openai.example/v1/models":
				modelsHits["openai"]++
				_ = json.NewEncoder(rec).Encode(map[string]any{
					"data": []map[string]any{
						{"id": "gpt-4.1", "object": "model", "owned_by": "openai-compatible"},
						{"id": "gpt-4.1-mini", "object": "model", "owned_by": "openai-compatible"},
						{"id": "gpt-upstream-error", "object": "model", "owned_by": "openai-compatible"},
					},
				})
			case "https://openai.example/v1/chat/completions":
				if req.Header.Get("x-trace-id") != "trace-openai" {
					rec.WriteHeader(http.StatusBadRequest)
					_ = json.NewEncoder(rec).Encode(map[string]any{
						"error":   "missing_header",
						"message": "x-trace-id missing",
					})
					return rec.Result(), nil
				}
				var payload map[string]any
				_ = json.NewDecoder(req.Body).Decode(&payload)
				if model, _ := payload["model"].(string); model == "gpt-upstream-error" {
					rec.Header().Set("Content-Type", "application/json")
					rec.WriteHeader(http.StatusTooManyRequests)
					_ = json.NewEncoder(rec).Encode(map[string]any{
						"error":   "rate_limited",
						"message": "quota exceeded",
					})
					return rec.Result(), nil
				}
				if stream, _ := payload["stream"].(bool); stream {
					rec.Header().Set("Content-Type", "text/event-stream")
					_, _ = rec.WriteString("data: {\"id\":\"chunk-1\"}\n\n")
					_, _ = rec.WriteString("data: [DONE]\n\n")
					return rec.Result(), nil
				}
				_ = json.NewEncoder(rec).Encode(map[string]any{"id": "chatcmpl-test", "object": "chat.completion"})
			case "https://openai.example/v1/responses":
				_ = json.NewEncoder(rec).Encode(map[string]any{"id": "resp-test", "object": "response"})
			case "https://anthropic.example/v1/messages":
				if req.Header.Get("x-request-id") != "trace-anthropic" {
					rec.WriteHeader(http.StatusBadRequest)
					_ = json.NewEncoder(rec).Encode(map[string]any{
						"error":   "missing_header",
						"message": "x-request-id missing",
					})
					return rec.Result(), nil
				}
				var payload map[string]any
				_ = json.NewDecoder(req.Body).Decode(&payload)
				if stream, _ := payload["stream"].(bool); stream {
					rec.Header().Set("Content-Type", "text/event-stream")
					_, _ = rec.WriteString("event: message_start\ndata: {\"type\":\"message_start\"}\n\n")
					_, _ = rec.WriteString("event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
					return rec.Result(), nil
				}
				_ = json.NewEncoder(rec).Encode(map[string]any{"id": "msg_test", "type": "message"})
			case "https://anthropic.example/v1/models":
				modelsHits["anthropic"]++
				rec.WriteHeader(http.StatusNotFound)
			case "https://anthropic.example/v1/messages/count_tokens":
				_ = json.NewEncoder(rec).Encode(map[string]any{"input_tokens": 3})
			case "https://offline.example/v1/models":
				rec.WriteHeader(http.StatusUnauthorized)
			default:
				rec.WriteHeader(http.StatusNotFound)
			}

			return rec.Result(), nil
		}),
	}

	router := NewRouterWithProxyAndInfo(store, executor.NewProxyWithClient(client), buildinfo.Info{
		RuntimeKind:     "ai-mini-gateway",
		Version:         "dev",
		Commit:          "unknown",
		ContractVersion: "v1",
		Host:            "127.0.0.1",
		Port:            3457,
		DataDir:         dir,
	})

	t.Run("health", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d", rec.Code)
		}
		var payload map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if payload["status"] != "ok" || payload["runtime_kind"] != "ai-mini-gateway" || payload["version"] != "dev" || payload["commit"] != "unknown" || payload["contract_version"] != "v1" {
			t.Fatalf("unexpected health payload: %+v", payload)
		}
	})

	t.Run("capabilities", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/capabilities", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
		}
		var payload map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if payload["runtime_kind"] != "ai-mini-gateway" ||
			payload["version"] != "dev" ||
			payload["commit"] != "unknown" ||
			payload["contract_version"] != "v1" ||
			payload["supports_openai_compatible"] != true ||
			payload["supports_anthropic_compatible"] != true ||
			payload["supports_models_api"] != true ||
			payload["supports_stream"] != true ||
			payload["supports_admin_api"] != true ||
			payload["supports_model_source_admin"] != true ||
			payload["supports_selected_model_admin"] != true ||
			payload["supports_source_capabilities"] != true ||
			payload["supports_atomic_source_sync"] != true ||
			payload["supports_runtime_version"] != true ||
			payload["supports_contract_version"] != true ||
			payload["supports_explicit_source_health"] != true {
			t.Fatalf("unexpected capabilities payload: %+v", payload)
		}
	})

	t.Run("runtime status", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/runtime/status", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
		}
		var payload map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if payload["runtime_kind"] != "ai-mini-gateway" ||
			payload["status"] != "ok" ||
			payload["version"] != "dev" ||
			payload["commit"] != "unknown" ||
			payload["contract_version"] != "v1" ||
			payload["host"] != "127.0.0.1" ||
			payload["port"] != float64(3457) ||
			payload["data_dir"] != dir ||
			payload["sync_in_progress"] != false {
			t.Fatalf("unexpected runtime status payload: %+v", payload)
		}
	})

	t.Run("models", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d", rec.Code)
		}
		if got := rec.Body.String(); got != "{\"data\":[{\"id\":\"claude-3-7-sonnet\",\"object\":\"model\",\"owned_by\":\"anthropic-compatible\"},{\"id\":\"claude-3-haiku\",\"object\":\"model\",\"owned_by\":\"anthropic-compatible\"},{\"id\":\"gpt-4.1-mini\",\"object\":\"model\",\"owned_by\":\"openai-compatible\"},{\"id\":\"gpt-upstream-error\",\"object\":\"model\",\"owned_by\":\"openai-compatible\"}]}\n" {
			t.Fatalf("unexpected body: %q", got)
		}
	})

	t.Run("models cache reused", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d", rec.Code)
		}
		if modelsHits["openai"] != 1 {
			t.Fatalf("expected openai models fetched once, got %d", modelsHits["openai"])
		}
		if modelsHits["anthropic"] != 1 {
			t.Fatalf("expected anthropic models fetched once, got %d", modelsHits["anthropic"])
		}
	})

	t.Run("unsupported models capability cached", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d", rec.Code)
		}
		if modelsHits["anthropic"] != 1 {
			t.Fatalf("expected unsupported anthropic /models not to be retried, got %d", modelsHits["anthropic"])
		}
	})

	t.Run("model source capabilities", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/model-sources/capabilities", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
		}
		var payload []map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if len(payload) != 2 {
			t.Fatalf("unexpected payload length: %d", len(payload))
		}
		if payload[0]["name"] != "OpenAI" ||
			payload[0]["models_api_status"] != "supported" ||
			payload[0]["supports_models_api"] != true ||
			payload[0]["supports_openai_chat_completions"] != true ||
			payload[0]["openai_chat_completions_status"] != "configured_supported" ||
			payload[0]["supports_openai_responses"] != true ||
			payload[0]["openai_responses_status"] != "configured_supported" ||
			payload[0]["supports_anthropic_messages"] != false ||
			payload[0]["supports_anthropic_count_tokens"] != false ||
			payload[0]["supports_stream"] != true ||
			payload[0]["stream_status"] != "configured_supported" {
			t.Fatalf("unexpected first capability row: %+v", payload[0])
		}
		if payload[1]["name"] != "Anthropic" ||
			payload[1]["models_api_status"] != "unsupported" ||
			payload[1]["supports_models_api"] != false ||
			payload[1]["supports_openai_chat_completions"] != false ||
			payload[1]["supports_openai_responses"] != false ||
			payload[1]["supports_anthropic_messages"] != true ||
			payload[1]["anthropic_messages_status"] != "configured_supported" ||
			payload[1]["supports_anthropic_count_tokens"] != true ||
			payload[1]["anthropic_count_tokens_status"] != "configured_supported" ||
			payload[1]["supports_stream"] != true ||
			payload[1]["stream_status"] != "configured_supported" {
			t.Fatalf("unexpected second capability row: %+v", payload[1])
		}
	})

	t.Run("admin write invalidates models cache", func(t *testing.T) {
		body := []byte(`{"name":"Extra","base_url":"https://extra.example/v1","provider_type":"openai-compatible","default_model_id":"gpt-extra","enabled":true,"api_key":"sk-extra"}`)
		req := httptest.NewRequest(http.MethodPost, "/admin/model-sources", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
		}

		modelsReq := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
		modelsRec := httptest.NewRecorder()
		router.ServeHTTP(modelsRec, modelsReq)

		if modelsRec.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d body=%s", modelsRec.Code, modelsRec.Body.String())
		}
		if modelsHits["openai"] != 2 {
			t.Fatalf("expected openai models fetched twice after invalidation, got %d", modelsHits["openai"])
		}
		if modelsHits["anthropic"] != 2 {
			t.Fatalf("expected anthropic models fetched twice after invalidation, got %d", modelsHits["anthropic"])
		}
	})

	t.Run("non-selected model rejected", func(t *testing.T) {
		body := []byte(`{"model":"gpt-4.1","messages":[{"role":"user","content":"hello"}]}`)
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("chat completions", func(t *testing.T) {
		body := []byte(`{"model":"gpt-4.1-mini","messages":[{"role":"user","content":"hello"}]}`)
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
		req.Header.Set("x-trace-id", "trace-openai")
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
		req.Header.Set("x-request-id", "trace-anthropic")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("anthropic exposed fallback model", func(t *testing.T) {
		body := []byte(`{"model":"claude-3-haiku","messages":[{"role":"user","content":"hello"}],"max_tokens":128}`)
		req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
		req.Header.Set("anthropic-version", "2023-06-01")
		req.Header.Set("x-request-id", "trace-anthropic")
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

	t.Run("chat completions stream", func(t *testing.T) {
		body := []byte(`{"model":"gpt-4.1-mini","stream":true,"messages":[{"role":"user","content":"hello"}]}`)
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
		req.Header.Set("x-trace-id", "trace-openai")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
		}
		if got := rec.Header().Get("Content-Type"); got != "text/event-stream" {
			t.Fatalf("unexpected content-type: %s", got)
		}
		if got := rec.Body.String(); got != "data: {\"id\":\"chunk-1\"}\n\ndata: [DONE]\n\n" {
			t.Fatalf("unexpected body: %q", got)
		}
	})

	t.Run("anthropic stream", func(t *testing.T) {
		body := []byte(`{"model":"claude-3-7-sonnet","stream":true,"messages":[{"role":"user","content":"hello"}],"max_tokens":128}`)
		req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
		req.Header.Set("anthropic-version", "2023-06-01")
		req.Header.Set("x-request-id", "trace-anthropic")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
		}
		if got := rec.Header().Get("Content-Type"); got != "text/event-stream" {
			t.Fatalf("unexpected content-type: %s", got)
		}
		if got := rec.Body.String(); got != "event: message_start\ndata: {\"type\":\"message_start\"}\n\nevent: message_stop\ndata: {\"type\":\"message_stop\"}\n\n" {
			t.Fatalf("unexpected body: %q", got)
		}
	})

	t.Run("dynamic operation capability status observed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/model-sources/capabilities", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
		}
		var payload []map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if payload[0]["openai_chat_completions_status"] != "supported" {
			t.Fatalf("unexpected openai dynamic status: %+v", payload[0])
		}
		if payload[0]["stream_status"] != "supported" {
			t.Fatalf("unexpected openai stream status: %+v", payload[0])
		}
		if payload[1]["anthropic_messages_status"] != "supported" {
			t.Fatalf("unexpected anthropic dynamic status: %+v", payload[1])
		}
		if payload[1]["stream_status"] != "supported" {
			t.Fatalf("unexpected anthropic stream status: %+v", payload[1])
		}
	})

	t.Run("upstream json error mapped", func(t *testing.T) {
		body := []byte(`{"model":"gpt-upstream-error","messages":[{"role":"user","content":"hello"}]}`)
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
		req.Header.Set("x-trace-id", "trace-openai")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusTooManyRequests {
			t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
		}
		if got := rec.Body.String(); got != "{\"error\":\"rate_limited\",\"message\":\"quota exceeded\"}\n" {
			t.Fatalf("unexpected body: %q", got)
		}
	})

	t.Run("selected models admin duplicate rejected", func(t *testing.T) {
		body := []byte(`[{"model_id":"gpt-4.1-mini","position":0},{"model_id":"gpt-4.1-mini","position":1}]`)
		req := httptest.NewRequest(http.MethodPut, "/admin/selected-models", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusConflict {
			t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("source healthcheck openai", func(t *testing.T) {
		sources := store.ListModelSources()
		req := httptest.NewRequest(http.MethodPost, "/admin/model-sources/"+sources[0].ID+"/healthcheck", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
		}
		var payload map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if payload["status"] != "ok" || payload["status_code"] != float64(200) || payload["checked_at"] == "" {
			t.Fatalf("unexpected healthcheck payload: %+v", payload)
		}
	})

	t.Run("source healthcheck anthropic fallback", func(t *testing.T) {
		sources := store.ListModelSources()
		req := httptest.NewRequest(http.MethodPost, "/admin/model-sources/"+sources[1].ID+"/healthcheck", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
		}
		var payload map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if payload["status"] != "ok" || payload["status_code"] != float64(200) {
			t.Fatalf("unexpected anthropic healthcheck payload: %+v", payload)
		}
	})

	t.Run("source healthcheck error result", func(t *testing.T) {
		source, err := store.CreateModelSource(t.Context(), state.ModelSourceUpsertRequest{
			Name:           "Offline",
			BaseURL:        "https://offline.example/v1",
			ProviderType:   "openai-compatible",
			DefaultModelID: "gpt-offline",
			Enabled:        true,
			APIKey:         "sk-offline",
		})
		if err != nil {
			t.Fatalf("create source: %v", err)
		}

		req := httptest.NewRequest(http.MethodPost, "/admin/model-sources/"+source.ID+"/healthcheck", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
		}
		var payload map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if payload["status"] != "error" || payload["status_code"] != float64(401) {
			t.Fatalf("unexpected error healthcheck payload: %+v", payload)
		}
	})

	t.Run("source healthcheck not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/admin/model-sources/src_missing/healthcheck", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("runtime sync replaces config atomically", func(t *testing.T) {
		openAIHitsBefore := modelsHits["openai"]

		body := []byte(`{
			"sources":[
				{
					"name":"OpenAI Synced",
					"base_url":"https://openai.example/v1",
					"provider_type":"openai-compatible",
					"default_model_id":"gpt-4.1",
					"exposed_model_ids":["gpt-4.1-mini"],
					"enabled":true,
					"position":0,
					"api_key":"sk-sync"
				}
			],
			"selected_models":[
				{"model_id":"gpt-4.1-mini","position":0}
			]
		}`)
		req := httptest.NewRequest(http.MethodPut, "/admin/runtime/sync", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
		}
		var payload map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if payload["applied_sources"] != float64(1) || payload["applied_selected_models"] != float64(1) || payload["last_synced_at"] == "" {
			t.Fatalf("unexpected sync payload: %+v", payload)
		}

		sourcesReq := httptest.NewRequest(http.MethodGet, "/admin/model-sources", nil)
		sourcesRec := httptest.NewRecorder()
		router.ServeHTTP(sourcesRec, sourcesReq)

		var sources []map[string]any
		if err := json.Unmarshal(sourcesRec.Body.Bytes(), &sources); err != nil {
			t.Fatalf("decode sources: %v", err)
		}
		if len(sources) != 1 || sources[0]["name"] != "OpenAI Synced" {
			t.Fatalf("unexpected sources after sync: %+v", sources)
		}

		selectedReq := httptest.NewRequest(http.MethodGet, "/admin/selected-models", nil)
		selectedRec := httptest.NewRecorder()
		router.ServeHTTP(selectedRec, selectedReq)

		if got := selectedRec.Body.String(); got != "[{\"model_id\":\"gpt-4.1-mini\",\"position\":0}]\n" {
			t.Fatalf("unexpected selected models after sync: %q", got)
		}

		modelsReq := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
		modelsRec := httptest.NewRecorder()
		router.ServeHTTP(modelsRec, modelsReq)

		if modelsRec.Code != http.StatusOK {
			t.Fatalf("unexpected status after sync models read: %d body=%s", modelsRec.Code, modelsRec.Body.String())
		}
		if modelsHits["openai"] != openAIHitsBefore+1 {
			t.Fatalf("expected openai models fetched once more after sync invalidation, before=%d after=%d", openAIHitsBefore, modelsHits["openai"])
		}
	})

	t.Run("runtime sync invalid request preserves previous config", func(t *testing.T) {
		body := []byte(`{
			"sources":[
				{
					"name":"Broken",
					"base_url":"https://openai.example/v1",
					"provider_type":"openai-compatible",
					"default_model_id":"gpt-4.1",
					"enabled":true,
					"position":0,
					"api_key":"sk-sync"
				}
			],
			"selected_models":[
				{"model_id":"does-not-exist","position":0}
			]
		}`)
		req := httptest.NewRequest(http.MethodPut, "/admin/runtime/sync", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
		}

		sourcesReq := httptest.NewRequest(http.MethodGet, "/admin/model-sources", nil)
		sourcesRec := httptest.NewRecorder()
		router.ServeHTTP(sourcesRec, sourcesReq)

		var sources []map[string]any
		if err := json.Unmarshal(sourcesRec.Body.Bytes(), &sources); err != nil {
			t.Fatalf("decode sources: %v", err)
		}
		if len(sources) != 1 || sources[0]["name"] != "OpenAI Synced" {
			t.Fatalf("expected previous config to be preserved, got %+v", sources)
		}

		statusReq := httptest.NewRequest(http.MethodGet, "/runtime/status", nil)
		statusRec := httptest.NewRecorder()
		router.ServeHTTP(statusRec, statusReq)

		var payload map[string]any
		if err := json.Unmarshal(statusRec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode runtime status: %v", err)
		}
		if payload["last_sync_error"] == "" {
			t.Fatalf("expected last_sync_error to be recorded, got %+v", payload)
		}
	})

	t.Run("runtime sync conflict while in progress", func(t *testing.T) {
		if !store.TryBeginRuntimeSync() {
			t.Fatal("expected to acquire sync guard")
		}
		defer store.EndRuntimeSync()

		req := httptest.NewRequest(http.MethodPut, "/admin/runtime/sync", bytes.NewReader([]byte(`{"sources":[],"selected_models":[]}`)))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusConflict {
			t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
		}

		statusReq := httptest.NewRequest(http.MethodGet, "/runtime/status", nil)
		statusRec := httptest.NewRecorder()
		router.ServeHTTP(statusRec, statusReq)

		var payload map[string]any
		if err := json.Unmarshal(statusRec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode runtime status: %v", err)
		}
		if payload["sync_in_progress"] != true {
			t.Fatalf("expected sync_in_progress=true, got %+v", payload)
		}
	})

	t.Run("model source admin invalid provider rejected", func(t *testing.T) {
		body := []byte(`{"name":"Bad","base_url":"https://bad.example/v1","provider_type":"custom","default_model_id":"x","enabled":true,"api_key":"sk-test"}`)
		req := httptest.NewRequest(http.MethodPost, "/admin/model-sources", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
		}
	})
}

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

package inbound

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/yuanjunliang/ai-mini-gateway/internal/runtime/executor"
	"github.com/yuanjunliang/ai-mini-gateway/internal/runtime/state"
	"github.com/yuanjunliang/ai-mini-gateway/internal/runtime/web"
)

type openAIChatRequest struct {
	Model string `json:"model"`
}

type openAIResponseRequest struct {
	Model string `json:"model"`
}

func RegisterOpenAI(mux *http.ServeMux, store *state.Store, proxy *executor.Proxy) {
	mux.HandleFunc("GET /v1/models", func(w http.ResponseWriter, r *http.Request) {
		web.WriteJSON(w, http.StatusOK, map[string]any{
			"data": store.ListModels(),
		})
	})

	mux.HandleFunc("POST /v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		body, req, err := decodeOpenAIChatRequest(r)
		if err != nil {
			web.WriteError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}

		source, err := store.ResolveModelSource(req.Model, "openai-compatible")
		if err != nil {
			web.WriteError(w, http.StatusBadRequest, "model_not_available", err.Error())
			return
		}

		resp, err := proxy.Forward(r.Context(), source, "/chat/completions", r.Header, body)
		if err != nil {
			web.WriteError(w, http.StatusBadGateway, "upstream_request_failed", err.Error())
			return
		}
		web.WriteProxyResponse(w, resp)
	})

	mux.HandleFunc("POST /v1/responses", func(w http.ResponseWriter, r *http.Request) {
		body, req, err := decodeOpenAIResponseRequest(r)
		if err != nil {
			web.WriteError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}

		source, err := store.ResolveModelSource(req.Model, "openai-compatible")
		if err != nil {
			web.WriteError(w, http.StatusBadRequest, "model_not_available", err.Error())
			return
		}

		resp, err := proxy.Forward(r.Context(), source, "/responses", r.Header, body)
		if err != nil {
			web.WriteError(w, http.StatusBadGateway, "upstream_request_failed", err.Error())
			return
		}
		web.WriteProxyResponse(w, resp)
	})
}

func decodeOpenAIChatRequest(r *http.Request) ([]byte, openAIChatRequest, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, openAIChatRequest{}, err
	}
	var req openAIChatRequest
	err = json.Unmarshal(body, &req)
	return body, req, err
}

func decodeOpenAIResponseRequest(r *http.Request) ([]byte, openAIResponseRequest, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, openAIResponseRequest{}, err
	}
	var req openAIResponseRequest
	err = json.Unmarshal(body, &req)
	return body, req, err
}

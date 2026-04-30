package inbound

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/yuanjunliang/ai-mini-gateway/internal/runtime/executor"
	"github.com/yuanjunliang/ai-mini-gateway/internal/runtime/state"
	"github.com/yuanjunliang/ai-mini-gateway/internal/runtime/web"
)

type anthropicMessageRequest struct {
	Model string `json:"model"`
}

type anthropicCountTokensRequest struct {
	Model string `json:"model"`
}

func RegisterAnthropic(mux *http.ServeMux, store *state.Store, proxy *executor.Proxy) {
	mux.HandleFunc("POST /v1/messages", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("anthropic-version") == "" {
			web.WriteError(w, http.StatusBadRequest, "missing_header", "anthropic-version header is required")
			return
		}

		body, req, err := decodeAnthropicMessageRequest(r)
		if err != nil {
			web.WriteError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}

		source, err := resolveModelSource(r.Context(), store, proxy, req.Model, "anthropic-compatible")
		if err != nil {
			web.WriteError(w, http.StatusBadRequest, "model_not_available", err.Error())
			return
		}

		resp, err := proxy.Forward(r.Context(), source, "/messages", r.Header, body)
		if err != nil {
			web.WriteError(w, http.StatusBadGateway, "upstream_request_failed", err.Error())
			return
		}
		if handled := web.WriteProxyOrError(w, resp); handled {
			return
		}
	})

	mux.HandleFunc("POST /v1/messages/count_tokens", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("anthropic-version") == "" {
			web.WriteError(w, http.StatusBadRequest, "missing_header", "anthropic-version header is required")
			return
		}

		body, req, err := decodeAnthropicCountTokensRequest(r)
		if err != nil {
			web.WriteError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}

		source, err := resolveModelSource(r.Context(), store, proxy, req.Model, "anthropic-compatible")
		if err != nil {
			web.WriteError(w, http.StatusBadRequest, "model_not_available", err.Error())
			return
		}

		resp, err := proxy.Forward(r.Context(), source, "/messages/count_tokens", r.Header, body)
		if err != nil {
			web.WriteError(w, http.StatusBadGateway, "upstream_request_failed", err.Error())
			return
		}
		if handled := web.WriteProxyOrError(w, resp); handled {
			return
		}
	})
}

func decodeAnthropicMessageRequest(r *http.Request) ([]byte, anthropicMessageRequest, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, anthropicMessageRequest{}, err
	}
	var req anthropicMessageRequest
	err = json.Unmarshal(body, &req)
	return body, req, err
}

func decodeAnthropicCountTokensRequest(r *http.Request) ([]byte, anthropicCountTokensRequest, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, anthropicCountTokensRequest{}, err
	}
	var req anthropicCountTokensRequest
	err = json.Unmarshal(body, &req)
	return body, req, err
}

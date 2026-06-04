package inbound

import (
	"encoding/json"
	"net/http"

	"github.com/yuanjunliang/ai-mini-gateway/internal/runtime/executor"
	"github.com/yuanjunliang/ai-mini-gateway/internal/runtime/providers"
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
		models, err := aggregateModels(r.Context(), store, proxy)
		if err != nil {
			web.WriteError(w, http.StatusInternalServerError, "models_unavailable", err.Error())
			return
		}
		web.WriteJSON(w, http.StatusOK, map[string]any{"data": models})
	})

	mux.HandleFunc("POST /v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		provider := providers.ForSource(state.ModelSource{ProviderType: "openai-compatible"})
		body, req, err := decodeOpenAIChatRequest(r)
		if err != nil {
			web.WriteError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}

		source, err := resolveModelSource(r.Context(), store, proxy, req.Model, "openai-compatible")
		if err != nil {
			web.WriteError(w, http.StatusBadRequest, "model_not_available", err.Error())
			return
		}

		resp, err := proxy.ForwardOperation(r.Context(), source, providers.OperationOpenAIChatCompletions, r.Header, body)
		if err != nil {
			web.WriteError(w, http.StatusBadGateway, "upstream_request_failed", err.Error())
			return
		}
		writeProviderResponse(w, provider, providers.OperationOpenAIChatCompletions, resp)
	})

	mux.HandleFunc("POST /v1/responses", func(w http.ResponseWriter, r *http.Request) {
		provider := providers.ForSource(state.ModelSource{ProviderType: "openai-compatible"})
		body, req, err := decodeOpenAIResponseRequest(r)
		if err != nil {
			web.WriteError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}

		source, err := resolveModelSource(r.Context(), store, proxy, req.Model, "openai-compatible")
		if err != nil {
			web.WriteError(w, http.StatusBadRequest, "model_not_available", err.Error())
			return
		}

		if shouldUseChatCompletionsResponsesAdapter(proxy, source.ID) {
			forwardResponsesViaChatCompletions(w, r, proxy, source, body)
			return
		}

		resp, err := proxy.ForwardOperation(r.Context(), source, providers.OperationOpenAIResponses, r.Header, body)
		if err != nil {
			web.WriteError(w, http.StatusBadGateway, "upstream_request_failed", err.Error())
			return
		}
		if provider.IsOperationUnsupported(providers.OperationOpenAIResponses, resp.StatusCode) {
			drainAndClose(resp.Body)
			forwardResponsesViaChatCompletions(w, r, proxy, source, body)
			return
		}
		writeProviderResponse(w, provider, providers.OperationOpenAIResponses, resp)
	})
}

func shouldUseChatCompletionsResponsesAdapter(proxy *executor.Proxy, sourceID string) bool {
	switch proxy.OperationStatus(sourceID, providers.OperationOpenAIResponses) {
	case "unsupported", "adapted":
		return true
	default:
		return false
	}
}

func forwardResponsesViaChatCompletions(w http.ResponseWriter, r *http.Request, proxy *executor.Proxy, source state.ModelSource, body []byte) {
	chatBody, responsePayload, err := responsesToChatCompletionsBody(body)
	if err != nil {
		web.WriteError(w, http.StatusBadRequest, "invalid_responses_request", err.Error())
		return
	}
	resp, err := proxy.ForwardOperation(r.Context(), source, providers.OperationOpenAIChatCompletions, r.Header, chatBody)
	if err != nil {
		web.WriteError(w, http.StatusBadGateway, "upstream_request_failed", err.Error())
		return
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		proxy.ObserveOperationStatus(source.ID, providers.OperationOpenAIResponses, "adapted")
	}
	if responsePayload.Stream {
		writeChatCompletionStreamAsResponses(w, resp, responsePayload.Model)
		return
	}
	writeChatCompletionAsResponse(w, resp, responsePayload.Model)
}

func decodeOpenAIChatRequest(r *http.Request) ([]byte, openAIChatRequest, error) {
	body, err := readDecodedRequestBody(r)
	if err != nil {
		return nil, openAIChatRequest{}, err
	}
	var req openAIChatRequest
	err = json.Unmarshal(body, &req)
	return body, req, err
}

func decodeOpenAIResponseRequest(r *http.Request) ([]byte, openAIResponseRequest, error) {
	body, err := readDecodedRequestBody(r)
	if err != nil {
		return nil, openAIResponseRequest{}, err
	}
	var req openAIResponseRequest
	err = json.Unmarshal(body, &req)
	return body, req, err
}

package inbound

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/yuanjunliang/ai-mini-gateway/internal/runtime/state"
	"github.com/yuanjunliang/ai-mini-gateway/internal/runtime/web"
)

type openAIChatRequest struct {
	Model    string              `json:"model"`
	Stream   bool                `json:"stream"`
	Messages []openAIChatMessage `json:"messages"`
}

type openAIChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponseRequest struct {
	Model  string        `json:"model"`
	Stream bool          `json:"stream"`
	Input  []responseMsg `json:"input"`
}

type responseMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func RegisterOpenAI(mux *http.ServeMux, store *state.Store) {
	mux.HandleFunc("GET /v1/models", func(w http.ResponseWriter, r *http.Request) {
		web.WriteJSON(w, http.StatusOK, map[string]any{
			"data": store.ListModels(),
		})
	})

	mux.HandleFunc("POST /v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		var req openAIChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			web.WriteError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}

		if err := store.ValidateModel(req.Model); err != nil {
			web.WriteError(w, http.StatusBadRequest, "model_not_available", err.Error())
			return
		}

		text := composeChatReply(req.Messages)
		if req.Stream {
			streamOpenAIChat(w, req.Model, text)
			return
		}

		web.WriteJSON(w, http.StatusOK, map[string]any{
			"id":      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   req.Model,
			"choices": []any{
				map[string]any{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": text,
					},
					"finish_reason": "stop",
				},
			},
		})
	})

	mux.HandleFunc("POST /v1/responses", func(w http.ResponseWriter, r *http.Request) {
		var req openAIResponseRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			web.WriteError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}

		if err := store.ValidateModel(req.Model); err != nil {
			web.WriteError(w, http.StatusBadRequest, "model_not_available", err.Error())
			return
		}

		text := composeResponseReply(req.Input)
		if req.Stream {
			streamOpenAIResponse(w, req.Model, text)
			return
		}

		web.WriteJSON(w, http.StatusOK, map[string]any{
			"id":     fmt.Sprintf("resp-%d", time.Now().UnixNano()),
			"object": "response",
			"model":  req.Model,
			"output": []any{
				map[string]any{
					"type": "message",
					"role": "assistant",
					"content": []any{
						map[string]any{
							"type": "output_text",
							"text": text,
						},
					},
				},
			},
		})
	})
}

func composeChatReply(messages []openAIChatMessage) string {
	if len(messages) == 0 {
		return "gateway runtime is ready"
	}

	last := messages[len(messages)-1].Content
	if last == "" {
		return "gateway received an empty prompt"
	}

	return "echo: " + last
}

func composeResponseReply(messages []responseMsg) string {
	if len(messages) == 0 {
		return "gateway runtime is ready"
	}

	last := messages[len(messages)-1].Content
	if last == "" {
		return "gateway received an empty prompt"
	}

	return "echo: " + last
}

func streamOpenAIChat(w http.ResponseWriter, model string, text string) {
	web.WriteSSEHeaders(w)
	web.WriteSSE(w, map[string]any{
		"id":      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
		"object":  "chat.completion.chunk",
		"created": time.Now().Unix(),
		"model":   model,
		"choices": []any{
			map[string]any{
				"index":         0,
				"delta":         map[string]any{"role": "assistant", "content": text},
				"finish_reason": nil,
			},
		},
	})
	web.WriteSSE(w, map[string]any{
		"id":      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
		"object":  "chat.completion.chunk",
		"created": time.Now().Unix(),
		"model":   model,
		"choices": []any{
			map[string]any{
				"index":         0,
				"delta":         map[string]any{},
				"finish_reason": "stop",
			},
		},
	})
	web.WriteDoneSSE(w)
}

func streamOpenAIResponse(w http.ResponseWriter, model string, text string) {
	web.WriteSSEHeaders(w)
	web.WriteSSE(w, map[string]any{
		"type":  "response.output_text.delta",
		"model": model,
		"delta": text,
	})
	web.WriteSSE(w, map[string]any{
		"type":  "response.completed",
		"model": model,
	})
	web.WriteDoneSSE(w)
}

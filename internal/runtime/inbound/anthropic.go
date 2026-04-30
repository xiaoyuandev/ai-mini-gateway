package inbound

import (
	"encoding/json"
	"net/http"

	"github.com/yuanjunliang/ai-mini-gateway/internal/runtime/state"
	"github.com/yuanjunliang/ai-mini-gateway/internal/runtime/web"
)

type anthropicMessageRequest struct {
	Model     string                    `json:"model"`
	Stream    bool                      `json:"stream"`
	MaxTokens int                       `json:"max_tokens"`
	Messages  []anthropicMessageContent `json:"messages"`
}

type anthropicMessageContent struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicCountTokensRequest struct {
	Model    string                    `json:"model"`
	Messages []anthropicMessageContent `json:"messages"`
}

func RegisterAnthropic(mux *http.ServeMux, store *state.Store) {
	mux.HandleFunc("POST /v1/messages", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("anthropic-version") == "" {
			web.WriteError(w, http.StatusBadRequest, "missing_header", "anthropic-version header is required")
			return
		}

		var req anthropicMessageRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			web.WriteError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}

		if err := store.ValidateModel(req.Model); err != nil {
			web.WriteError(w, http.StatusBadRequest, "model_not_available", err.Error())
			return
		}

		text := composeAnthropicReply(req.Messages)
		if req.Stream {
			streamAnthropicMessage(w, req.Model, text)
			return
		}

		web.WriteJSON(w, http.StatusOK, map[string]any{
			"id":    "msg_stub",
			"type":  "message",
			"role":  "assistant",
			"model": req.Model,
			"content": []any{
				map[string]any{
					"type": "text",
					"text": text,
				},
			},
			"stop_reason": "end_turn",
		})
	})

	mux.HandleFunc("POST /v1/messages/count_tokens", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("anthropic-version") == "" {
			web.WriteError(w, http.StatusBadRequest, "missing_header", "anthropic-version header is required")
			return
		}

		var req anthropicCountTokensRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			web.WriteError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}

		if err := store.ValidateModel(req.Model); err != nil {
			web.WriteError(w, http.StatusBadRequest, "model_not_available", err.Error())
			return
		}

		web.WriteJSON(w, http.StatusOK, map[string]any{
			"input_tokens": estimateTokensFromAnthropic(req.Messages),
		})
	})
}

func composeAnthropicReply(messages []anthropicMessageContent) string {
	if len(messages) == 0 {
		return "gateway runtime is ready"
	}

	last := messages[len(messages)-1].Content
	if last == "" {
		return "gateway received an empty prompt"
	}

	return "echo: " + last
}

func estimateTokensFromAnthropic(messages []anthropicMessageContent) int {
	total := 0
	for _, message := range messages {
		total += len(message.Content)
	}

	if total == 0 {
		return 0
	}

	return (total + 3) / 4
}

func streamAnthropicMessage(w http.ResponseWriter, model string, text string) {
	web.WriteSSEHeaders(w)
	web.WriteNamedSSE(w, "message_start", map[string]any{
		"type":    "message_start",
		"message": map[string]any{"id": "msg_stub", "model": model, "role": "assistant"},
	})
	web.WriteNamedSSE(w, "content_block_delta", map[string]any{
		"type":  "content_block_delta",
		"index": 0,
		"delta": map[string]any{"type": "text_delta", "text": text},
	})
	web.WriteNamedSSE(w, "message_stop", map[string]any{
		"type": "message_stop",
	})
}

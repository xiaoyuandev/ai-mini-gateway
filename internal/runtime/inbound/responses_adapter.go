package inbound

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/yuanjunliang/ai-mini-gateway/internal/runtime/web"
)

type responsesRequestPayload struct {
	Model           string          `json:"model"`
	Input           json.RawMessage `json:"input"`
	Instructions    string          `json:"instructions"`
	Stream          bool            `json:"stream"`
	Temperature     *float64        `json:"temperature,omitempty"`
	TopP            *float64        `json:"top_p,omitempty"`
	MaxOutputTokens *int            `json:"max_output_tokens,omitempty"`
}

type chatCompletionPayload struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Stream      bool          `json:"stream,omitempty"`
	Temperature *float64      `json:"temperature,omitempty"`
	TopP        *float64      `json:"top_p,omitempty"`
	MaxTokens   *int          `json:"max_tokens,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created,omitempty"`
	Model   string `json:"model,omitempty"`
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

type chatCompletionChunk struct {
	ID      string `json:"id"`
	Created int64  `json:"created,omitempty"`
	Model   string `json:"model,omitempty"`
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason any `json:"finish_reason"`
	} `json:"choices"`
}

func responsesToChatCompletionsBody(body []byte) ([]byte, responsesRequestPayload, error) {
	var payload responsesRequestPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, responsesRequestPayload{}, err
	}

	messages := make([]chatMessage, 0, 4)
	if strings.TrimSpace(payload.Instructions) != "" {
		messages = append(messages, chatMessage{Role: "system", Content: payload.Instructions})
	}

	inputMessages, err := responseInputToMessages(payload.Input)
	if err != nil {
		return nil, responsesRequestPayload{}, err
	}
	messages = append(messages, inputMessages...)

	chatPayload := chatCompletionPayload{
		Model:       payload.Model,
		Messages:    messages,
		Stream:      payload.Stream,
		Temperature: payload.Temperature,
		TopP:        payload.TopP,
		MaxTokens:   payload.MaxOutputTokens,
	}

	encoded, err := json.Marshal(chatPayload)
	if err != nil {
		return nil, responsesRequestPayload{}, err
	}
	return encoded, payload, nil
}

func responseInputToMessages(input json.RawMessage) ([]chatMessage, error) {
	if len(bytes.TrimSpace(input)) == 0 {
		return nil, nil
	}

	var text string
	if err := json.Unmarshal(input, &text); err == nil {
		if strings.TrimSpace(text) == "" {
			return nil, nil
		}
		return []chatMessage{{Role: "user", Content: text}}, nil
	}

	var items []map[string]any
	if err := json.Unmarshal(input, &items); err != nil {
		return nil, err
	}

	messages := make([]chatMessage, 0, len(items))
	for _, item := range items {
		role, _ := item["role"].(string)
		role = normalizeChatRole(role)
		content := responseInputContentToText(item["content"])
		if strings.TrimSpace(content) == "" {
			continue
		}
		messages = append(messages, chatMessage{Role: role, Content: content})
	}
	return messages, nil
}

func normalizeChatRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "system", "user", "assistant", "tool":
		return strings.ToLower(strings.TrimSpace(role))
	case "developer":
		return "system"
	default:
		return "user"
	}
}

func responseInputContentToText(value any) string {
	switch content := value.(type) {
	case string:
		return content
	case []any:
		parts := make([]string, 0, len(content))
		for _, item := range content {
			object, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if text, _ := object["text"].(string); strings.TrimSpace(text) != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	default:
		return ""
	}
}

func writeChatCompletionAsResponse(w http.ResponseWriter, resp *http.Response, fallbackModel string) {
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		web.WriteProxyResponse(w, resp)
		return
	}

	var payload chatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		web.WriteError(w, http.StatusBadGateway, "invalid_chat_completion_response", err.Error())
		return
	}

	text := ""
	if len(payload.Choices) > 0 {
		text = payload.Choices[0].Message.Content
	}
	model := firstNonEmpty(payload.Model, fallbackModel)
	responseID := firstNonEmpty(payload.ID, fmt.Sprintf("resp_%d", time.Now().UnixNano()))

	web.WriteJSON(w, http.StatusOK, map[string]any{
		"id":      responseID,
		"object":  "response",
		"created": firstNonZero(payload.Created, time.Now().Unix()),
		"model":   model,
		"output": []map[string]any{
			{
				"type": "message",
				"role": "assistant",
				"content": []map[string]any{
					{"type": "output_text", "text": text},
				},
			},
		},
		"output_text": text,
	})
}

func writeChatCompletionStreamAsResponses(w http.ResponseWriter, resp *http.Response, fallbackModel string) {
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		web.WriteProxyResponse(w, resp)
		return
	}

	web.WriteSSEHeaders(w)
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	responseID := fmt.Sprintf("resp_%d", time.Now().UnixNano())
	itemID := fmt.Sprintf("msg_%d", time.Now().UnixNano())
	model := fallbackModel
	created := time.Now().Unix()
	textParts := []string{}
	web.WriteNamedSSE(w, "response.created", map[string]any{
		"type": "response.created",
		"response": map[string]any{
			"id":      responseID,
			"object":  "response",
			"created": created,
			"model":   model,
		},
	})
	web.WriteNamedSSE(w, "response.in_progress", map[string]any{
		"type": "response.in_progress",
		"response": map[string]any{
			"id":      responseID,
			"object":  "response",
			"created": created,
			"model":   model,
		},
	})
	web.WriteNamedSSE(w, "response.output_item.added", map[string]any{
		"type":         "response.output_item.added",
		"output_index": 0,
		"item": map[string]any{
			"id":      itemID,
			"type":    "message",
			"status":  "in_progress",
			"role":    "assistant",
			"content": []map[string]any{},
		},
	})
	web.WriteNamedSSE(w, "response.content_part.added", map[string]any{
		"type":          "response.content_part.added",
		"item_id":       itemID,
		"output_index":  0,
		"content_index": 0,
		"part": map[string]any{
			"type": "output_text",
			"text": "",
		},
	})

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") || strings.HasPrefix(line, "event:") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}

		var chunk chatCompletionChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if strings.TrimSpace(chunk.ID) != "" {
			responseID = chunk.ID
		}
		if strings.TrimSpace(chunk.Model) != "" {
			model = chunk.Model
		}
		if chunk.Created > 0 {
			created = chunk.Created
		}
		for _, choice := range chunk.Choices {
			if choice.Delta.Content == "" {
				continue
			}
			textParts = append(textParts, choice.Delta.Content)
			web.WriteNamedSSE(w, "response.output_text.delta", map[string]any{
				"type":          "response.output_text.delta",
				"item_id":       itemID,
				"output_index":  0,
				"content_index": 0,
				"delta":         choice.Delta.Content,
			})
		}
	}

	outputText := strings.Join(textParts, "")
	web.WriteNamedSSE(w, "response.output_text.done", map[string]any{
		"type":          "response.output_text.done",
		"item_id":       itemID,
		"output_index":  0,
		"content_index": 0,
		"text":          outputText,
	})
	web.WriteNamedSSE(w, "response.content_part.done", map[string]any{
		"type":          "response.content_part.done",
		"item_id":       itemID,
		"output_index":  0,
		"content_index": 0,
		"part": map[string]any{
			"type": "output_text",
			"text": outputText,
		},
	})
	web.WriteNamedSSE(w, "response.output_item.done", map[string]any{
		"type":         "response.output_item.done",
		"output_index": 0,
		"item": map[string]any{
			"id":     itemID,
			"type":   "message",
			"status": "completed",
			"role":   "assistant",
			"content": []map[string]any{
				{"type": "output_text", "text": outputText},
			},
		},
	})
	web.WriteNamedSSE(w, "response.completed", map[string]any{
		"type": "response.completed",
		"response": map[string]any{
			"id":      responseID,
			"object":  "response",
			"created": created,
			"model":   model,
			"status":  "completed",
			"output": []map[string]any{
				{
					"id":     itemID,
					"type":   "message",
					"status": "completed",
					"role":   "assistant",
					"content": []map[string]any{
						{"type": "output_text", "text": outputText},
					},
				},
			},
			"output_text": outputText,
		},
	})
	web.WriteDoneSSE(w)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func firstNonZero(values ...int64) int64 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func drainAndClose(body io.ReadCloser) {
	if body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, body)
	_ = body.Close()
}

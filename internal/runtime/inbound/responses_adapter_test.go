package inbound

import (
	"encoding/json"
	"testing"
)

func TestResponsesToChatCompletionsBody(t *testing.T) {
	body := []byte(`{
		"model":"deepseek-v4-flash",
		"instructions":"answer briefly",
		"input":[
			{"role":"user","content":[{"type":"input_text","text":"hello"},{"type":"input_text","text":"world"}]},
			{"role":"assistant","content":"previous answer"}
		],
		"stream":true,
		"max_output_tokens":16,
		"temperature":0.2,
		"top_p":0.8
	}`)

	encoded, payload, err := responsesToChatCompletionsBody(body)
	if err != nil {
		t.Fatalf("convert body: %v", err)
	}
	if payload.Model != "deepseek-v4-flash" || !payload.Stream {
		t.Fatalf("unexpected source payload: %+v", payload)
	}

	var chat chatCompletionPayload
	if err := json.Unmarshal(encoded, &chat); err != nil {
		t.Fatalf("decode chat body: %v", err)
	}
	if chat.Model != "deepseek-v4-flash" || !chat.Stream {
		t.Fatalf("unexpected chat payload: %+v", chat)
	}
	if chat.MaxTokens == nil || *chat.MaxTokens != 16 {
		t.Fatalf("unexpected max tokens: %+v", chat.MaxTokens)
	}
	if chat.Temperature == nil || *chat.Temperature != 0.2 {
		t.Fatalf("unexpected temperature: %+v", chat.Temperature)
	}
	if chat.TopP == nil || *chat.TopP != 0.8 {
		t.Fatalf("unexpected top_p: %+v", chat.TopP)
	}

	wantMessages := []chatMessage{
		{Role: "system", Content: "answer briefly"},
		{Role: "user", Content: "hello\nworld"},
		{Role: "assistant", Content: "previous answer"},
	}
	if len(chat.Messages) != len(wantMessages) {
		t.Fatalf("unexpected messages: %+v", chat.Messages)
	}
	for index, want := range wantMessages {
		if chat.Messages[index] != want {
			t.Fatalf("message %d = %+v, want %+v", index, chat.Messages[index], want)
		}
	}
}

func TestResponsesToChatCompletionsBodyStringInput(t *testing.T) {
	encoded, _, err := responsesToChatCompletionsBody([]byte(`{"model":"deepseek-v4-pro","input":"hello"}`))
	if err != nil {
		t.Fatalf("convert body: %v", err)
	}

	var chat chatCompletionPayload
	if err := json.Unmarshal(encoded, &chat); err != nil {
		t.Fatalf("decode chat body: %v", err)
	}
	if len(chat.Messages) != 1 || chat.Messages[0].Role != "user" || chat.Messages[0].Content != "hello" {
		t.Fatalf("unexpected messages: %+v", chat.Messages)
	}
}

func TestResponsesToChatCompletionsBodyNormalizesDeveloperRole(t *testing.T) {
	encoded, _, err := responsesToChatCompletionsBody([]byte(`{
		"model":"deepseek-v4-flash",
		"input":[
			{"role":"developer","content":"follow the repo instructions"},
			{"role":"latest_reminder","content":"stay concise"}
		]
	}`))
	if err != nil {
		t.Fatalf("convert body: %v", err)
	}

	var chat chatCompletionPayload
	if err := json.Unmarshal(encoded, &chat); err != nil {
		t.Fatalf("decode chat body: %v", err)
	}
	wantMessages := []chatMessage{
		{Role: "system", Content: "follow the repo instructions"},
		{Role: "user", Content: "stay concise"},
	}
	if len(chat.Messages) != len(wantMessages) {
		t.Fatalf("unexpected messages: %+v", chat.Messages)
	}
	for index, want := range wantMessages {
		if chat.Messages[index] != want {
			t.Fatalf("message %d = %+v, want %+v", index, chat.Messages[index], want)
		}
	}
}

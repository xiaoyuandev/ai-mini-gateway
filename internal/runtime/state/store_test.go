package state

import "testing"

func TestStoreModelSourceValidation(t *testing.T) {
	store := newTestStore(t)

	_, err := store.CreateModelSource(t.Context(), ModelSourceUpsertRequest{
		Name:           "Invalid",
		BaseURL:        "not-a-url",
		ProviderType:   "openai-compatible",
		DefaultModelID: "gpt-4.1",
		Enabled:        true,
	})
	if err != ErrInvalidInput {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}

	_, err = store.CreateModelSource(t.Context(), ModelSourceUpsertRequest{
		Name:           "Invalid",
		BaseURL:        "https://example.com/v1",
		ProviderType:   "custom",
		DefaultModelID: "gpt-4.1",
		Enabled:        true,
	})
	if err != ErrInvalidInput {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestStoreReplaceSelectedModelsRejectsDuplicates(t *testing.T) {
	store := newTestStore(t)

	err := store.ReplaceSelectedModels(t.Context(), []SelectedModel{
		{ModelID: "gpt-4.1", Position: 0},
		{ModelID: "gpt-4.1", Position: 1},
	})
	if err != ErrConflict {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}

func TestDeleteModelSourceRemovesOrphanedSelectedModels(t *testing.T) {
	store := newTestStore(t)

	openAI, err := store.CreateModelSource(t.Context(), ModelSourceUpsertRequest{
		Name:           "OpenAI",
		BaseURL:        "https://api.openai.com/v1",
		ProviderType:   "openai-compatible",
		DefaultModelID: "gpt-4.1",
		Enabled:        true,
		APIKey:         "sk-openai",
	})
	if err != nil {
		t.Fatalf("create openai source: %v", err)
	}

	_, err = store.CreateModelSource(t.Context(), ModelSourceUpsertRequest{
		Name:           "Anthropic",
		BaseURL:        "https://api.anthropic.com/v1",
		ProviderType:   "anthropic-compatible",
		DefaultModelID: "claude-3-7-sonnet",
		Enabled:        true,
		APIKey:         "sk-anthropic",
	})
	if err != nil {
		t.Fatalf("create anthropic source: %v", err)
	}

	err = store.ReplaceSelectedModels(t.Context(), []SelectedModel{
		{ModelID: "gpt-4.1", Position: 0},
		{ModelID: "claude-3-7-sonnet", Position: 1},
	})
	if err != nil {
		t.Fatalf("replace selected models: %v", err)
	}

	if err := store.DeleteModelSource(t.Context(), openAI.ID); err != nil {
		t.Fatalf("delete source: %v", err)
	}

	selected := store.ListSelectedModels()
	if len(selected) != 1 {
		t.Fatalf("expected 1 selected model, got %d", len(selected))
	}
	if selected[0].ModelID != "claude-3-7-sonnet" {
		t.Fatalf("unexpected selected model: %+v", selected[0])
	}
}

func newTestStore(t *testing.T) *Store {
	t.Helper()

	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	return store
}

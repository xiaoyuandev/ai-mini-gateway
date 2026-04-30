package state

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

var (
	ErrNotFound     = errors.New("resource not found")
	ErrConflict     = errors.New("resource conflict")
	ErrInvalidInput = errors.New("invalid input")
)

type Store struct {
	mu          sync.RWMutex
	dataDir     string
	statePath   string
	credsPath   string
	versionPath string
	data        persistedState
	credentials credentialsState
}

type persistedState struct {
	ModelSources   []ModelSource   `json:"model_sources"`
	SelectedModels []SelectedModel `json:"selected_models"`
	Meta           map[string]any  `json:"meta"`
}

type credentialsState struct {
	APIKeys map[string]string `json:"api_keys"`
}

type ModelSource struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	BaseURL        string `json:"base_url"`
	ProviderType   string `json:"provider_type"`
	DefaultModelID string `json:"default_model_id"`
	Enabled        bool   `json:"enabled"`
	Position       int    `json:"position"`
	APIKey         string `json:"api_key"`
	APIKeyMasked   string `json:"api_key_masked"`
}

type ModelSourceUpsertRequest struct {
	Name           string `json:"name"`
	BaseURL        string `json:"base_url"`
	ProviderType   string `json:"provider_type"`
	DefaultModelID string `json:"default_model_id"`
	Enabled        bool   `json:"enabled"`
	Position       int    `json:"position"`
	APIKey         string `json:"api_key"`
}

type ModelSourceOrderItem struct {
	ID       string `json:"id"`
	Position int    `json:"position"`
}

type SelectedModel struct {
	ModelID  string `json:"model_id"`
	Position int    `json:"position"`
}

type ExposedModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	OwnedBy string `json:"owned_by"`
}

func NewStore(dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}

	s := &Store{
		dataDir:     dataDir,
		statePath:   filepath.Join(dataDir, "state.json"),
		credsPath:   filepath.Join(dataDir, "credentials.json"),
		versionPath: filepath.Join(dataDir, "schema-version.json"),
		data: persistedState{
			ModelSources:   []ModelSource{},
			SelectedModels: []SelectedModel{},
			Meta:           map[string]any{"storage_backend": "json-file"},
		},
		credentials: credentialsState{
			APIKeys: map[string]string{},
		},
	}

	if err := s.load(); err != nil {
		return nil, err
	}
	if err := s.ensureVersionFile(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) Ping(context.Context) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, err := os.Stat(s.statePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if _, err := os.Stat(s.credsPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (s *Store) ListModelSources() []ModelSource {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return cloneSources(s.data.ModelSources, s.credentials.APIKeys)
}

func (s *Store) CreateModelSource(_ context.Context, req ModelSourceUpsertRequest) (ModelSource, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := validateModelSourceRequest(req); err != nil {
		return ModelSource{}, err
	}

	source := ModelSource{
		ID:             newID("src"),
		Name:           req.Name,
		BaseURL:        req.BaseURL,
		ProviderType:   req.ProviderType,
		DefaultModelID: req.DefaultModelID,
		Enabled:        req.Enabled,
		Position:       len(s.data.ModelSources),
	}

	s.data.ModelSources = append(s.data.ModelSources, source)
	s.credentials.APIKeys[source.ID] = strings.TrimSpace(req.APIKey)
	s.sortModelSourcesLocked()

	if err := s.persistLocked(); err != nil {
		return ModelSource{}, err
	}

	return withCredentialView(source, s.credentials.APIKeys[source.ID]), nil
}

func (s *Store) UpdateModelSource(_ context.Context, id string, req ModelSourceUpsertRequest) (ModelSource, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := validateModelSourceRequest(req); err != nil {
		return ModelSource{}, err
	}

	index := slices.IndexFunc(s.data.ModelSources, func(item ModelSource) bool { return item.ID == id })
	if index < 0 {
		return ModelSource{}, ErrNotFound
	}

	source := s.data.ModelSources[index]
	source.Name = req.Name
	source.BaseURL = req.BaseURL
	source.ProviderType = req.ProviderType
	source.DefaultModelID = req.DefaultModelID
	source.Enabled = req.Enabled
	s.data.ModelSources[index] = source
	s.credentials.APIKeys[id] = strings.TrimSpace(req.APIKey)

	if err := s.persistLocked(); err != nil {
		return ModelSource{}, err
	}

	return withCredentialView(source, s.credentials.APIKeys[id]), nil
}

func (s *Store) DeleteModelSource(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	index := slices.IndexFunc(s.data.ModelSources, func(item ModelSource) bool { return item.ID == id })
	if index < 0 {
		return ErrNotFound
	}

	s.data.ModelSources = append(s.data.ModelSources[:index], s.data.ModelSources[index+1:]...)
	delete(s.credentials.APIKeys, id)
	s.sortModelSourcesLocked()
	return s.persistLocked()
}

func (s *Store) ReorderModelSources(_ context.Context, items []ModelSourceOrderItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(items) != len(s.data.ModelSources) {
		return ErrInvalidInput
	}

	positions := map[string]int{}
	for _, item := range items {
		positions[item.ID] = item.Position
	}

	for i := range s.data.ModelSources {
		position, ok := positions[s.data.ModelSources[i].ID]
		if !ok {
			return ErrInvalidInput
		}
		s.data.ModelSources[i].Position = position
	}

	s.sortModelSourcesLocked()
	return s.persistLocked()
}

func (s *Store) ListSelectedModels() []SelectedModel {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := append([]SelectedModel(nil), s.data.SelectedModels...)
	slices.SortFunc(items, func(a, b SelectedModel) int { return a.Position - b.Position })
	return items
}

func (s *Store) ReplaceSelectedModels(_ context.Context, models []SelectedModel) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, model := range models {
		if strings.TrimSpace(model.ModelID) == "" {
			return ErrInvalidInput
		}
	}

	next := append([]SelectedModel(nil), models...)
	slices.SortFunc(next, func(a, b SelectedModel) int { return a.Position - b.Position })
	for i := range next {
		next[i].Position = i
	}
	s.data.SelectedModels = next
	return s.persistLocked()
}

func (s *Store) ListModels() []ExposedModel {
	s.mu.RLock()
	defer s.mu.RUnlock()

	selected := make([]ExposedModel, 0, len(s.data.SelectedModels))
	seen := map[string]struct{}{}

	for _, item := range s.data.SelectedModels {
		selected = appendIfModelVisible(selected, seen, item.ModelID, s.data.ModelSources)
	}

	if len(selected) > 0 {
		return selected
	}

	fallback := make([]ExposedModel, 0, len(s.data.ModelSources))
	for _, source := range s.data.ModelSources {
		if !source.Enabled || strings.TrimSpace(source.DefaultModelID) == "" {
			continue
		}
		fallback = appendIfModelVisible(fallback, seen, source.DefaultModelID, []ModelSource{source})
	}

	return fallback
}

func (s *Store) ValidateModel(modelID string) error {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return ErrInvalidInput
	}

	for _, model := range s.ListModels() {
		if model.ID == modelID {
			return nil
		}
	}
	return fmt.Errorf("%w: %s", ErrNotFound, modelID)
}

func (s *Store) load() error {
	if err := loadJSONFile(s.statePath, &s.data); err != nil {
		return fmt.Errorf("load state: %w", err)
	}
	if err := loadJSONFile(s.credsPath, &s.credentials); err != nil {
		return fmt.Errorf("load credentials: %w", err)
	}
	return nil
}

func (s *Store) ensureVersionFile() error {
	payload := map[string]any{
		"version":         1,
		"storage_backend": "json-file",
	}
	return writeJSONFile(s.versionPath, payload)
}

func (s *Store) persistLocked() error {
	if err := writeJSONFile(s.statePath, s.data); err != nil {
		return err
	}
	if err := writeJSONFile(s.credsPath, s.credentials); err != nil {
		return err
	}
	return nil
}

func (s *Store) sortModelSourcesLocked() {
	slices.SortFunc(s.data.ModelSources, func(a, b ModelSource) int {
		if a.Position == b.Position {
			return strings.Compare(a.ID, b.ID)
		}
		return a.Position - b.Position
	})
	for i := range s.data.ModelSources {
		s.data.ModelSources[i].Position = i
	}
}

func validateModelSourceRequest(req ModelSourceUpsertRequest) error {
	if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.BaseURL) == "" || strings.TrimSpace(req.ProviderType) == "" || strings.TrimSpace(req.DefaultModelID) == "" {
		return ErrInvalidInput
	}
	return nil
}

func appendIfModelVisible(items []ExposedModel, seen map[string]struct{}, modelID string, sources []ModelSource) []ExposedModel {
	if _, ok := seen[modelID]; ok || strings.TrimSpace(modelID) == "" {
		return items
	}

	ownedBy := "openai-compatible"
	for _, source := range sources {
		if source.DefaultModelID != modelID {
			continue
		}
		if strings.Contains(source.ProviderType, "anthropic") {
			ownedBy = "anthropic-compatible"
		}
		break
	}

	seen[modelID] = struct{}{}
	return append(items, ExposedModel{
		ID:      modelID,
		Object:  "model",
		OwnedBy: ownedBy,
	})
}

func cloneSources(items []ModelSource, keys map[string]string) []ModelSource {
	result := make([]ModelSource, 0, len(items))
	for _, item := range items {
		result = append(result, withCredentialView(item, keys[item.ID]))
	}
	return result
}

func withCredentialView(item ModelSource, apiKey string) ModelSource {
	item.APIKey = ""
	item.APIKeyMasked = maskAPIKey(apiKey)
	return item
}

func maskAPIKey(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if len(value) <= 4 {
		return "****"
	}
	return value[:3] + "-****"
}

func newID(prefix string) string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return prefix + "_fallback"
	}
	return prefix + "_" + hex.EncodeToString(buf)
}

func loadJSONFile(path string, target any) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, target)
}

func writeJSONFile(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

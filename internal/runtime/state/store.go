package state

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	_ "modernc.org/sqlite"

	"github.com/yuanjunliang/ai-mini-gateway/internal/runtime/migration"
)

var (
	ErrNotFound     = errors.New("resource not found")
	ErrConflict     = errors.New("resource conflict")
	ErrInvalidInput = errors.New("invalid input")
)

type Store struct {
	mu          sync.Mutex
	db          *sql.DB
	dbPath      string
	credsPath   string
	credentials credentialsState
}

type credentialsState struct {
	APIKeys map[string]string `json:"api_keys"`
}

type ModelSource struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	BaseURL         string   `json:"base_url"`
	ProviderType    string   `json:"provider_type"`
	DefaultModelID  string   `json:"default_model_id"`
	ExposedModelIDs []string `json:"exposed_model_ids"`
	Enabled         bool     `json:"enabled"`
	Position        int      `json:"position"`
	APIKey          string   `json:"api_key"`
	APIKeyMasked    string   `json:"api_key_masked"`
}

type ModelSourceUpsertRequest struct {
	Name            string   `json:"name"`
	BaseURL         string   `json:"base_url"`
	ProviderType    string   `json:"provider_type"`
	DefaultModelID  string   `json:"default_model_id"`
	ExposedModelIDs []string `json:"exposed_model_ids"`
	Enabled         bool     `json:"enabled"`
	Position        int      `json:"position"`
	APIKey          string   `json:"api_key"`
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

func (s *Store) ListEnabledModelSources() ([]ModelSource, error) {
	sources, err := s.listModelSources(context.Background())
	if err != nil {
		return nil, err
	}

	apiKeys := s.apiKeysSnapshot()
	result := make([]ModelSource, 0, len(sources))
	for _, source := range sources {
		if !source.Enabled {
			continue
		}
		source.APIKey = apiKeys[source.ID]
		result = append(result, source)
	}
	return result, nil
}

func (s *Store) ResolveModelSource(modelID string, providerType string) (ModelSource, error) {
	sources, err := s.ListEnabledModelSources()
	if err != nil {
		return ModelSource{}, err
	}

	for _, source := range sources {
		if source.ProviderType != providerType {
			continue
		}
		if source.DefaultModelID != modelID && !slices.Contains(source.ExposedModelIDs, modelID) {
			continue
		}
		return source, nil
	}

	return ModelSource{}, ErrNotFound
}

func NewStore(dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}

	dbPath := filepath.Join(dataDir, "gateway.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	if err := migration.Apply(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("apply sqlite migration: %w", err)
	}

	store := &Store{
		db:        db,
		dbPath:    dbPath,
		credsPath: filepath.Join(dataDir, "credentials.json"),
		credentials: credentialsState{
			APIKeys: map[string]string{},
		},
	}

	if err := loadJSONFile(store.credsPath, &store.credentials); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("load credentials: %w", err)
	}

	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Ping(ctx context.Context) error {
	if err := s.db.PingContext(ctx); err != nil {
		return err
	}
	_, err := os.Stat(s.dbPath)
	return err
}

func (s *Store) ListModelSources() []ModelSource {
	sources, err := s.listModelSources(context.Background())
	if err != nil {
		return []ModelSource{}
	}
	return sources
}

func (s *Store) CreateModelSource(ctx context.Context, req ModelSourceUpsertRequest) (ModelSource, error) {
	if err := validateModelSourceRequest(req); err != nil {
		return ModelSource{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ModelSource{}, err
	}
	defer tx.Rollback()

	source := ModelSource{
		ID:              newID("src"),
		Name:            req.Name,
		BaseURL:         req.BaseURL,
		ProviderType:    req.ProviderType,
		DefaultModelID:  req.DefaultModelID,
		ExposedModelIDs: sanitizeModelIDs(req.ExposedModelIDs),
		Enabled:         req.Enabled,
	}

	if err := insertModelSource(ctx, tx, source); err != nil {
		return ModelSource{}, err
	}
	if err := replaceExposedModels(ctx, tx, source.ID, source.ExposedModelIDs); err != nil {
		return ModelSource{}, err
	}
	if err := normalizeModelSourcePositions(ctx, tx); err != nil {
		return ModelSource{}, err
	}

	s.credentials.APIKeys[source.ID] = strings.TrimSpace(req.APIKey)
	if err := s.persistCredentialsLocked(); err != nil {
		return ModelSource{}, err
	}

	if err := tx.Commit(); err != nil {
		return ModelSource{}, err
	}

	source, err = s.getModelSource(ctx, source.ID)
	if err != nil {
		return ModelSource{}, err
	}
	return withCredentialView(source, s.credentials.APIKeys[source.ID]), nil
}

func (s *Store) UpdateModelSource(ctx context.Context, id string, req ModelSourceUpsertRequest) (ModelSource, error) {
	if err := validateModelSourceRequest(req); err != nil {
		return ModelSource{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ModelSource{}, err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		UPDATE model_sources
		SET name = ?, base_url = ?, provider_type = ?, default_model_id = ?, enabled = ?
		WHERE id = ?
	`, req.Name, req.BaseURL, req.ProviderType, req.DefaultModelID, boolToInt(req.Enabled), id)
	if err != nil {
		return ModelSource{}, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return ModelSource{}, err
	}
	if rowsAffected == 0 {
		return ModelSource{}, ErrNotFound
	}
	if err := replaceExposedModels(ctx, tx, id, sanitizeModelIDs(req.ExposedModelIDs)); err != nil {
		return ModelSource{}, err
	}

	if apiKey := strings.TrimSpace(req.APIKey); apiKey != "" {
		s.credentials.APIKeys[id] = apiKey
		if err := s.persistCredentialsLocked(); err != nil {
			return ModelSource{}, err
		}
	}

	if err := tx.Commit(); err != nil {
		return ModelSource{}, err
	}

	source, err := s.getModelSource(ctx, id)
	if err != nil {
		return ModelSource{}, err
	}
	return withCredentialView(source, s.credentials.APIKeys[id]), nil
}

func (s *Store) DeleteModelSource(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `DELETE FROM model_sources WHERE id = ?`, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrNotFound
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM model_source_exposed_models WHERE source_id = ?`, id); err != nil {
		return err
	}

	if err := normalizeModelSourcePositions(ctx, tx); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM selected_models
		WHERE model_id NOT IN (
			SELECT default_model_id FROM model_sources WHERE enabled = 1
			UNION
			SELECT esm.model_id
			FROM model_source_exposed_models esm
			JOIN model_sources ms ON ms.id = esm.source_id
			WHERE ms.enabled = 1
		)`); err != nil {
		return err
	}

	delete(s.credentials.APIKeys, id)
	if err := s.persistCredentialsLocked(); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *Store) ReorderModelSources(ctx context.Context, items []ModelSourceOrderItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	current, err := listModelSourcesTx(ctx, s.db)
	if err != nil {
		return err
	}
	if len(items) != len(current) {
		return ErrInvalidInput
	}

	positions := map[string]int{}
	for _, item := range items {
		positions[item.ID] = item.Position
	}
	for _, source := range current {
		if _, ok := positions[source.ID]; !ok {
			return ErrInvalidInput
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, item := range items {
		if _, err := tx.ExecContext(ctx, `UPDATE model_sources SET position = ? WHERE id = ?`, item.Position, item.ID); err != nil {
			return err
		}
	}

	if err := normalizeModelSourcePositions(ctx, tx); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ListSelectedModels() []SelectedModel {
	items, err := s.listSelectedModels(context.Background())
	if err != nil {
		return []SelectedModel{}
	}
	return items
}

func (s *Store) ReplaceSelectedModels(ctx context.Context, models []SelectedModel) error {
	seen := map[string]struct{}{}
	for _, model := range models {
		modelID := strings.TrimSpace(model.ModelID)
		if modelID == "" {
			return ErrInvalidInput
		}
		if _, ok := seen[modelID]; ok {
			return ErrConflict
		}
		seen[modelID] = struct{}{}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	availableModelIDs, err := listAvailableModelIDsTx(ctx, s.db)
	if err != nil {
		return err
	}
	for _, model := range models {
		if _, ok := availableModelIDs[model.ModelID]; !ok {
			return ErrInvalidInput
		}
	}

	next := append([]SelectedModel(nil), models...)
	slices.SortFunc(next, func(a, b SelectedModel) int { return a.Position - b.Position })

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM selected_models`); err != nil {
		return err
	}
	for i, model := range next {
		if _, err := tx.ExecContext(ctx, `INSERT INTO selected_models(model_id, position) VALUES(?, ?)`, model.ModelID, i); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Store) ListModels() []ExposedModel {
	sources, err := s.listModelSources(context.Background())
	if err != nil {
		return []ExposedModel{}
	}
	selected, err := s.listSelectedModels(context.Background())
	if err != nil {
		return []ExposedModel{}
	}

	models := make([]ExposedModel, 0, len(selected))
	seen := map[string]struct{}{}

	for _, item := range selected {
		models = appendIfModelVisible(models, seen, item.ModelID, sources)
	}
	if len(models) > 0 {
		return models
	}

	fallback := make([]ExposedModel, 0, len(sources))
	for _, source := range sources {
		if !source.Enabled {
			continue
		}
		fallback = appendFallbackModels(fallback, seen, source)
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

func (s *Store) getModelSource(ctx context.Context, id string) (ModelSource, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name, base_url, provider_type, default_model_id, enabled, position
		FROM model_sources
		WHERE id = ?
	`, id)

	var source ModelSource
	var enabled int
	err := row.Scan(
		&source.ID,
		&source.Name,
		&source.BaseURL,
		&source.ProviderType,
		&source.DefaultModelID,
		&enabled,
		&source.Position,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return ModelSource{}, ErrNotFound
	}
	if err != nil {
		return ModelSource{}, err
	}

	source.Enabled = enabled != 0
	exposedModelIDs, err := listExposedModels(ctx, s.db, source.ID)
	if err != nil {
		return ModelSource{}, err
	}
	source.ExposedModelIDs = exposedModelIDs
	return source, nil
}

func (s *Store) listModelSources(ctx context.Context) ([]ModelSource, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sources, err := listModelSourcesTx(ctx, s.db)
	if err != nil {
		return nil, err
	}
	return cloneSources(sources, s.credentials.APIKeys), nil
}

func (s *Store) listSelectedModels(ctx context.Context) ([]SelectedModel, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT model_id, position
		FROM selected_models
		ORDER BY position ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []SelectedModel
	for rows.Next() {
		var item SelectedModel
		if err := rows.Scan(&item.ModelID, &item.Position); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func insertModelSource(ctx context.Context, tx *sql.Tx, source ModelSource) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO model_sources(id, name, base_url, provider_type, default_model_id, enabled, position)
		VALUES(?, ?, ?, ?, ?, ?, COALESCE((SELECT MAX(position) + 1 FROM model_sources), 0))
	`, source.ID, source.Name, source.BaseURL, source.ProviderType, source.DefaultModelID, boolToInt(source.Enabled))
	return err
}

func listModelSourcesTx(ctx context.Context, db queryer) ([]ModelSource, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, name, base_url, provider_type, default_model_id, enabled, position
		FROM model_sources
		ORDER BY position ASC, id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []ModelSource
	for rows.Next() {
		var item ModelSource
		var enabled int
		if err := rows.Scan(
			&item.ID,
			&item.Name,
			&item.BaseURL,
			&item.ProviderType,
			&item.DefaultModelID,
			&enabled,
			&item.Position,
		); err != nil {
			return nil, err
		}
		item.Enabled = enabled != 0
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range items {
		exposedModelIDs, err := listExposedModels(ctx, db, items[i].ID)
		if err != nil {
			return nil, err
		}
		items[i].ExposedModelIDs = exposedModelIDs
	}
	return items, nil
}

func normalizeModelSourcePositions(ctx context.Context, tx *sql.Tx) error {
	sources, err := listModelSourcesTx(ctx, tx)
	if err != nil {
		return err
	}
	for i, source := range sources {
		if _, err := tx.ExecContext(ctx, `UPDATE model_sources SET position = ? WHERE id = ?`, i, source.ID); err != nil {
			return err
		}
	}
	return nil
}

func replaceExposedModels(ctx context.Context, tx *sql.Tx, sourceID string, modelIDs []string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM model_source_exposed_models WHERE source_id = ?`, sourceID); err != nil {
		return err
	}
	for i, modelID := range modelIDs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO model_source_exposed_models(source_id, model_id, position) VALUES(?, ?, ?)`, sourceID, modelID, i); err != nil {
			return err
		}
	}
	return nil
}

func listExposedModels(ctx context.Context, db queryer, sourceID string) ([]string, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT model_id
		FROM model_source_exposed_models
		WHERE source_id = ?
		ORDER BY position ASC
	`, sourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var modelIDs []string
	for rows.Next() {
		var modelID string
		if err := rows.Scan(&modelID); err != nil {
			return nil, err
		}
		modelIDs = append(modelIDs, modelID)
	}
	return modelIDs, rows.Err()
}

func listAvailableModelIDsTx(ctx context.Context, db queryer) (map[string]struct{}, error) {
	sources, err := listModelSourcesTx(ctx, db)
	if err != nil {
		return nil, err
	}

	modelIDs := map[string]struct{}{}
	for _, source := range sources {
		if !source.Enabled {
			continue
		}
		if strings.TrimSpace(source.DefaultModelID) != "" {
			modelIDs[source.DefaultModelID] = struct{}{}
		}
		for _, modelID := range source.ExposedModelIDs {
			if strings.TrimSpace(modelID) == "" {
				continue
			}
			modelIDs[modelID] = struct{}{}
		}
	}
	return modelIDs, nil
}

func (s *Store) persistCredentialsLocked() error {
	return writeJSONFile(s.credsPath, s.credentials)
}

func (s *Store) apiKeysSnapshot() map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make(map[string]string, len(s.credentials.APIKeys))
	for key, value := range s.credentials.APIKeys {
		result[key] = value
	}
	return result
}

func validateModelSourceRequest(req ModelSourceUpsertRequest) error {
	if strings.TrimSpace(req.Name) == "" ||
		strings.TrimSpace(req.BaseURL) == "" ||
		strings.TrimSpace(req.ProviderType) == "" ||
		strings.TrimSpace(req.DefaultModelID) == "" {
		return ErrInvalidInput
	}

	parsed, err := url.Parse(strings.TrimSpace(req.BaseURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ErrInvalidInput
	}

	switch strings.TrimSpace(req.ProviderType) {
	case "openai-compatible", "anthropic-compatible":
		seen := map[string]struct{}{}
		for _, modelID := range sanitizeModelIDs(req.ExposedModelIDs) {
			if _, ok := seen[modelID]; ok {
				return ErrConflict
			}
			seen[modelID] = struct{}{}
		}
		return nil
	default:
		return ErrInvalidInput
	}
}

func appendFallbackModels(items []ExposedModel, seen map[string]struct{}, source ModelSource) []ExposedModel {
	if strings.TrimSpace(source.DefaultModelID) != "" {
		items = appendIfModelVisible(items, seen, source.DefaultModelID, []ModelSource{source})
	}
	for _, modelID := range source.ExposedModelIDs {
		items = appendIfModelVisible(items, seen, modelID, []ModelSource{source})
	}
	return items
}

func sanitizeModelIDs(modelIDs []string) []string {
	if len(modelIDs) == 0 {
		return nil
	}
	result := make([]string, 0, len(modelIDs))
	for _, modelID := range modelIDs {
		modelID = strings.TrimSpace(modelID)
		if modelID == "" {
			continue
		}
		result = append(result, modelID)
	}
	return result
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

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

type queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

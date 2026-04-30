package inbound

import (
	"context"

	"github.com/yuanjunliang/ai-mini-gateway/internal/runtime/executor"
	"github.com/yuanjunliang/ai-mini-gateway/internal/runtime/state"
)

func aggregateModels(ctx context.Context, store *state.Store, proxy *executor.Proxy) ([]state.ExposedModel, error) {
	sources, err := store.ListEnabledModelSources()
	if err != nil {
		return nil, err
	}

	models := make([]state.ExposedModel, 0, len(sources))
	seen := map[string]struct{}{}

	for _, source := range sources {
		fetched, err := proxy.FetchModels(ctx, source)
		if err != nil {
			appendFallbackModels(&models, seen, source)
			continue
		}

		for _, model := range fetched {
			if _, ok := seen[model.ID]; ok {
				continue
			}
			if model.ID == "" {
				continue
			}
			if model.Object == "" {
				model.Object = "model"
			}
			if model.OwnedBy == "" {
				model.OwnedBy = source.ProviderType
			}
			seen[model.ID] = struct{}{}
			models = append(models, model)
		}
	}

	selected := store.ListSelectedModels()
	if len(selected) == 0 {
		return models, nil
	}

	indexed := make(map[string]state.ExposedModel, len(models))
	for _, model := range models {
		indexed[model.ID] = model
	}

	filtered := make([]state.ExposedModel, 0, len(selected))
	for _, item := range selected {
		model, ok := indexed[item.ModelID]
		if !ok {
			continue
		}
		filtered = append(filtered, model)
	}

	return filtered, nil
}

func resolveModelSource(ctx context.Context, store *state.Store, proxy *executor.Proxy, modelID string, providerType string) (state.ModelSource, error) {
	if !modelAllowed(store.ListSelectedModels(), modelID) {
		return state.ModelSource{}, state.ErrNotFound
	}

	source, err := store.ResolveModelSource(modelID, providerType)
	if err == nil {
		return source, nil
	}

	sources, listErr := store.ListEnabledModelSources()
	if listErr != nil {
		return state.ModelSource{}, listErr
	}

	for _, candidate := range sources {
		if candidate.ProviderType != providerType {
			continue
		}

		models, fetchErr := proxy.FetchModels(ctx, candidate)
		if fetchErr != nil {
			continue
		}
		for _, model := range models {
			if model.ID == modelID {
				return candidate, nil
			}
		}
	}

	return state.ModelSource{}, err
}

func appendFallbackModels(models *[]state.ExposedModel, seen map[string]struct{}, source state.ModelSource) {
	appendIfAbsent := func(modelID string) {
		if modelID == "" {
			return
		}
		if _, ok := seen[modelID]; ok {
			return
		}
		seen[modelID] = struct{}{}
		*models = append(*models, state.ExposedModel{
			ID:      modelID,
			Object:  "model",
			OwnedBy: source.ProviderType,
		})
	}

	appendIfAbsent(source.DefaultModelID)
	for _, modelID := range source.ExposedModelIDs {
		appendIfAbsent(modelID)
	}
}

func modelAllowed(selected []state.SelectedModel, modelID string) bool {
	if len(selected) == 0 {
		return true
	}
	for _, item := range selected {
		if item.ModelID == modelID {
			return true
		}
	}
	return false
}

package admin

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/yuanjunliang/ai-mini-gateway/internal/runtime/executor"
	"github.com/yuanjunliang/ai-mini-gateway/internal/runtime/state"
	"github.com/yuanjunliang/ai-mini-gateway/internal/runtime/web"
)

func Register(mux *http.ServeMux, store *state.Store, proxy *executor.Proxy) {
	mux.HandleFunc("GET /admin/model-sources", func(w http.ResponseWriter, r *http.Request) {
		web.WriteJSON(w, http.StatusOK, store.ListModelSources())
	})

	mux.HandleFunc("POST /admin/model-sources", func(w http.ResponseWriter, r *http.Request) {
		var req state.ModelSourceUpsertRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			web.WriteError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}

		source, err := store.CreateModelSource(r.Context(), req)
		if err != nil {
			writeStoreError(w, err)
			return
		}

		proxy.InvalidateModelsCache()
		web.WriteJSON(w, http.StatusCreated, source)
	})

	mux.HandleFunc("PUT /admin/model-sources/order", func(w http.ResponseWriter, r *http.Request) {
		var req []state.ModelSourceOrderItem
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			web.WriteError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}

		if err := store.ReorderModelSources(r.Context(), req); err != nil {
			writeStoreError(w, err)
			return
		}

		proxy.InvalidateModelsCache()
		web.WriteJSON(w, http.StatusOK, store.ListModelSources())
	})

	mux.HandleFunc("/admin/model-sources/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/admin/model-sources/")
		if id == "" {
			web.WriteError(w, http.StatusNotFound, "not_found", "model source id is required")
			return
		}

		switch r.Method {
		case http.MethodPut:
			var req state.ModelSourceUpsertRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				web.WriteError(w, http.StatusBadRequest, "invalid_json", err.Error())
				return
			}

			source, err := store.UpdateModelSource(r.Context(), id, req)
			if err != nil {
				writeStoreError(w, err)
				return
			}

			proxy.InvalidateModelsCache(id)
			web.WriteJSON(w, http.StatusOK, source)
		case http.MethodDelete:
			if err := store.DeleteModelSource(r.Context(), id); err != nil {
				writeStoreError(w, err)
				return
			}

			proxy.InvalidateModelsCache(id)
			w.WriteHeader(http.StatusNoContent)
		default:
			web.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "unsupported method")
		}
	})

	mux.HandleFunc("GET /admin/selected-models", func(w http.ResponseWriter, r *http.Request) {
		web.WriteJSON(w, http.StatusOK, store.ListSelectedModels())
	})

	mux.HandleFunc("PUT /admin/selected-models", func(w http.ResponseWriter, r *http.Request) {
		var req []state.SelectedModel
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			web.WriteError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}

		if err := store.ReplaceSelectedModels(r.Context(), req); err != nil {
			writeStoreError(w, err)
			return
		}

		web.WriteJSON(w, http.StatusOK, store.ListSelectedModels())
	})
}

func writeStoreError(w http.ResponseWriter, err error) {
	switch {
	case err == state.ErrNotFound:
		web.WriteError(w, http.StatusNotFound, "not_found", err.Error())
	case err == state.ErrConflict:
		web.WriteError(w, http.StatusConflict, "conflict", err.Error())
	case err == state.ErrInvalidInput:
		web.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error())
	default:
		web.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
	}
}

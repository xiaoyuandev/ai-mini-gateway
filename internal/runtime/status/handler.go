package status

import (
	"net/http"

	"github.com/yuanjunliang/ai-mini-gateway/internal/runtime/buildinfo"
	"github.com/yuanjunliang/ai-mini-gateway/internal/runtime/state"
	"github.com/yuanjunliang/ai-mini-gateway/internal/runtime/web"
)

func Register(mux *http.ServeMux, store *state.Store, info buildinfo.Info) {
	mux.HandleFunc("GET /runtime/status", func(w http.ResponseWriter, r *http.Request) {
		lastAppliedAt, err := store.GetLastAppliedAt(r.Context())
		if err != nil {
			web.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
		lastSyncError, err := store.GetLastSyncError(r.Context())
		if err != nil {
			web.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}

		payload := map[string]any{
			"runtime_kind":     info.RuntimeKind,
			"status":           "ok",
			"version":          info.Version,
			"commit":           info.Commit,
			"host":             info.Host,
			"port":             info.Port,
			"data_dir":         info.DataDir,
			"last_applied_at":  lastAppliedAt,
			"sync_in_progress": store.IsRuntimeSyncInProgress(),
			"last_sync_error":  lastSyncError,
		}

		if err := store.Ping(r.Context()); err != nil {
			payload["status"] = "error"
			payload["message"] = err.Error()
		}

		web.WriteJSON(w, http.StatusOK, payload)
	})
}

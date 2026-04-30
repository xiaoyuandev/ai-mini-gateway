package health

import (
	"net/http"

	"github.com/yuanjunliang/ai-mini-gateway/internal/runtime/state"
	"github.com/yuanjunliang/ai-mini-gateway/internal/runtime/web"
)

func Register(mux *http.ServeMux, store *state.Store) {
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		if err := store.Ping(r.Context()); err != nil {
			web.WriteJSON(w, http.StatusServiceUnavailable, map[string]string{
				"status":  "error",
				"message": err.Error(),
			})
			return
		}

		web.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
}

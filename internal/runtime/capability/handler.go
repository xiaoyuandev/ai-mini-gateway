package capability

import (
	"net/http"

	"github.com/yuanjunliang/ai-mini-gateway/internal/runtime/web"
)

func Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /capabilities", func(w http.ResponseWriter, r *http.Request) {
		web.WriteJSON(w, http.StatusOK, map[string]any{
			"supports_openai_compatible":    true,
			"supports_anthropic_compatible": true,
			"supports_models_api":           true,
			"supports_stream":               true,
			"supports_admin_api":            true,
			"supports_model_source_admin":   true,
			"supports_selected_model_admin": true,
		})
	})
}

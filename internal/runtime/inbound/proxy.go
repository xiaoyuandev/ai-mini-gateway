package inbound

import (
	"encoding/json"
	"net/http"

	"github.com/yuanjunliang/ai-mini-gateway/internal/runtime/providers"
	"github.com/yuanjunliang/ai-mini-gateway/internal/runtime/web"
)

func writeProviderResponse(w http.ResponseWriter, provider providers.Provider, operation providers.Operation, resp *http.Response) {
	classification := provider.ClassifyResponse(operation, resp)
	if classification.IsErrorJSON {
		defer resp.Body.Close()

		var payload map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&payload); err == nil {
			normalized := provider.NormalizeUpstreamError(operation, resp.StatusCode, payload)
			web.WriteError(w, resp.StatusCode, normalized.Code, normalized.Message)
			return
		}
	}

	web.WriteProxyResponse(w, resp)
}

package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/yuanjunliang/ai-mini-gateway/internal/runtime/state"
)

type Proxy struct {
	client *http.Client
}

type modelsEnvelope struct {
	Data []state.ExposedModel `json:"data"`
}

func NewProxy() *Proxy {
	return NewProxyWithClient(&http.Client{Timeout: 5 * time.Minute})
}

func NewProxyWithClient(client *http.Client) *Proxy {
	return &Proxy{
		client: client,
	}
}

func (p *Proxy) Forward(ctx context.Context, source state.ModelSource, path string, incomingHeader http.Header, body []byte) (*http.Response, error) {
	req, err := newUpstreamRequest(ctx, http.MethodPost, source, path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	copyHeader(req.Header, incomingHeader)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upstream_request_failed: %w", err)
	}
	return resp, nil
}

func (p *Proxy) FetchModels(ctx context.Context, source state.ModelSource) ([]state.ExposedModel, error) {
	req, err := newUpstreamRequest(ctx, http.MethodGet, source, "/models", nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upstream_models_failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("upstream_models_failed: status=%d", resp.StatusCode)
	}

	var payload modelsEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("upstream_models_invalid: %w", err)
	}
	return payload.Data, nil
}

func newUpstreamRequest(ctx context.Context, method string, source state.ModelSource, path string, body *bytes.Reader) (*http.Request, error) {
	url := strings.TrimRight(source.BaseURL, "/") + path
	var reader httpBody
	if body != nil {
		reader = body
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return nil, err
	}
	applyAuthHeader(req.Header, source)
	return req, nil
}

func copyHeader(dst http.Header, src http.Header) {
	for key, values := range src {
		switch http.CanonicalHeaderKey(key) {
		case "Host", "Content-Length":
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func applyAuthHeader(header http.Header, source state.ModelSource) {
	if source.APIKey == "" {
		return
	}

	if header.Get("Authorization") == "" && source.ProviderType == "openai-compatible" {
		header.Set("Authorization", "Bearer "+source.APIKey)
	}
	if header.Get("x-api-key") == "" && source.ProviderType == "anthropic-compatible" {
		header.Set("x-api-key", source.APIKey)
	}
}

type httpBody interface {
	Read(p []byte) (n int, err error)
}

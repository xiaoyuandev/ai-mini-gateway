package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/yuanjunliang/ai-mini-gateway/internal/runtime/state"
)

type Proxy struct {
	client    *http.Client
	modelsTTL time.Duration
	now       func() time.Time
	mu        sync.Mutex
	cache     map[string]modelsCacheEntry
}

var ErrModelsUnsupported = errors.New("models_api_unsupported")

type modelsEnvelope struct {
	Data []state.ExposedModel `json:"data"`
}

type modelsCacheEntry struct {
	models      []state.ExposedModel
	err         string
	unsupported bool
	expiresAt   time.Time
}

func NewProxy() *Proxy {
	return NewProxyWithClient(&http.Client{Timeout: 5 * time.Minute})
}

func NewProxyWithClient(client *http.Client) *Proxy {
	return NewProxyWithClientAndTTL(client, 15*time.Second)
}

func NewProxyWithClientAndTTL(client *http.Client, ttl time.Duration) *Proxy {
	return &Proxy{
		client:    client,
		modelsTTL: ttl,
		now:       time.Now,
		cache:     map[string]modelsCacheEntry{},
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
	if models, err, ok := p.getCachedModels(source.ID); ok {
		return models, err
	}

	req, err := newUpstreamRequest(ctx, http.MethodGet, source, "/models", nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.client.Do(req)
	if err != nil {
		cachedErr := fmt.Errorf("upstream_models_failed: %w", err)
		p.setCachedModels(source.ID, nil, cachedErr)
		return nil, cachedErr
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if isModelsUnsupportedStatus(resp.StatusCode) {
			p.setUnsupportedModels(source.ID)
			return nil, ErrModelsUnsupported
		}
		cachedErr := fmt.Errorf("upstream_models_failed: status=%d", resp.StatusCode)
		p.setCachedModels(source.ID, nil, cachedErr)
		return nil, cachedErr
	}

	var payload modelsEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		cachedErr := fmt.Errorf("upstream_models_invalid: %w", err)
		p.setCachedModels(source.ID, nil, cachedErr)
		return nil, cachedErr
	}
	p.setCachedModels(source.ID, payload.Data, nil)
	return cloneModels(payload.Data), nil
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

func (p *Proxy) getCachedModels(sourceID string) ([]state.ExposedModel, error, bool) {
	if p.modelsTTL <= 0 {
		return nil, nil, false
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	entry, ok := p.cache[sourceID]
	if !ok || p.now().After(entry.expiresAt) {
		if ok && p.now().After(entry.expiresAt) {
			delete(p.cache, sourceID)
		}
		return nil, nil, false
	}

	if entry.err != "" {
		return nil, fmt.Errorf("%s", entry.err), true
	}
	if entry.unsupported {
		return nil, ErrModelsUnsupported, true
	}
	return cloneModels(entry.models), nil, true
}

func (p *Proxy) setCachedModels(sourceID string, models []state.ExposedModel, err error) {
	if p.modelsTTL <= 0 {
		return
	}

	entry := modelsCacheEntry{
		models:    cloneModels(models),
		expiresAt: p.now().Add(p.modelsTTL),
	}
	if err != nil {
		entry.err = err.Error()
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.cache[sourceID] = entry
}

func (p *Proxy) setUnsupportedModels(sourceID string) {
	if p.modelsTTL <= 0 {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.cache[sourceID] = modelsCacheEntry{
		unsupported: true,
		expiresAt:   p.now().Add(p.modelsTTL),
	}
}

func cloneModels(models []state.ExposedModel) []state.ExposedModel {
	if len(models) == 0 {
		return nil
	}
	cloned := make([]state.ExposedModel, len(models))
	copy(cloned, models)
	return cloned
}

func (p *Proxy) InvalidateModelsCache(sourceIDs ...string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(sourceIDs) == 0 {
		clear(p.cache)
		return
	}

	for _, sourceID := range sourceIDs {
		delete(p.cache, sourceID)
	}
}

func isModelsUnsupportedStatus(status int) bool {
	switch status {
	case http.StatusNotFound, http.StatusMethodNotAllowed, http.StatusNotImplemented:
		return true
	default:
		return false
	}
}

type httpBody interface {
	Read(p []byte) (n int, err error)
}

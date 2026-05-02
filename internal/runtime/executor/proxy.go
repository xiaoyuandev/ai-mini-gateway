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

	"github.com/yuanjunliang/ai-mini-gateway/internal/runtime/providers"
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

type HealthcheckResult struct {
	Status     string `json:"status"`
	StatusCode int    `json:"status_code"`
	LatencyMS  int64  `json:"latency_ms"`
	Summary    string `json:"summary"`
	CheckedAt  string `json:"checked_at"`
}

type modelsEnvelope struct {
	Data []state.ExposedModel `json:"data"`
}

type modelsCacheEntry struct {
	models          []state.ExposedModel
	err             string
	unsupported     bool
	operationStatus map[providers.Operation]string
	streamStatus    string
	expiresAt       time.Time
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
	provider := providers.ForSource(source)
	req, err := newUpstreamRequest(ctx, http.MethodPost, source, path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	copyHeader(req.Header, incomingHeader, provider)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upstream_request_failed: %w", err)
	}
	return resp, nil
}

func (p *Proxy) ForwardOperation(ctx context.Context, source state.ModelSource, operation providers.Operation, incomingHeader http.Header, body []byte) (*http.Response, error) {
	provider := providers.ForSource(source)
	resp, err := p.Forward(ctx, source, provider.PathForOperation(operation), incomingHeader, body)
	if err != nil {
		p.setOperationStatus(source.ID, operation, "error")
		return nil, err
	}

	switch {
	case provider.IsOperationUnsupported(operation, resp.StatusCode):
		p.setOperationStatus(source.ID, operation, "unsupported")
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		p.setOperationStatus(source.ID, operation, "supported")
	default:
		p.setOperationStatus(source.ID, operation, "error")
	}

	if requestAsksForStream(body) {
		switch {
		case resp.StatusCode >= 200 && resp.StatusCode < 300 && strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream"):
			p.setStreamStatus(source.ID, "supported")
		case provider.IsOperationUnsupported(operation, resp.StatusCode):
			p.setStreamStatus(source.ID, "unsupported")
		default:
			p.setStreamStatus(source.ID, "error")
		}
	}

	return resp, nil
}

func (p *Proxy) HealthcheckSource(ctx context.Context, source state.ModelSource) HealthcheckResult {
	start := p.now()
	status := "error"
	statusCode := 0
	summary := "healthcheck_failed"

	resp, err := p.doHealthcheck(ctx, source)
	if err != nil {
		summary = err.Error()
		return HealthcheckResult{
			Status:     status,
			StatusCode: statusCode,
			LatencyMS:  p.now().Sub(start).Milliseconds(),
			Summary:    summary,
			CheckedAt:  p.now().UTC().Format(time.RFC3339),
		}
	}
	defer resp.Body.Close()

	statusCode = resp.StatusCode
	summary = fmt.Sprintf("HTTP %d", resp.StatusCode)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		status = "ok"
	}

	return HealthcheckResult{
		Status:     status,
		StatusCode: statusCode,
		LatencyMS:  p.now().Sub(start).Milliseconds(),
		Summary:    summary,
		CheckedAt:  p.now().UTC().Format(time.RFC3339),
	}
}

func (p *Proxy) FetchModels(ctx context.Context, source state.ModelSource) ([]state.ExposedModel, error) {
	if models, err, ok := p.getCachedModels(source.ID); ok {
		return models, err
	}

	req, err := newUpstreamRequest(ctx, http.MethodGet, source, providers.ForSource(source).PathForOperation(providers.OperationModels), nil)
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
		if providers.ForSource(source).IsOperationUnsupported(providers.OperationModels, resp.StatusCode) {
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

func (p *Proxy) doHealthcheck(ctx context.Context, source state.ModelSource) (*http.Response, error) {
	resp, unsupported, err := p.healthcheckModels(ctx, source)
	if err == nil && !unsupported {
		return resp, nil
	}
	if resp != nil {
		resp.Body.Close()
	}
	if source.ProviderType != "anthropic-compatible" {
		return nil, err
	}

	fallbackResp, fallbackErr := p.healthcheckAnthropicCountTokens(ctx, source)
	if fallbackErr != nil {
		if err != nil {
			return nil, err
		}
		return nil, fallbackErr
	}
	return fallbackResp, nil
}

func (p *Proxy) healthcheckModels(ctx context.Context, source state.ModelSource) (*http.Response, bool, error) {
	req, err := newUpstreamRequest(ctx, http.MethodGet, source, providers.ForSource(source).PathForOperation(providers.OperationModels), nil)
	if err != nil {
		return nil, false, err
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("upstream_request_failed: %w", err)
	}

	if providers.ForSource(source).IsOperationUnsupported(providers.OperationModels, resp.StatusCode) {
		return resp, true, nil
	}
	return resp, false, nil
}

func (p *Proxy) healthcheckAnthropicCountTokens(ctx context.Context, source state.ModelSource) (*http.Response, error) {
	body := []byte(fmt.Sprintf(`{"model":%q,"messages":[{"role":"user","content":"healthcheck"}]}`, source.DefaultModelID))
	req, err := newUpstreamRequest(ctx, http.MethodPost, source, providers.ForSource(source).PathForOperation(providers.OperationAnthropicCountTokens), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upstream_request_failed: %w", err)
	}
	return resp, nil
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
	providers.ForSource(source).ApplyAuthHeader(req.Header, source)
	return req, nil
}

func copyHeader(dst http.Header, src http.Header, provider providers.Provider) {
	for key, values := range src {
		if !provider.ShouldForwardHeader(key) {
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
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
		models:          cloneModels(models),
		operationStatus: p.currentOperationStatus(sourceID),
		streamStatus:    p.currentStreamStatus(sourceID),
		expiresAt:       p.now().Add(p.modelsTTL),
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
		unsupported:     true,
		operationStatus: p.currentOperationStatusLocked(sourceID),
		streamStatus:    p.currentStreamStatusLocked(sourceID),
		expiresAt:       p.now().Add(p.modelsTTL),
	}
}

func (p *Proxy) GetSourceCapabilities(source state.ModelSource) providers.Capabilities {
	capabilities := providers.ForSource(source).DefaultCapabilities()

	p.mu.Lock()
	defer p.mu.Unlock()

	entry, ok := p.cache[source.ID]
	if !ok || p.now().After(entry.expiresAt) {
		return capabilities
	}

	switch {
	case entry.unsupported:
		capabilities.SupportsModelsAPI = false
		capabilities.ModelsAPIStatus = "unsupported"
	case entry.err != "":
		capabilities.ModelsAPIStatus = "error"
	default:
		capabilities.ModelsAPIStatus = "supported"
	}

	applyObservedOperationStatuses(&capabilities, entry.operationStatus)
	if entry.streamStatus != "" {
		capabilities.StreamStatus = entry.streamStatus
		capabilities.SupportsStream = entry.streamStatus != "unsupported"
	}
	return capabilities
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

func (p *Proxy) setOperationStatus(sourceID string, operation providers.Operation, status string) {
	if p.modelsTTL <= 0 {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	entry := p.cache[sourceID]
	if entry.operationStatus == nil {
		entry.operationStatus = map[providers.Operation]string{}
	}
	entry.operationStatus[operation] = status
	entry.expiresAt = p.now().Add(p.modelsTTL)
	p.cache[sourceID] = entry
}

func (p *Proxy) currentOperationStatus(sourceID string) map[providers.Operation]string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.currentOperationStatusLocked(sourceID)
}

func (p *Proxy) currentOperationStatusLocked(sourceID string) map[providers.Operation]string {
	entry, ok := p.cache[sourceID]
	if !ok || len(entry.operationStatus) == 0 {
		return nil
	}
	cloned := make(map[providers.Operation]string, len(entry.operationStatus))
	for key, value := range entry.operationStatus {
		cloned[key] = value
	}
	return cloned
}

func (p *Proxy) setStreamStatus(sourceID string, status string) {
	if p.modelsTTL <= 0 {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	entry := p.cache[sourceID]
	entry.streamStatus = status
	entry.expiresAt = p.now().Add(p.modelsTTL)
	p.cache[sourceID] = entry
}

func (p *Proxy) currentStreamStatus(sourceID string) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.currentStreamStatusLocked(sourceID)
}

func (p *Proxy) currentStreamStatusLocked(sourceID string) string {
	entry, ok := p.cache[sourceID]
	if !ok {
		return ""
	}
	return entry.streamStatus
}

func applyObservedOperationStatuses(capabilities *providers.Capabilities, statuses map[providers.Operation]string) {
	for operation, status := range statuses {
		switch operation {
		case providers.OperationOpenAIChatCompletions:
			capabilities.OpenAIChatCompletionsStatus = status
			capabilities.SupportsOpenAIChatCompletions = status != "unsupported"
		case providers.OperationOpenAIResponses:
			capabilities.OpenAIResponsesStatus = status
			capabilities.SupportsOpenAIResponses = status != "unsupported"
		case providers.OperationAnthropicMessages:
			capabilities.AnthropicMessagesStatus = status
			capabilities.SupportsAnthropicMessages = status != "unsupported"
			if status == "supported" {
				capabilities.StreamStatus = "supported"
			}
		case providers.OperationAnthropicCountTokens:
			capabilities.AnthropicCountTokensStatus = status
			capabilities.SupportsAnthropicCountTokens = status != "unsupported"
		}
	}

	if statuses[providers.OperationOpenAIChatCompletions] == "supported" || statuses[providers.OperationOpenAIResponses] == "supported" {
		capabilities.StreamStatus = "supported"
	}
}

func requestAsksForStream(body []byte) bool {
	if len(body) == 0 {
		return false
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return false
	}
	stream, _ := payload["stream"].(bool)
	return stream
}

type httpBody interface {
	Read(p []byte) (n int, err error)
}

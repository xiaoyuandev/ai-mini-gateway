package executor

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/yuanjunliang/ai-mini-gateway/internal/runtime/state"
)

type Proxy struct {
	client *http.Client
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
	url := strings.TrimRight(source.BaseURL, "/") + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	copyHeader(req.Header, incomingHeader)
	applyAuthHeader(req.Header, source)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upstream_request_failed: %w", err)
	}
	return resp, nil
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
	if source.APIKeyMasked == "" {
		return
	}

	if header.Get("Authorization") == "" && source.ProviderType == "openai-compatible" {
		header.Set("Authorization", "Bearer "+source.APIKey)
	}
	if header.Get("x-api-key") == "" && source.ProviderType == "anthropic-compatible" {
		header.Set("x-api-key", source.APIKey)
	}
}

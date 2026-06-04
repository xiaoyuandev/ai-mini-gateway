package inbound

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/klauspost/compress/zstd"
)

func TestDecodeOpenAIResponseRequestZstdEncodedClearsHeader(t *testing.T) {
	encoder, err := zstd.NewWriter(nil)
	if err != nil {
		t.Fatalf("create zstd encoder: %v", err)
	}
	defer encoder.Close()

	compressed := encoder.EncodeAll([]byte(`{"model":"deepseek-v4-flash","input":"hello"}`), nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(compressed))
	req.Header.Set("Content-Encoding", "zstd")

	body, payload, err := decodeOpenAIResponseRequest(req)
	if err != nil {
		t.Fatalf("decode request: %v", err)
	}
	if payload.Model != "deepseek-v4-flash" {
		t.Fatalf("unexpected model: %+v", payload)
	}
	if string(body) != `{"model":"deepseek-v4-flash","input":"hello"}` {
		t.Fatalf("unexpected body: %s", body)
	}
	if got := req.Header.Get("Content-Encoding"); got != "" {
		t.Fatalf("expected content encoding to be cleared, got %q", got)
	}
	if got := req.Header.Get("Content-Length"); got != strconv.Itoa(len(body)) {
		t.Fatalf("unexpected content length header: %q", got)
	}
	forwarded, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("read reset body: %v", err)
	}
	if string(forwarded) != string(body) {
		t.Fatalf("unexpected reset body: %s", forwarded)
	}
}

func TestDecodeRequestBodyGzip(t *testing.T) {
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	_, _ = writer.Write([]byte(`{"model":"gpt-4.1-mini","messages":[]}`))
	if err := writer.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}

	body, err := decodeRequestBody(compressed.Bytes(), "gzip")
	if err != nil {
		t.Fatalf("decode gzip body: %v", err)
	}
	if string(body) != `{"model":"gpt-4.1-mini","messages":[]}` {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestDecodeRequestBodyRejectsUnsupportedEncoding(t *testing.T) {
	if _, err := decodeRequestBody([]byte("{}"), "br"); err == nil {
		t.Fatal("expected unsupported encoding error")
	}
}

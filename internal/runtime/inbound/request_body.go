package inbound

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/klauspost/compress/zstd"
)

func readDecodedRequestBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}

	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	body, err := decodeRequestBody(rawBody, r.Header.Get("Content-Encoding"))
	if err != nil {
		return nil, err
	}

	r.Body = io.NopCloser(bytes.NewReader(body))
	r.ContentLength = int64(len(body))
	r.Header.Del("Content-Encoding")
	r.Header.Set("Content-Length", strconv.Itoa(len(body)))
	return body, nil
}

func decodeRequestBody(body []byte, contentEncoding string) ([]byte, error) {
	encodings := contentEncodings(contentEncoding)
	if len(encodings) == 0 {
		return body, nil
	}

	decoded := body
	for index := len(encodings) - 1; index >= 0; index-- {
		switch encoding := encodings[index]; encoding {
		case "", "identity":
			continue
		case "gzip", "x-gzip":
			reader, err := gzip.NewReader(bytes.NewReader(decoded))
			if err != nil {
				return nil, fmt.Errorf("decode gzip request body: %w", err)
			}
			next, err := io.ReadAll(reader)
			closeErr := reader.Close()
			if err != nil {
				return nil, fmt.Errorf("read gzip request body: %w", err)
			}
			if closeErr != nil {
				return nil, fmt.Errorf("close gzip request body: %w", closeErr)
			}
			decoded = next
		case "zstd", "zstandard":
			reader, err := zstd.NewReader(bytes.NewReader(decoded))
			if err != nil {
				return nil, fmt.Errorf("decode zstd request body: %w", err)
			}
			next, err := io.ReadAll(reader)
			reader.Close()
			if err != nil {
				return nil, fmt.Errorf("read zstd request body: %w", err)
			}
			decoded = next
		default:
			return nil, fmt.Errorf("unsupported request content encoding: %s", encoding)
		}
	}

	return decoded, nil
}

func contentEncodings(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}

	parts := strings.Split(value, ",")
	encodings := make([]string, 0, len(parts))
	for _, part := range parts {
		encoding := strings.ToLower(strings.TrimSpace(part))
		if encoding == "" || encoding == "identity" {
			continue
		}
		encodings = append(encodings, encoding)
	}
	return encodings
}

package web

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func WriteJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func WriteError(w http.ResponseWriter, status int, code string, message string) {
	WriteJSON(w, status, map[string]string{
		"error":   code,
		"message": message,
	})
}

func WriteSSEHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
}

func WriteSSE(w http.ResponseWriter, payload any) {
	data, _ := json.Marshal(payload)
	_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

func WriteNamedSSE(w http.ResponseWriter, event string, payload any) {
	data, _ := json.Marshal(payload)
	_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

func WriteDoneSSE(w http.ResponseWriter) {
	_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

func WriteProxyResponse(w http.ResponseWriter, resp *http.Response) {
	defer resp.Body.Close()

	status := resp.StatusCode
	header := resp.Header
	for key, values := range header {
		switch http.CanonicalHeaderKey(key) {
		case "Content-Length", "Transfer-Encoding", "Connection":
			continue
		}
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(status)

	reader := bufio.NewReader(resp.Body)
	buffer := make([]byte, 32*1024)
	for {
		n, err := reader.Read(buffer)
		if n > 0 {
			_, _ = w.Write(buffer[:n])
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}
		if err == nil {
			continue
		}
		if err == io.EOF {
			return
		}
		return
	}
}

func WriteProxyOrError(w http.ResponseWriter, resp *http.Response) bool {
	contentType := resp.Header.Get("Content-Type")
	if resp.StatusCode >= 400 && strings.Contains(contentType, "application/json") {
		defer resp.Body.Close()

		var payload map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&payload); err == nil {
			errorCode := "upstream_request_failed"
			if value, ok := payload["error"].(string); ok && strings.TrimSpace(value) != "" {
				errorCode = value
			}
			message := http.StatusText(resp.StatusCode)
			if value, ok := payload["message"].(string); ok && strings.TrimSpace(value) != "" {
				message = value
			}
			WriteError(w, resp.StatusCode, errorCode, message)
			return true
		}
	}

	WriteProxyResponse(w, resp)
	return false
}

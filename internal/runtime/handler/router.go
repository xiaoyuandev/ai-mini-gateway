package handler

import (
	"log"
	"net/http"
	"time"

	"github.com/yuanjunliang/ai-mini-gateway/internal/runtime/admin"
	"github.com/yuanjunliang/ai-mini-gateway/internal/runtime/health"
	"github.com/yuanjunliang/ai-mini-gateway/internal/runtime/inbound"
	"github.com/yuanjunliang/ai-mini-gateway/internal/runtime/state"
)

func NewRouter(store *state.Store) http.Handler {
	mux := http.NewServeMux()

	health.Register(mux, store)
	inbound.RegisterOpenAI(mux, store)
	inbound.RegisterAnthropic(mux, store)
	admin.Register(mux, store)

	return loggingMiddleware(mux)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
	})
}

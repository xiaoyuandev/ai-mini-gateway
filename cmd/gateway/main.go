package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yuanjunliang/ai-mini-gateway/internal/runtime/buildinfo"
	"github.com/yuanjunliang/ai-mini-gateway/internal/runtime/config"
	"github.com/yuanjunliang/ai-mini-gateway/internal/runtime/executor"
	"github.com/yuanjunliang/ai-mini-gateway/internal/runtime/handler"
	"github.com/yuanjunliang/ai-mini-gateway/internal/runtime/state"
)

var (
	version = "dev"
	commit  = "unknown"
)

func main() {
	cfg := config.FromFlags()

	store, err := state.NewStore(cfg.DataDir)
	if err != nil {
		log.Fatalf("initialize runtime state: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			log.Printf("close runtime state: %v", err)
		}
	}()

	proxy := executor.NewProxyWithClientAndTTL(&http.Client{Timeout: 5 * time.Minute}, cfg.ModelsCacheTTL)

	srv := &http.Server{
		Addr: cfg.Address(),
		Handler: handler.NewRouterWithProxyAndInfo(store, proxy, buildinfo.Info{
			RuntimeKind: "ai-mini-gateway",
			Version:     version,
			Commit:      commit,
		}),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("shutdown server: %v", err)
		}
	}()

	log.Printf("ai-mini-gateway listening on %s data_dir=%s", cfg.Address(), cfg.DataDir)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("serve gateway: %v", err)
	}
}

func init() {
	flag.CommandLine.SetOutput(os.Stdout)
}

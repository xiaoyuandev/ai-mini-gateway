package config

import (
	"reflect"
	"testing"
	"time"
)

func TestFromArgsDefaults(t *testing.T) {
	cfg := FromArgs(nil, func(string) (string, bool) {
		return "", false
	})

	want := Config{
		Host:           "127.0.0.1",
		Port:           3457,
		DataDir:        "./data",
		ModelsCacheTTL: 15 * time.Second,
	}

	if !reflect.DeepEqual(cfg, want) {
		t.Fatalf("unexpected config: got %+v want %+v", cfg, want)
	}
}

func TestFromArgsUsesEmbeddedEnv(t *testing.T) {
	env := map[string]string{
		"LOCAL_GATEWAY_RUNTIME_HOST": "0.0.0.0",
		"LOCAL_GATEWAY_RUNTIME_PORT": "4567",
		"CORE_DATA_DIR":              "/tmp/gateway-data",
	}

	cfg := FromArgs(nil, func(key string) (string, bool) {
		value, ok := env[key]
		return value, ok
	})

	if cfg.Host != "0.0.0.0" {
		t.Fatalf("unexpected host: %s", cfg.Host)
	}
	if cfg.Port != 4567 {
		t.Fatalf("unexpected port: %d", cfg.Port)
	}
	if cfg.DataDir != "/tmp/gateway-data" {
		t.Fatalf("unexpected data dir: %s", cfg.DataDir)
	}
}

func TestFromArgsFlagsOverrideEmbeddedEnv(t *testing.T) {
	env := map[string]string{
		"LOCAL_GATEWAY_RUNTIME_HOST": "0.0.0.0",
		"LOCAL_GATEWAY_RUNTIME_PORT": "4567",
		"CORE_DATA_DIR":              "/tmp/gateway-data",
	}

	cfg := FromArgs([]string{
		"--host", "127.0.0.1",
		"--port", "3457",
		"--data-dir", "./override-data",
		"--models-cache-ttl", "30s",
	}, func(key string) (string, bool) {
		value, ok := env[key]
		return value, ok
	})

	if cfg.Host != "127.0.0.1" {
		t.Fatalf("unexpected host: %s", cfg.Host)
	}
	if cfg.Port != 3457 {
		t.Fatalf("unexpected port: %d", cfg.Port)
	}
	if cfg.DataDir != "./override-data" {
		t.Fatalf("unexpected data dir: %s", cfg.DataDir)
	}
	if cfg.ModelsCacheTTL != 30*time.Second {
		t.Fatalf("unexpected models cache ttl: %s", cfg.ModelsCacheTTL)
	}
}

func TestFromArgsIgnoresInvalidEmbeddedPort(t *testing.T) {
	cfg := FromArgs(nil, func(key string) (string, bool) {
		if key == "LOCAL_GATEWAY_RUNTIME_PORT" {
			return "invalid", true
		}
		return "", false
	})

	if cfg.Port != 3457 {
		t.Fatalf("unexpected port: %d", cfg.Port)
	}
}

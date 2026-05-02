package config

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Host           string
	Port           int
	DataDir        string
	ModelsCacheTTL time.Duration
}

func FromFlags() Config {
	return FromArgs(os.Args[1:], os.LookupEnv)
}

func FromArgs(args []string, lookupEnv func(string) (string, bool)) Config {
	cfg := Config{}
	fs := flag.NewFlagSet("ai-mini-gateway", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	fs.StringVar(&cfg.Host, "host", envString(lookupEnv, "LOCAL_GATEWAY_RUNTIME_HOST", "127.0.0.1"), "gateway bind host")
	fs.IntVar(&cfg.Port, "port", envInt(lookupEnv, "LOCAL_GATEWAY_RUNTIME_PORT", 3457), "gateway bind port")
	fs.StringVar(&cfg.DataDir, "data-dir", envString(lookupEnv, "CORE_DATA_DIR", "./data"), "gateway data directory")
	fs.DurationVar(&cfg.ModelsCacheTTL, "models-cache-ttl", 15*time.Second, "TTL for upstream models discovery cache")

	_ = fs.Parse(args)
	return cfg
}

func (c Config) Address() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

func envString(lookupEnv func(string) (string, bool), key string, fallback string) string {
	if value, ok := lookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}

func envInt(lookupEnv func(string) (string, bool), key string, fallback int) int {
	value, ok := lookupEnv(key)
	if !ok || value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

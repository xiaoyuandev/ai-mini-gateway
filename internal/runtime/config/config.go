package config

import (
	"flag"
	"fmt"
	"time"
)

type Config struct {
	Host           string
	Port           int
	DataDir        string
	ModelsCacheTTL time.Duration
}

func FromFlags() Config {
	cfg := Config{}
	flag.StringVar(&cfg.Host, "host", "127.0.0.1", "gateway bind host")
	flag.IntVar(&cfg.Port, "port", 3457, "gateway bind port")
	flag.StringVar(&cfg.DataDir, "data-dir", "./data", "gateway data directory")
	flag.DurationVar(&cfg.ModelsCacheTTL, "models-cache-ttl", 15*time.Second, "TTL for upstream models discovery cache")
	flag.Parse()
	return cfg
}

func (c Config) Address() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

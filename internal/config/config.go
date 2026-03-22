package config

import (
	"log"
	"os"
	"strconv"
	"time"
)

type Config struct {
	DBPath            string
	CrawlDelay        time.Duration
	MaxRetries        int
	TokenBudget       int
	UserAgent         string
	ServerAddr        string
	Workers           int
	RequestsPerSecond int
}

func Load() Config {
	return Config{
		DBPath:            envOr("DEFSOURCE_DB_PATH", "./data/defsource.db"),
		CrawlDelay:        envDuration("DEFSOURCE_CRAWL_DELAY", 1*time.Second),
		MaxRetries:        envInt("DEFSOURCE_MAX_RETRIES", 3),
		TokenBudget:       envInt("DEFSOURCE_TOKEN_BUDGET", 8000),
		UserAgent:         envOr("DEFSOURCE_USER_AGENT", "defSource/1.0 (open-source documentation indexer)"),
		ServerAddr:        envOr("DEFSOURCE_ADDR", ":8080"),
		Workers:           envInt("DEFSOURCE_WORKERS", 10),
		RequestsPerSecond: envInt("DEFSOURCE_RPS", 10),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
		log.Printf("WARNING: invalid value for %s: %q, using default %d", key, v, fallback)
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
		log.Printf("WARNING: invalid value for %s: %q, using default %s", key, v, fallback)
	}
	return fallback
}

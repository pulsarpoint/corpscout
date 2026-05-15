package config

import "os"

type Config struct {
	DatabaseURL       string
	ListenAddr        string
	CrawlerURL        string
	CrawlConcurrency  int
	DomainConcurrency int
}

func Load() Config {
	return Config{
		DatabaseURL:       requireEnv("DATABASE_URL", "CORPSCOUT_DATABASE_URL"),
		ListenAddr:        getEnv("CORPSCOUT_LISTEN_ADDR", ":8090"),
		CrawlerURL:        getEnv("CORPSCOUT_CRAWLER_URL", "http://localhost:8000"),
		CrawlConcurrency:  getEnvInt("CORPSCOUT_CRAWL_CONCURRENCY", 5),
		DomainConcurrency: getEnvInt("CORPSCOUT_DOMAIN_CONCURRENCY", 10),
	}
}

func requireEnv(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	panic("required env var not set: " + keys[0])
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n := 0
	for _, c := range v {
		if c < '0' || c > '9' {
			return fallback
		}
		n = n*10 + int(c-'0')
	}
	return n
}

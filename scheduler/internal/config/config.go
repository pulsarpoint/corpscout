package config

import "os"

type Config struct {
	DatabaseURL            string
	ListenAddr             string
	CrawlerURL             string
	PostgRESTURL           string
	CrawlConcurrency       int
	DomainConcurrency      int
	GLEIFEnrichConcurrency int
	S3Endpoint             string
	S3AccessKey            string
	S3SecretKey            string
	S3Bucket               string
	LLMBaseURL             string
	LLMModel               string
	TemporalHost           string
	TemporalUIURL          string
}

func Load() Config {
	return Config{
		DatabaseURL:            requireEnv("DATABASE_URL", "CORPSCOUT_DATABASE_URL"),
		ListenAddr:             getEnv("CORPSCOUT_LISTEN_ADDR", ":8090"),
		CrawlerURL:             getEnv("CORPSCOUT_CRAWLER_URL", "http://localhost:8000"),
		PostgRESTURL:           getEnv("CORPSCOUT_POSTGREST_URL", "http://localhost:3000"),
		CrawlConcurrency:       getEnvInt("CORPSCOUT_CRAWL_CONCURRENCY", 5),
		DomainConcurrency:      getEnvInt("CORPSCOUT_DOMAIN_CONCURRENCY", 10),
		GLEIFEnrichConcurrency: getEnvInt("CORPSCOUT_GLEIF_ENRICH_CONCURRENCY", 3),
		S3Endpoint:             getEnv("CORPSCOUT_S3_ENDPOINT", "http://localhost:9000"),
		S3AccessKey:            getEnv("CORPSCOUT_S3_ACCESS_KEY", "corpscout"),
		S3SecretKey:            getEnv("CORPSCOUT_S3_SECRET_KEY", "corpscout123"),
		S3Bucket:               getEnv("CORPSCOUT_S3_BUCKET", "crawls"),
		LLMBaseURL:             getEnv("CORPSCOUT_LLM_BASE_URL", "http://100.77.62.33:8080"),
		LLMModel:               getEnv("CORPSCOUT_LLM_MODEL", "qwen3:6b"),
		TemporalHost:           getEnv("CORPSCOUT_TEMPORAL_HOST", "localhost:7233"),
		TemporalUIURL:          getEnv("CORPSCOUT_TEMPORAL_UI_URL", "http://localhost:8089"),
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

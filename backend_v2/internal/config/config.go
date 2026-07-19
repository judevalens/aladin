package config

import (
	"fmt"
	"os"
	"strconv"
)

type APIConfig struct {
	DatabaseURL         string
	HTTPAddr            string
	ProviderConnections ProviderConnectionConfig
	// DataVolumePath is the root of the Doc Surface page file store
	// (users/{userId}/pages/{pageId}/...). The API process SERVES built
	// dist from here; the MCP process WRITES + BUILDS into the same root.
	// Both must resolve to the same path.
	DataVolumePath string
}

type MCPConfig struct {
	DatabaseURL  string
	HTTPAddr     string
	ConverterURL string
	// ConverterAdminSecret authenticates the MCP collab bridge calls
	// (/admin/*) to the blocknote sidecar (M8c). Defaults to the same
	// local-dev value the sidecar defaults to; override in Docker/prod.
	ConverterAdminSecret string
	// DataVolumePath — see APIConfig.DataVolumePath. Must match the API's.
	DataVolumePath string
}

type WorkerConfig struct {
	DatabaseURL  string
	RedisURL     string
	OpenAIAPIKey string
	Concurrency  int
	// Tavily web search (optional) — empty ⇒ the resolve_low_confidence stage skips cleanly.
	TavilyAPIKey string
	// Neo4j graph projection (optional — empty URI ⇒ the graph stage skips cleanly).
	Neo4jURI  string
	Neo4jUser string
	Neo4jPass string
}

type ProviderConnectionConfig struct {
	NangoBaseURL                 string
	NangoConnectBaseURL          string
	NangoSecretKey               string
	NangoWebhookSigningKey       string
	NangoGoogleProviderConfigKey string
}

func LoadMCP() (MCPConfig, error) {
	databaseURL, err := require("DATABASE_URL")
	if err != nil {
		return MCPConfig{}, err
	}
	return MCPConfig{
		DatabaseURL:          databaseURL,
		HTTPAddr:             optional("MCP_HTTP_ADDR", ":8090"),
		ConverterURL:         optional("CONVERTER_URL", "http://localhost:3500"),
		ConverterAdminSecret: optional("BLOCKNOTE_ADMIN_SHARED_SECRET", "local-dev-admin-secret"),
		DataVolumePath:       DataVolumePathOrDefault(),
	}, nil
}

func LoadAPI() (APIConfig, error) {
	databaseURL, err := require("DATABASE_URL")
	if err != nil {
		return APIConfig{}, err
	}
	return APIConfig{
		DatabaseURL:         databaseURL,
		HTTPAddr:            optional("API_ADDR", ":8080"),
		ProviderConnections: LoadProviderConnections(),
		DataVolumePath:      DataVolumePathOrDefault(),
	}, nil
}

func LoadWorker() (WorkerConfig, error) {
	databaseURL, err := require("DATABASE_URL")
	if err != nil {
		return WorkerConfig{}, err
	}
	redisURL, err := require("REDIS_URL")
	if err != nil {
		return WorkerConfig{}, err
	}
	openAIAPIKey, err := require("OPENAI_API_KEY")
	if err != nil {
		return WorkerConfig{}, err
	}
	return WorkerConfig{
		DatabaseURL:  databaseURL,
		RedisURL:     redisURL,
		OpenAIAPIKey: openAIAPIKey,
		Concurrency:  optionalInt("WORKER_CONCURRENCY", 16),
		TavilyAPIKey: os.Getenv("TAVILY_API_KEY"),
		Neo4jURI:     os.Getenv("NEO4J_URI"),
		Neo4jUser:    os.Getenv("NEO4J_USER"),
		Neo4jPass:    os.Getenv("NEO4J_PASS"),
	}, nil
}

// AlpacaConfig authenticates the market-data + trading integration. Same key/secret pair
// works for the data API and the (paper or live) trading API — only the base URL differs.
// Empty keys ⇒ the assets sync skips cleanly (the seeded universe stays in place).
type AlpacaConfig struct {
	APIKey    string
	APISecret string
	// TradingBaseURL — where the Assets API lives. Defaults to PAPER (safe): paper and
	// live share the same asset universe, so asset sync never needs the live endpoint.
	TradingBaseURL string
	// DataBaseURL — historical bars + the real-time WS feed derive from here (T1+).
	DataBaseURL string
}

func (c AlpacaConfig) Configured() bool { return c.APIKey != "" && c.APISecret != "" }

func LoadAlpaca() AlpacaConfig {
	return AlpacaConfig{
		APIKey:         os.Getenv("ALPACA_API_KEY"),
		APISecret:      os.Getenv("ALPACA_API_SECRET"),
		TradingBaseURL: optional("ALPACA_TRADING_BASE_URL", "https://paper-api.alpaca.markets"),
		DataBaseURL:    optional("ALPACA_DATA_BASE_URL", "https://data.alpaca.markets"),
	}
}

func LoadProviderConnections() ProviderConnectionConfig {
	return ProviderConnectionConfig{
		NangoBaseURL:                 optional("NANGO_BASE_URL", "https://api.nango.dev"),
		NangoConnectBaseURL:          optional("NANGO_CONNECT_BASE_URL", "https://connect.nango.dev"),
		NangoSecretKey:               os.Getenv("NANGO_SECRET_KEY"),
		NangoWebhookSigningKey:       os.Getenv("NANGO_WEBHOOK_SIGNING_KEY"),
		NangoGoogleProviderConfigKey: os.Getenv("NANGO_GOOGLE_PROVIDER_CONFIG_KEY"),
	}
}

func require(key string) (string, error) {
	v := os.Getenv(key)
	if v == "" {
		return "", fmt.Errorf("missing required env var: %s", key)
	}
	return v, nil
}

// DataVolumePathOrDefault resolves the Doc Surface data volume path from
// DATA_VOLUME_PATH, defaulting to ./data. Single source for the default so the
// API and MCP processes agree.
func DataVolumePathOrDefault() string {
	return optional("DATA_VOLUME_PATH", "./data")
}

func optional(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}

func optionalInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(v)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

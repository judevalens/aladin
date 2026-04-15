package config

import (
	"log"
	"os"
)

type APIConfig struct {
	DatabaseURL string
	HTTPAddr    string
}

type WorkerConfig struct {
	DatabaseURL  string
	RedisURL     string
	OpenAIAPIKey string
	TavilyAPIKey string
	Neo4jURI     string
	Neo4jUser    string
	Neo4jPass    string
}

func LoadAPI() APIConfig {
	return APIConfig{
		DatabaseURL: require("DATABASE_URL"),
		HTTPAddr:    optional("API_ADDR", ":8080"),
	}
}

func LoadWorker() WorkerConfig {
	return WorkerConfig{
		DatabaseURL:  require("DATABASE_URL"),
		RedisURL:     require("REDIS_URL"),
		OpenAIAPIKey: require("OPENAI_API_KEY"),
		TavilyAPIKey: require("TAVILY_API_KEY"),
		Neo4jURI:     os.Getenv("NEO4J_URI"),
		Neo4jUser:    os.Getenv("NEO4J_USER"),
		Neo4jPass:    os.Getenv("NEO4J_PASS"),
	}
}

func require(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("missing required env var: %s", key)
	}
	return v
}

func optional(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}

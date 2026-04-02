package config

import (
	"log"
	"os"
)

type Config struct {
	DatabaseURL  string
	RedisURL     string
	OpenAIAPIKey string
	TavilyAPIKey string
	Neo4jURI     string
	Neo4jUser    string
	Neo4jPass    string
}

func Load() Config {
	return Config{
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

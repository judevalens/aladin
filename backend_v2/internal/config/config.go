package config

import (
	"fmt"
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

func LoadAPI() (APIConfig, error) {
	databaseURL, err := require("DATABASE_URL")
	if err != nil {
		return APIConfig{}, err
	}
	return APIConfig{
		DatabaseURL: databaseURL,
		HTTPAddr:    optional("API_ADDR", ":8080"),
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
	tavilyAPIKey, err := require("TAVILY_API_KEY")
	if err != nil {
		return WorkerConfig{}, err
	}
	return WorkerConfig{
		DatabaseURL:  databaseURL,
		RedisURL:     redisURL,
		OpenAIAPIKey: openAIAPIKey,
		TavilyAPIKey: tavilyAPIKey,
		Neo4jURI:     os.Getenv("NEO4J_URI"),
		Neo4jUser:    os.Getenv("NEO4J_USER"),
		Neo4jPass:    os.Getenv("NEO4J_PASS"),
	}, nil
}

func require(key string) (string, error) {
	v := os.Getenv(key)
	if v == "" {
		return "", fmt.Errorf("missing required env var: %s", key)
	}
	return v, nil
}

func optional(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}

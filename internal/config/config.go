package config

import (
	"os"
	"strconv"
)

type Config struct {
	ServiceName    string
	JaegerEndpoint string
	EmbedderModel  string
	GeneratorModel string
	QdrantHost     string
	QdrantPort     int
	ServerAddr     string
}

// Load reads configuration from environment variables, providing default values where necessary.
func Load() *Config {
	return &Config{
		ServiceName:    getEnv("SERVICE_NAME", "sentinel-rag"),
		JaegerEndpoint: getEnv("JAEGER_ENDPOINT", "localhost:4317"),
		EmbedderModel:  getEnv("EMBEDDER_MODEL", "nomic-embed-text"),
		GeneratorModel: getEnv("GENERATOR_MODEL", "phi3"),
		QdrantHost:     getEnv("QDRANT_HOST", "localhost"),
		QdrantPort:     getEnvAsInt("QDRANT_PORT", 6334),
		ServerAddr:     getEnv("SERVER_ADDR", ":8080"),
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func getEnvAsInt(key string, fallback int) int {
	strValue := getEnv(key, "")
	if value, err := strconv.Atoi(strValue); err == nil {
		return value
	}
	return fallback
}

package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	DB     DBConfig
	LLM    LLMConfig
	Server ServerConfig
}

type DBConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Name     string
	SSLMode  string
}

type LLMConfig struct {
	Provider       string
	APIKey         string
	BaseURL        string
	Model          string
	EmbeddingModel string
	EmbeddingDim   int
}

type ServerConfig struct {
	Port        string
	GinMode     string
	FrontendURL string
}

func Load() *Config {
	return &Config{
		DB: DBConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnvInt("DB_PORT", 5432),
			User:     getEnv("DB_USER", "coach"),
			Password: getEnv("DB_PASSWORD", "coach123"),
			Name:     getEnv("DB_NAME", "mysql_coach"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		LLM: LLMConfig{
			Provider:       getEnv("LLM_PROVIDER", "openai"),
			APIKey:         getEnv("LLM_API_KEY", ""),
			BaseURL:        getEnv("LLM_BASE_URL", "https://api.openai.com/v1"),
			Model:          getEnv("LLM_MODEL", "gpt-4o-mini"),
			EmbeddingModel: getEnv("EMBEDDING_MODEL", "text-embedding-3-small"),
			EmbeddingDim:   getEnvInt("EMBEDDING_DIM", 1536),
		},
		Server: ServerConfig{
			Port:        getEnv("SERVER_PORT", "8080"),
			GinMode:     getEnv("GIN_MODE", "debug"),
			FrontendURL: getEnv("FRONTEND_URL", "http://localhost:3000"),
		},
	}
}

func (c *DBConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Name, c.SSLMode)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

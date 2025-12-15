package config

import (
	"fmt"
	"os"
)

type Config struct {
	Database DatabaseConfig
	AI       AIConfig
	Server   ServerConfig
	Parent   ParentConfig
	Sidecar  SidecarConfig
}

type DatabaseConfig struct {
	Type     string // mysql, dynamodb, mongodb, etc.
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
}

type AIConfig struct {
	Provider  string // "bedrock" or "ollama"
	AWSRegion string
	ModelID   string
	OllamaURL string
}

type ServerConfig struct {
	Port string
}

type ParentConfig struct {
	Address string // Parent service gRPC address (host:port)
	Enabled bool   // Whether to connect to parent service
}

type SidecarConfig struct {
	TeamID      string
	ServiceName string
	Version     string
}

// LoadFromEnv loads configuration from environment variables
func LoadFromEnv() (*Config, error) {
	config := &Config{
		Database: DatabaseConfig{
			Type:     getEnv("DB_TYPE", "mysql"),
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "3306"),
			User:     getEnv("DB_USER", "root"),
			Password: getEnv("DB_PASSWORD", "password"),
			DBName:   getEnv("DB_NAME", "testdb"),
		},
		AI: AIConfig{
			Provider:  getEnv("AI_PROVIDER", "ollama"),
			AWSRegion: getEnv("AWS_REGION", "us-east-1"),
			ModelID:   getEnv("AI_MODEL_ID", "qwen2.5-coder:7b"),
			OllamaURL: getEnv("OLLAMA_URL", "http://host.docker.internal:11434"),
		},
		Server: ServerConfig{
			Port: getEnv("SERVER_PORT", "8080"),
		},
		Parent: ParentConfig{
			Address: getEnv("PARENT_ADDRESS", "localhost:9090"),
			Enabled: getEnv("PARENT_ENABLED", "false") == "true",
		},
		Sidecar: SidecarConfig{
			TeamID:      getEnv("TEAM_ID", "default-team"),
			ServiceName: getEnv("SERVICE_NAME", "default-service"),
			Version:     getEnv("SIDECAR_VERSION", "1.0.0"),
		},
	}

	return config, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// GetDSN returns the database connection string
func (c *DatabaseConfig) GetDSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true",
		c.User, c.Password, c.Host, c.Port, c.DBName)
}

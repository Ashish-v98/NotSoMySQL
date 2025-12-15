package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	Database DatabaseConfig
	AI       AIConfig
	Server   ServerConfig
	Parent   ParentConfig
	Sidecar  SidecarConfig
	S3       S3Config
}

type DatabaseConfig struct {
	Type     string
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

type S3Config struct {
	Bucket         string
	Region         string
	ResultSizeLimit int // Size limit in bytes for inline results (larger results go to S3)
}

// LoadFromEnv loads configuration from config.yaml file
// Environment variables can override config values
func LoadFromEnv() (*Config, error) {
	// Set config file name and path
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")           // Look in current directory
	viper.AddConfigPath("./sidecar")   // Look in sidecar directory
	viper.AddConfigPath("/etc/sidecar") // Look in /etc/sidecar

	// Enable environment variable overrides
	viper.AutomaticEnv()

	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	config := &Config{
		Database: DatabaseConfig{
			Type:     viper.GetString("database.type"),
			Host:     viper.GetString("database.host"),
			Port:     viper.GetString("database.port"),
			User:     viper.GetString("database.user"),
			Password: viper.GetString("database.password"),
			DBName:   viper.GetString("database.dbname"),
		},
		AI: AIConfig{
			Provider:  viper.GetString("ai.provider"),
			AWSRegion: viper.GetString("ai.aws_region"),
			ModelID:   viper.GetString("ai.model_id"),
			OllamaURL: viper.GetString("ai.ollama_url"),
		},
		Server: ServerConfig{
			Port: viper.GetString("server.port"),
		},
		Parent: ParentConfig{
			Address: viper.GetString("parent.address"),
			Enabled: viper.GetBool("parent.enabled"),
		},
		Sidecar: SidecarConfig{
			TeamID:      viper.GetString("sidecar.team_id"),
			ServiceName: viper.GetString("sidecar.service_name"),
			Version:     viper.GetString("sidecar.version"),
		},
		S3: S3Config{
			Bucket:          viper.GetString("s3.bucket"),
			Region:          viper.GetString("s3.region"),
			ResultSizeLimit: viper.GetInt("s3.result_size_limit"),
		},
	}

	return config, nil
}

// GetDSN returns the database connection string
func (c *DatabaseConfig) GetDSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true",
		c.User, c.Password, c.Host, c.Port, c.DBName)
}

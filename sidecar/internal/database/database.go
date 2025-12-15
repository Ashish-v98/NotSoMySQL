package database

import (
	"context"
	"fmt"
)

// QueryResult represents the result of a database query
type QueryResult struct {
	Columns []string                 `json:"columns"`
	Rows    []map[string]interface{} `json:"rows"`
	Count   int                      `json:"count"`
}

// Database is the interface that all database implementations must implement.
// To add a new database type:
// 1. Add a new DatabaseType constant below
// 2. Create a new file (e.g., dynamodb.go) implementing this interface
// 3. Add a case in NewDatabase() factory function
type Database interface {
	// ExecuteQuery executes a query and returns the results
	ExecuteQuery(ctx context.Context, query string) (*QueryResult, error)

	// GetSchema retrieves the database schema for AI context
	GetSchema(ctx context.Context, dbName string) (string, error)

	// Close closes the database connection
	Close() error

	// Type returns the database type (mysql, dynamodb, mongodb, etc.)
	Type() string
}

// DatabaseType represents supported database types
type DatabaseType string

const (
	DatabaseTypeMySQL DatabaseType = "mysql"
	// Add new database types here:
	// DatabaseTypeDynamoDB DatabaseType = "dynamodb"
	// DatabaseTypeMongoDB  DatabaseType = "mongodb"
	// DatabaseTypePostgres DatabaseType = "postgres"
)

// Config holds database connection configuration
type Config struct {
	Type     DatabaseType
	Host     string
	Port     int
	User     string
	Password string
	DBName   string

	// Add provider-specific fields as needed:
	// AWSRegion     string // for DynamoDB
	// ConnectionURI string // for MongoDB
}

// NewDatabase creates a new database instance based on the configuration.
// This is the factory function - add new database types here.
func NewDatabase(cfg Config) (Database, error) {
	switch cfg.Type {
	case DatabaseTypeMySQL:
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true",
			cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName)
		return NewMySQLDatabase(dsn)

	// Add new database implementations here:
	// case DatabaseTypeDynamoDB:
	//     return NewDynamoDBDatabase(cfg)
	// case DatabaseTypeMongoDB:
	//     return NewMongoDBDatabase(cfg)

	default:
		return nil, fmt.Errorf("unsupported database type: %s (supported: mysql)", cfg.Type)
	}
}

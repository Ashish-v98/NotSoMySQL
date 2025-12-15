package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

type MySQLDatabase struct {
	db *sqlx.DB
}

// Ensure MySQLDatabase implements Database interface
var _ Database = (*MySQLDatabase)(nil)

// NewMySQLDatabase creates a new MySQL database connection
func NewMySQLDatabase(dsn string) (*MySQLDatabase, error) {
	db, err := sqlx.Connect("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &MySQLDatabase{db: db}, nil
}

// ExecuteQuery executes a SQL query and returns the results
func (m *MySQLDatabase) ExecuteQuery(ctx context.Context, query string) (*QueryResult, error) {
	// Set query timeout
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	rows, err := m.db.QueryxContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	// Collect results
	results := make([]map[string]interface{}, 0)
	for rows.Next() {
		row := make(map[string]interface{})
		if err := rows.MapScan(row); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Convert []byte to string for better JSON serialization
		for k, v := range row {
			if b, ok := v.([]byte); ok {
				row[k] = string(b)
			}
		}

		results = append(results, row)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return &QueryResult{
		Columns: columns,
		Rows:    results,
		Count:   len(results),
	}, nil
}

// GetSchema retrieves basic schema information
func (m *MySQLDatabase) GetSchema(ctx context.Context, dbName string) (string, error) {
	query := `
		SELECT
			TABLE_NAME,
			COLUMN_NAME,
			DATA_TYPE,
			IS_NULLABLE,
			COLUMN_KEY
		FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_SCHEMA = ?
		ORDER BY TABLE_NAME, ORDINAL_POSITION
	`

	rows, err := m.db.QueryxContext(ctx, query, dbName)
	if err != nil {
		return "", fmt.Errorf("failed to get schema: %w", err)
	}
	defer rows.Close()

	type SchemaRow struct {
		TableName  string         `db:"TABLE_NAME"`
		ColumnName string         `db:"COLUMN_NAME"`
		DataType   string         `db:"DATA_TYPE"`
		IsNullable string         `db:"IS_NULLABLE"`
		ColumnKey  sql.NullString `db:"COLUMN_KEY"`
	}

	schema := ""
	currentTable := ""

	for rows.Next() {
		var row SchemaRow
		if err := rows.StructScan(&row); err != nil {
			return "", fmt.Errorf("failed to scan schema row: %w", err)
		}

		if row.TableName != currentTable {
			if currentTable != "" {
				schema += "\n\n"
			}
			schema += fmt.Sprintf("Table: %s\n", row.TableName)
			currentTable = row.TableName
		}

		keyInfo := ""
		if row.ColumnKey.Valid && row.ColumnKey.String == "PRI" {
			keyInfo = " (PRIMARY KEY)"
		}

		schema += fmt.Sprintf("  - %s: %s%s\n", row.ColumnName, row.DataType, keyInfo)
	}

	return schema, nil
}

// Close closes the database connection
func (m *MySQLDatabase) Close() error {
	return m.db.Close()
}

// Type returns the database type
func (m *MySQLDatabase) Type() string {
	return string(DatabaseTypeMySQL)
}

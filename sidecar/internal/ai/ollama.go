package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type OllamaClient struct {
	baseURL string
	model   string
	client  *http.Client
}

type OllamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type OllamaResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

// NewOllamaClient creates a new Ollama client
func NewOllamaClient(baseURL, model string) *OllamaClient {
	if baseURL == "" {
		baseURL = "http://host.docker.internal:11434" // Ollama default on Windows/Mac
	}
	return &OllamaClient{
		baseURL: baseURL,
		model:   model,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// GenerateSQL generates SQL from natural language query using Ollama
func (o *OllamaClient) GenerateSQL(ctx context.Context, schema, userQuery string) (string, error) {
	prompt := o.buildPrompt(schema, userQuery)

	reqBody := OllamaRequest{
		Model:  o.model,
		Prompt: prompt,
		Stream: false,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", o.baseURL+"/api/generate", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call Ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Ollama returned status %d: %s", resp.StatusCode, string(body))
	}

	var result OllamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	sql := o.extractSQL(result.Response)
	return sql, nil
}

// buildPrompt creates the prompt for SQL generation
func (o *OllamaClient) buildPrompt(schema, userQuery string) string {
	return fmt.Sprintf(`You are a SQL expert for MySQL 8.0 with ONLY_FULL_GROUP_BY mode enabled.

Database Schema:
%s

STRICT RULES - FOLLOW EXACTLY:
1. Return ONLY the raw SQL query - NO explanations, NO markdown, NO code blocks
2. Use proper MySQL 8.0 syntax
3. Always use explicit JOINs with proper ON clauses (e.g., LEFT JOIN table t ON t.id = x.id)
4. Add LIMIT 1000 at the end for safety
5. Use short table aliases (e.g., c for customers, o for orders, p for products)
6. Only SELECT queries - never INSERT, UPDATE, DELETE, DROP
7. CRITICAL: When using GROUP BY, ALL columns in SELECT must either be:
   - In the GROUP BY clause, OR
   - Inside an aggregate function (COUNT, SUM, AVG, MAX, MIN)
8. For columns not in GROUP BY, use MAX() or MIN() to pick one value
9. Use COALESCE(column, 0) for nullable numeric columns in aggregations
10. Double-check all column names exist in the schema above
11. Ensure all JOINs have matching column types

User Question: %s

SQL Query:`, schema, userQuery)
}

// extractSQL extracts SQL from the response, removing any markdown or extra text
func (o *OllamaClient) extractSQL(text string) string {
	text = strings.TrimSpace(text)

	// Remove markdown code blocks
	text = strings.TrimPrefix(text, "```sql")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")

	// If there are multiple lines and first line is explanation, take the SQL part
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToUpper(trimmed), "SELECT") {
			// Found the SQL, join from here
			text = strings.Join(lines[i:], "\n")
			break
		}
	}

	return strings.TrimSpace(text)
}

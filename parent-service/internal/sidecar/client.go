package sidecar

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client handles HTTP communication with sidecars
type Client struct {
	httpClient *http.Client
}

// QueryRequest represents a query request to sidecar
type QueryRequest struct {
	Query string `json:"query"`
}

// QueryResponse represents a query response from sidecar
type QueryResponse struct {
	GeneratedSQL string       `json:"generated_sql"`
	Results      *QueryResult `json:"results,omitempty"`
	Error        string       `json:"error,omitempty"`
}

// QueryResult holds the query result data
type QueryResult struct {
	Columns []string                 `json:"columns"`
	Rows    []map[string]interface{} `json:"rows"`
	Count   int                      `json:"count"`
}

// NewClient creates a new sidecar HTTP client
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// ExecuteQuery sends a natural language query to a sidecar
func (c *Client) ExecuteQuery(ctx context.Context, sidecarEndpoint, query string) (*QueryResponse, error) {
	url := fmt.Sprintf("http://%s/query", sidecarEndpoint)

	reqBody := QueryRequest{Query: query}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request to sidecar: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sidecar returned status %d: %s", resp.StatusCode, string(body))
	}

	var queryResp QueryResponse
	if err := json.Unmarshal(body, &queryResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &queryResp, nil
}

// GetHealth checks the health of a sidecar
func (c *Client) GetHealth(ctx context.Context, sidecarEndpoint string) (map[string]interface{}, error) {
	url := fmt.Sprintf("http://%s/health", sidecarEndpoint)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var health map[string]interface{}
	if err := json.Unmarshal(body, &health); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return health, nil
}

package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
)

type BedrockClient struct {
	client  *bedrockruntime.Client
	modelID string
}

// ClaudeRequest represents the request format for Claude models on Bedrock
type ClaudeRequest struct {
	AnthropicVersion string    `json:"anthropic_version"`
	MaxTokens        int       `json:"max_tokens"`
	Messages         []Message `json:"messages"`
	System           string    `json:"system,omitempty"`
	Temperature      float64   `json:"temperature"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ClaudeResponse represents the response from Claude models
type ClaudeResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
	StopReason string `json:"stop_reason"`
}

// NewBedrockClient creates a new Bedrock client
func NewBedrockClient(ctx context.Context, region, modelID string) (*BedrockClient, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &BedrockClient{
		client:  bedrockruntime.NewFromConfig(cfg),
		modelID: modelID,
	}, nil
}

// GenerateSQL generates SQL from natural language query
func (b *BedrockClient) GenerateSQL(ctx context.Context, schema, userQuery string) (string, error) {
	systemPrompt := b.buildSystemPrompt(schema)

	request := ClaudeRequest{
		AnthropicVersion: "bedrock-2023-05-31",
		MaxTokens:        2000,
		System:           systemPrompt,
		Temperature:      0.2, // Low temperature for deterministic SQL
		Messages: []Message{
			{
				Role:    "user",
				Content: userQuery,
			},
		},
	}

	body, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	output, err := b.client.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
		ModelId:     aws.String(b.modelID),
		ContentType: aws.String("application/json"),
		Body:        body,
	})
	if err != nil {
		return "", fmt.Errorf("failed to invoke model: %w", err)
	}

	var response ClaudeResponse
	if err := json.Unmarshal(output.Body, &response); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(response.Content) == 0 {
		return "", fmt.Errorf("no content in response")
	}

	sql := b.extractSQL(response.Content[0].Text)
	return sql, nil
}

// buildSystemPrompt creates the system prompt with schema information
func (b *BedrockClient) buildSystemPrompt(schema string) string {
	return fmt.Sprintf(`You are a SQL expert for MySQL. Generate valid SQL queries from natural language.

Database Schema:
%s

Rules:
1. Return ONLY the SQL query, no explanations or markdown
2. Use proper MySQL syntax
3. Always use explicit JOINs (never implicit)
4. Add LIMIT clause (max 1000 rows) for safety
5. Use table aliases for readability
6. Only SELECT queries (no mutations like INSERT, UPDATE, DELETE, DROP)
7. Use proper date/time functions
8. Handle NULL values appropriately

Important: Return ONLY the SQL query itself, without any markdown formatting, code blocks, or explanations.`, schema)
}

// extractSQL extracts SQL from the AI response, removing any markdown formatting
func (b *BedrockClient) extractSQL(text string) string {
	// Remove markdown code blocks if present
	text = strings.TrimSpace(text)

	// Remove ```sql and ``` markers
	text = strings.TrimPrefix(text, "```sql")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")

	// Remove any leading/trailing whitespace
	text = strings.TrimSpace(text)

	return text
}

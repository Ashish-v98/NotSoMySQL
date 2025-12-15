package ai

import "context"

// AIClient interface for different AI providers
type AIClient interface {
	GenerateSQL(ctx context.Context, schema, userQuery string) (string, error)
}

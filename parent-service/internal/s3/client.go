package s3

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Client handles S3 operations for downloading query results
type Client struct {
	s3Client *s3.Client
}

// QueryResult represents the structure of query results stored in S3
type QueryResult struct {
	Columns []string                 `json:"columns"`
	Rows    []map[string]interface{} `json:"rows"`
	Count   int                      `json:"count"`
}

// NewClient creates a new S3 client
func NewClient(ctx context.Context, region string) (*Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Support custom endpoint for LocalStack
	var s3Client *s3.Client
	if endpoint := os.Getenv("AWS_ENDPOINT_URL"); endpoint != "" {
		s3Client = s3.NewFromConfig(cfg, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(endpoint)
			o.UsePathStyle = true // Required for LocalStack
		})
	} else {
		s3Client = s3.NewFromConfig(cfg)
	}

	return &Client{
		s3Client: s3Client,
	}, nil
}

// DownloadQueryResult downloads query results from S3
// s3Path format: s3://bucket/key
func (c *Client) DownloadQueryResult(ctx context.Context, s3Path string) (*QueryResult, error) {
	// Parse S3 path
	bucket, key, err := parseS3Path(s3Path)
	if err != nil {
		return nil, err
	}

	// Download from S3
	result, err := c.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to download from S3: %w", err)
	}
	defer result.Body.Close()

	// Read the body
	body, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read S3 object body: %w", err)
	}

	// Unmarshal JSON
	var queryResult QueryResult
	if err := json.Unmarshal(body, &queryResult); err != nil {
		return nil, fmt.Errorf("failed to unmarshal query result: %w", err)
	}

	return &queryResult, nil
}

// parseS3Path parses s3://bucket/key into bucket and key
func parseS3Path(s3Path string) (bucket, key string, err error) {
	if !strings.HasPrefix(s3Path, "s3://") {
		return "", "", fmt.Errorf("invalid S3 path format: %s (expected s3://bucket/key)", s3Path)
	}

	// Remove s3:// prefix
	path := strings.TrimPrefix(s3Path, "s3://")

	// Split by first /
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid S3 path format: %s (expected s3://bucket/key)", s3Path)
	}

	return parts[0], parts[1], nil
}

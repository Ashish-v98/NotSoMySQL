package s3

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/yourorg/ai-query-sidecar/internal/database"
)

// Client handles S3 operations for query results
type Client struct {
	s3Client *s3.Client
	bucket   string
}

// NewClient creates a new S3 client
func NewClient(ctx context.Context, region, bucket string) (*Client, error) {
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
		bucket:   bucket,
	}, nil
}

// UploadQueryResult uploads query results to S3 and returns the S3 path
func (c *Client) UploadQueryResult(ctx context.Context, queryID string, result *database.QueryResult) (string, error) {
	// Convert result to JSON
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	// Generate S3 key with timestamp for uniqueness
	timestamp := time.Now().Format("2006/01/02/15")
	key := fmt.Sprintf("query-results/%s/%s.json", timestamp, queryID)

	// Upload to S3
	_, err = c.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(c.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(resultJSON),
		ContentType: aws.String("application/json"),
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload to S3: %w", err)
	}

	// Return S3 path
	s3Path := fmt.Sprintf("s3://%s/%s", c.bucket, key)
	return s3Path, nil
}

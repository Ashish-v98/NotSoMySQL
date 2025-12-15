package sidecar

import (
	"context"
	"fmt"
	"time"

	pb "github.com/notsoMySQL/sidecar-client"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client handles gRPC communication with sidecars
type Client struct {
	conn   *grpc.ClientConn
	client pb.SidecarServiceClient
}

// QueryResult holds the query result data
type QueryResult struct {
	Columns []string                 `json:"columns"`
	Rows    []map[string]interface{} `json:"rows"`
	Count   int                      `json:"count"`
}

// NewClient creates a new sidecar gRPC client
func NewClient(sidecarAddress string) (*Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, sidecarAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to sidecar at %s: %w", sidecarAddress, err)
	}

	return &Client{
		conn:   conn,
		client: pb.NewSidecarServiceClient(conn),
	}, nil
}

// Close closes the gRPC connection
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// ExecuteQuery sends a natural language query to a sidecar
func (c *Client) ExecuteQuery(ctx context.Context, query string) (*pb.SidecarQueryResponse, error) {
	req := &pb.SidecarQueryRequest{
		Query: query,
		Options: &pb.QueryOptions{
			RowLimit:       10000,
			TimeoutSeconds: 30,
			UseCache:       false,
		},
	}

	resp, err := c.client.ExecuteQuery(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	return resp, nil
}

// ExecuteDirectQuery executes a pre-generated SQL query
func (c *Client) ExecuteDirectQuery(ctx context.Context, sql string) (*pb.SidecarQueryResponse, error) {
	req := &pb.DirectQueryRequest{
		Sql: sql,
		Options: &pb.QueryOptions{
			RowLimit:       10000,
			TimeoutSeconds: 30,
			UseCache:       false,
		},
	}

	resp, err := c.client.ExecuteDirectQuery(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute direct query: %w", err)
	}

	return resp, nil
}

// GetHealth checks the health of a sidecar
func (c *Client) GetHealth(ctx context.Context) (*pb.HealthCheckResponse, error) {
	req := &pb.HealthCheckRequest{}

	resp, err := c.client.Health(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get health: %w", err)
	}

	return resp, nil
}

// GetSchema retrieves the database schema from sidecar
func (c *Client) GetSchema(ctx context.Context) (*pb.SchemaResponse, error) {
	req := &pb.SidecarSchemaRequest{
		IncludeSampleData: false,
	}

	resp, err := c.client.GetSchema(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get schema: %w", err)
	}

	return resp, nil
}

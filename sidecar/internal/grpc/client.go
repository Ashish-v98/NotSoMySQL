package grpc

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	pb "github.com/notsoMySQL/sidecar-client"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Client manages the gRPC connection to the parent service
type Client struct {
	conn        *grpc.ClientConn
	client      pb.ParentServiceClient
	sidecarID   string
	teamID      string
	serviceName string
	version     string
	dbInfo      *pb.DatabaseInfo
	grpcPort    string // gRPC port for parent to call back

	mu         sync.RWMutex
	registered bool
	stopCh     chan struct{}
}

// NewClient creates a new gRPC client for connecting to parent service
func NewClient(address, teamID, serviceName, version, grpcPort string) (*Client, error) {
	// Create gRPC connection with options
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(10*1024*1024), // 10MB
			grpc.MaxCallSendMsgSize(10*1024*1024), // 10MB
		),
	}

	conn, err := grpc.NewClient(address, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to parent service: %w", err)
	}

	return &Client{
		conn:        conn,
		client:      pb.NewParentServiceClient(conn),
		teamID:      teamID,
		serviceName: serviceName,
		version:     version,
		grpcPort:    grpcPort,
		stopCh:      make(chan struct{}),
	}, nil
}

// SetDatabaseInfo sets the database information for registration
func (c *Client) SetDatabaseInfo(dbType, dbName, host string, port, tableCount int) {
	c.dbInfo = &pb.DatabaseInfo{
		DbType:     dbType,
		DbName:     dbName,
		Host:       host,
		Port:       int32(port),
		TableCount: int32(tableCount),
	}
}

// Register registers this sidecar with the parent service
func (c *Client) Register(ctx context.Context) error {
	hostname, _ := os.Hostname()
	// Include gRPC port in hostname for parent to call back
	grpcEndpoint := fmt.Sprintf("%s:%s", hostname, c.grpcPort)

	req := &pb.RegisterRequest{
		TeamId:      c.teamID,
		ServiceName: c.serviceName,
		Version:     c.version,
		DbInfo:      c.dbInfo,
		Hostname:    grpcEndpoint, // Format: "hostname:port"
	}

	resp, err := c.client.RegisterSidecar(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to register sidecar: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("registration failed: %s", resp.Message)
	}

	c.mu.Lock()
	c.sidecarID = resp.SidecarId
	c.registered = true
	c.mu.Unlock()

	log.Printf("Registered with parent service, sidecar ID: %s", resp.SidecarId)
	return nil
}

// StartHeartbeat starts the heartbeat loop in a goroutine
func (c *Client) StartHeartbeat(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := c.sendHeartbeat(); err != nil {
					log.Printf("Heartbeat failed: %v", err)
				}
			case <-c.stopCh:
				log.Println("Heartbeat loop stopped")
				return
			}
		}
	}()
}

// sendHeartbeat sends a single heartbeat to the parent service
func (c *Client) sendHeartbeat() error {
	c.mu.RLock()
	if !c.registered {
		c.mu.RUnlock()
		return fmt.Errorf("sidecar not registered")
	}
	sidecarID := c.sidecarID
	c.mu.RUnlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := &pb.HeartbeatRequest{
		SidecarId: sidecarID,
		Status:    "healthy",
		Timestamp: timestamppb.Now(),
		Metrics: &pb.HealthMetrics{
			ActiveConnections: 1,
			QueriesPerMinute:  0,
			CpuPercent:        0,
			MemoryPercent:     0,
		},
	}

	resp, err := c.client.Heartbeat(ctx, req)
	if err != nil {
		return fmt.Errorf("heartbeat request failed: %w", err)
	}

	if !resp.Acknowledged {
		return fmt.Errorf("heartbeat not acknowledged")
	}

	return nil
}

// Unregister unregisters the sidecar from the parent service
func (c *Client) Unregister(ctx context.Context, reason string) error {
	c.mu.RLock()
	if !c.registered {
		c.mu.RUnlock()
		return nil
	}
	sidecarID := c.sidecarID
	c.mu.RUnlock()

	req := &pb.UnregisterRequest{
		SidecarId: sidecarID,
		Reason:    reason,
	}

	_, err := c.client.UnregisterSidecar(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to unregister: %w", err)
	}

	c.mu.Lock()
	c.registered = false
	c.mu.Unlock()

	log.Printf("Unregistered from parent service")
	return nil
}

// Close stops the heartbeat and closes the connection
func (c *Client) Close() error {
	close(c.stopCh)
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// GetSidecarID returns the sidecar ID assigned by parent
func (c *Client) GetSidecarID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.sidecarID
}

// IsRegistered returns whether the sidecar is registered
func (c *Client) IsRegistered() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.registered
}

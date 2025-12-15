package grpc

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	pb "github.com/notsoMySQL/sidecar-client"
	"github.com/notsoMySQL/parent-service/internal/sidecar"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Server implements the ParentService gRPC server
type Server struct {
	pb.UnimplementedParentServiceServer
	sidecars map[string]*SidecarInfo
	mu       sync.RWMutex
}

// SidecarInfo stores information about a connected sidecar
type SidecarInfo struct {
	ID           string
	TeamID       string
	ServiceName  string
	Version      string
	DBInfo       *pb.DatabaseInfo
	Hostname     string
	GRPCAddress  string // host:port for gRPC connection
	Status       string
	LastSeen     time.Time
	RegisteredAt time.Time
}

// NewServer creates a new ParentService gRPC server
func NewServer() *Server {
	return &Server{
		sidecars: make(map[string]*SidecarInfo),
	}
}

// RegisterSidecar handles sidecar registration
func (s *Server) RegisterSidecar(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Generate sidecar ID
	sidecarID := fmt.Sprintf("%s-%s-%d", req.TeamId, req.ServiceName, time.Now().Unix())

	// Store sidecar info
	// Hostname already contains "host:port" format from sidecar registration
	s.sidecars[sidecarID] = &SidecarInfo{
		ID:           sidecarID,
		TeamID:       req.TeamId,
		ServiceName:  req.ServiceName,
		Version:      req.Version,
		DBInfo:       req.DbInfo,
		Hostname:     req.Hostname,
		GRPCAddress:  req.Hostname, // Use hostname:port for gRPC connection
		Status:       "healthy",
		LastSeen:     time.Now(),
		RegisteredAt: time.Now(),
	}

	log.Printf("Registered sidecar: %s (team: %s, service: %s, db: %s)",
		sidecarID, req.TeamId, req.ServiceName, req.DbInfo.DbType)

	return &pb.RegisterResponse{
		SidecarId: sidecarID,
		Success:   true,
		Message:   "Sidecar registered successfully",
	}, nil
}

// Heartbeat handles sidecar heartbeats
func (s *Server) Heartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sidecar, exists := s.sidecars[req.SidecarId]
	if !exists {
		return &pb.HeartbeatResponse{
			Acknowledged: false,
			ServerTime:   timestamppb.Now(),
		}, fmt.Errorf("sidecar %s not found", req.SidecarId)
	}

	// Update last seen and status
	sidecar.LastSeen = time.Now()
	sidecar.Status = req.Status

	return &pb.HeartbeatResponse{
		Acknowledged: true,
		ServerTime:   timestamppb.Now(),
	}, nil
}

// ExecuteQuery handles query execution requests
func (s *Server) ExecuteQuery(ctx context.Context, req *pb.QueryRequest) (*pb.QueryResponse, error) {
	s.mu.RLock()
	sidecarInfo, exists := s.sidecars[req.SidecarId]
	s.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("sidecar %s not found", req.SidecarId)
	}

	log.Printf("Routing query to sidecar %s (team: %s): %s", req.SidecarId, req.TeamId, req.Query)

	// Create sidecar client
	client, err := sidecar.NewClient(sidecarInfo.GRPCAddress)
	if err != nil {
		return &pb.QueryResponse{
			QueryId: fmt.Sprintf("q-%d", time.Now().Unix()),
			Error:   fmt.Sprintf("Failed to connect to sidecar: %v", err),
		}, nil
	}
	defer client.Close()

	// Execute query on sidecar
	resp, err := client.ExecuteQuery(ctx, req.Query)
	if err != nil {
		return &pb.QueryResponse{
			QueryId: fmt.Sprintf("q-%d", time.Now().Unix()),
			Error:   fmt.Sprintf("Failed to execute query: %v", err),
		}, nil
	}

	// Convert SidecarQueryResponse to QueryResponse
	return &pb.QueryResponse{
		QueryId:        resp.QueryId,
		GeneratedQuery: resp.GeneratedQuery,
		Result:         resp.GetInlineResult(),
		Metadata:       resp.Metadata,
		Error:          resp.Error,
	}, nil
}

// GetSchema retrieves database schema
func (s *Server) GetSchema(ctx context.Context, req *pb.SchemaRequest) (*pb.SchemaResponse, error) {
	s.mu.RLock()
	sidecarInfo, exists := s.sidecars[req.SidecarId]
	s.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("sidecar %s not found", req.SidecarId)
	}

	log.Printf("Routing schema request to sidecar %s (team: %s)", req.SidecarId, req.TeamId)

	// Create sidecar client
	client, err := sidecar.NewClient(sidecarInfo.GRPCAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to sidecar: %w", err)
	}
	defer client.Close()

	// Get schema from sidecar
	resp, err := client.GetSchema(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get schema: %w", err)
	}

	return resp, nil
}

// SaveDashboard saves a dashboard configuration
func (s *Server) SaveDashboard(ctx context.Context, req *pb.SaveDashboardRequest) (*pb.SaveDashboardResponse, error) {
	log.Printf("Saving dashboard for team %s: %s", req.TeamId, req.DashboardName)

	// TODO: Implement dashboard storage (DynamoDB)
	dashboardID := fmt.Sprintf("dash-%s-%d", req.TeamId, time.Now().Unix())

	return &pb.SaveDashboardResponse{
		DashboardId: dashboardID,
		Success:     true,
		Message:     "Dashboard saved successfully",
	}, nil
}

// UnregisterSidecar handles sidecar unregistration
func (s *Server) UnregisterSidecar(ctx context.Context, req *pb.UnregisterRequest) (*pb.UnregisterResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if sidecar, exists := s.sidecars[req.SidecarId]; exists {
		log.Printf("Unregistered sidecar: %s (team: %s, reason: %s)",
			req.SidecarId, sidecar.TeamID, req.Reason)
		delete(s.sidecars, req.SidecarId)
	}

	return &pb.UnregisterResponse{
		Acknowledged: true,
	}, nil
}

// GetSidecarCount returns the number of connected sidecars
func (s *Server) GetSidecarCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.sidecars)
}

// GetSidecars returns all connected sidecars
func (s *Server) GetSidecars() map[string]*SidecarInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return a copy to avoid race conditions
	sidecars := make(map[string]*SidecarInfo)
	for k, v := range s.sidecars {
		sidecars[k] = v
	}
	return sidecars
}

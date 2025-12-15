package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	pb "github.com/notsoMySQL/sidecar-client"
	grpcserver "github.com/notsoMySQL/parent-service/internal/grpc"
	"github.com/notsoMySQL/parent-service/internal/s3"
	"github.com/notsoMySQL/parent-service/internal/sidecar"
)

// RESTServer handles REST API requests from the dashboard
type RESTServer struct {
	grpcServer *grpcserver.Server
	s3Client   *s3.Client
	router     *gin.Engine
}

// QueryRequest represents a natural language query request
type QueryRequest struct {
	Query string `json:"query" binding:"required"`
}

// QueryResponse represents the response from a query
type QueryResponse struct {
	QueryID      string                   `json:"query_id"`
	GeneratedSQL string                   `json:"generated_sql"`
	Results      *QueryResults            `json:"results,omitempty"`
	Error        string                   `json:"error,omitempty"`
	Metadata     map[string]interface{}   `json:"metadata,omitempty"`
}

// QueryResults holds the query result data
type QueryResults struct {
	Columns  []string                 `json:"columns"`
	Rows     []map[string]interface{} `json:"rows"`
	RowCount int                      `json:"row_count"`
}

// TeamInfo represents a team's information
type TeamInfo struct {
	TeamID   string   `json:"team_id"`
	Services []string `json:"services"`
}

// SidecarStatus represents a sidecar's status
type SidecarStatus struct {
	SidecarID   string    `json:"sidecar_id"`
	TeamID      string    `json:"team_id"`
	ServiceName string    `json:"service_name"`
	Status      string    `json:"status"`
	DBType      string    `json:"db_type"`
	DBName      string    `json:"db_name"`
	LastSeen    time.Time `json:"last_seen"`
}

// NewRESTServer creates a new REST API server
func NewRESTServer(grpcServer *grpcserver.Server) *RESTServer {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(gin.Logger())

	// Configure CORS for dashboard
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173", "http://localhost:3000", "*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Initialize S3 client for downloading query results
	s3Client, err := s3.NewClient(context.Background(), "us-east-1") // TODO: Make region configurable
	if err != nil {
		// S3 client is optional, log warning but continue
		fmt.Printf("Warning: Failed to initialize S3 client: %v\n", err)
	}

	server := &RESTServer{
		grpcServer: grpcServer,
		s3Client:   s3Client,
		router:     router,
	}

	server.setupRoutes()
	return server
}

// setupRoutes configures all REST API routes
func (s *RESTServer) setupRoutes() {
	api := s.router.Group("/api/v1")
	{
		// Health check
		api.GET("/health", s.healthHandler)

		// Teams endpoints
		api.GET("/teams", s.listTeamsHandler)
		api.GET("/teams/:teamId/services", s.listServicesHandler)
		api.GET("/teams/:teamId/sidecars", s.listSidecarsHandler)

		// Query endpoints
		api.POST("/teams/:teamId/query", s.executeQueryHandler)
		api.GET("/teams/:teamId/schema", s.getSchemaHandler)

		// Sidecar management
		api.GET("/sidecars", s.listAllSidecarsHandler)
	}
}

// GetRouter returns the Gin router for starting the server
func (s *RESTServer) GetRouter() *gin.Engine {
	return s.router
}

// healthHandler returns the health status of the parent service
func (s *RESTServer) healthHandler(c *gin.Context) {
	sidecarCount := s.grpcServer.GetSidecarCount()
	c.JSON(http.StatusOK, gin.H{
		"status":         "healthy",
		"time":           time.Now().Format(time.RFC3339),
		"sidecar_count":  sidecarCount,
		"service":        "parent-service",
	})
}

// listTeamsHandler returns all registered teams
func (s *RESTServer) listTeamsHandler(c *gin.Context) {
	sidecars := s.grpcServer.GetSidecars()

	// Group sidecars by team
	teams := make(map[string][]string)
	for _, sidecar := range sidecars {
		teams[sidecar.TeamID] = append(teams[sidecar.TeamID], sidecar.ServiceName)
	}

	// Convert to response format
	var response []TeamInfo
	for teamID, services := range teams {
		response = append(response, TeamInfo{
			TeamID:   teamID,
			Services: services,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"teams": response,
		"count": len(response),
	})
}

// listServicesHandler returns services for a specific team
func (s *RESTServer) listServicesHandler(c *gin.Context) {
	teamID := c.Param("teamId")
	sidecars := s.grpcServer.GetSidecars()

	var services []string
	for _, sidecar := range sidecars {
		if sidecar.TeamID == teamID {
			services = append(services, sidecar.ServiceName)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"team_id":  teamID,
		"services": services,
		"count":    len(services),
	})
}

// listSidecarsHandler returns all sidecars for a team
func (s *RESTServer) listSidecarsHandler(c *gin.Context) {
	teamID := c.Param("teamId")
	sidecars := s.grpcServer.GetSidecars()

	var response []SidecarStatus
	for _, sidecar := range sidecars {
		if sidecar.TeamID == teamID {
			status := SidecarStatus{
				SidecarID:   sidecar.ID,
				TeamID:      sidecar.TeamID,
				ServiceName: sidecar.ServiceName,
				Status:      sidecar.Status,
				LastSeen:    sidecar.LastSeen,
			}
			if sidecar.DBInfo != nil {
				status.DBType = sidecar.DBInfo.DbType
				status.DBName = sidecar.DBInfo.DbName
			}
			response = append(response, status)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"team_id":  teamID,
		"sidecars": response,
		"count":    len(response),
	})
}

// listAllSidecarsHandler returns all connected sidecars
func (s *RESTServer) listAllSidecarsHandler(c *gin.Context) {
	sidecars := s.grpcServer.GetSidecars()

	var response []SidecarStatus
	for _, sidecar := range sidecars {
		status := SidecarStatus{
			SidecarID:   sidecar.ID,
			TeamID:      sidecar.TeamID,
			ServiceName: sidecar.ServiceName,
			Status:      sidecar.Status,
			LastSeen:    sidecar.LastSeen,
		}
		if sidecar.DBInfo != nil {
			status.DBType = sidecar.DBInfo.DbType
			status.DBName = sidecar.DBInfo.DbName
		}
		response = append(response, status)
	}

	c.JSON(http.StatusOK, gin.H{
		"sidecars": response,
		"count":    len(response),
	})
}

// executeQueryHandler executes a natural language query
func (s *RESTServer) executeQueryHandler(c *gin.Context) {
	teamID := c.Param("teamId")

	var req QueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Find a sidecar for this team
	sidecars := s.grpcServer.GetSidecars()
	var targetSidecar *grpcserver.SidecarInfo
	for _, sc := range sidecars {
		if sc.TeamID == teamID && sc.Status == "healthy" {
			targetSidecar = sc
			break
		}
	}

	if targetSidecar == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "No healthy sidecar found for team: " + teamID,
		})
		return
	}

	// Create sidecar gRPC client
	sidecarClient, err := sidecar.NewClient(targetSidecar.GRPCAddress)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      fmt.Sprintf("Failed to connect to sidecar: %v", err),
			"sidecar_id": targetSidecar.ID,
		})
		return
	}
	defer sidecarClient.Close()

	// Execute query on sidecar
	queryResp, err := sidecarClient.ExecuteQuery(c.Request.Context(), req.Query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      fmt.Sprintf("Failed to execute query on sidecar: %v", err),
			"sidecar_id": targetSidecar.ID,
		})
		return
	}

	// Build response
	response := QueryResponse{
		QueryID:      queryResp.QueryId,
		GeneratedSQL: queryResp.GeneratedQuery,
		Error:        queryResp.Error,
		Metadata: map[string]interface{}{
			"team_id":      teamID,
			"sidecar_id":   targetSidecar.ID,
			"service_name": targetSidecar.ServiceName,
		},
	}

	// Handle inline results or S3 path
	if inlineResult := queryResp.GetInlineResult(); inlineResult != nil {
		response.Results = convertProtoToQueryResults(inlineResult)
	} else if s3Path := queryResp.GetS3Path(); s3Path != "" {
		// Download results from S3
		if s.s3Client != nil {
			s3Result, err := s.s3Client.DownloadQueryResult(c.Request.Context(), s3Path)
			if err != nil {
				response.Error = fmt.Sprintf("Failed to download results from S3: %v", err)
				response.Metadata["s3_path"] = s3Path
			} else {
				response.Results = &QueryResults{
					Columns:  s3Result.Columns,
					Rows:     s3Result.Rows,
					RowCount: s3Result.Count,
				}
				response.Metadata["s3_path"] = s3Path
				response.Metadata["downloaded_from_s3"] = true
			}
		} else {
			response.Error = "Results stored in S3 but S3 client not available"
			response.Metadata["s3_path"] = s3Path
		}
	}

	c.JSON(http.StatusOK, response)
}

// getSchemaHandler returns the database schema for a team
func (s *RESTServer) getSchemaHandler(c *gin.Context) {
	teamID := c.Param("teamId")

	// Find a sidecar for this team
	sidecars := s.grpcServer.GetSidecars()
	var targetSidecar *grpcserver.SidecarInfo
	for _, sidecar := range sidecars {
		if sidecar.TeamID == teamID {
			targetSidecar = sidecar
			break
		}
	}

	if targetSidecar == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "No sidecar found for team: " + teamID,
		})
		return
	}

	// Return placeholder - will be wired to sidecar in next step
	c.JSON(http.StatusOK, gin.H{
		"team_id":    teamID,
		"sidecar_id": targetSidecar.ID,
		"schema":     []interface{}{},
		"message":    "Schema retrieval not yet implemented",
	})
}

// Helper function to convert proto QueryResult to REST API QueryResults
func convertProtoToQueryResults(protoResult *pb.QueryResult) *QueryResults {
	if protoResult == nil {
		return nil
	}

	rows := make([]map[string]interface{}, 0, len(protoResult.Rows))
	for _, protoRow := range protoResult.Rows {
		row := make(map[string]interface{})
		for key, val := range protoRow.Values {
			row[key] = protoValueToInterface(val)
		}
		rows = append(rows, row)
	}

	return &QueryResults{
		Columns:  protoResult.Columns,
		Rows:     rows,
		RowCount: int(protoResult.RowCount),
	}
}

// Helper function to convert proto Value to interface{}
func protoValueToInterface(val *pb.Value) interface{} {
	if val == nil || val.IsNull {
		return nil
	}

	switch v := val.Value.(type) {
	case *pb.Value_StringValue:
		return v.StringValue
	case *pb.Value_IntValue:
		return v.IntValue
	case *pb.Value_DoubleValue:
		return v.DoubleValue
	case *pb.Value_BoolValue:
		return v.BoolValue
	case *pb.Value_BytesValue:
		return v.BytesValue
	case *pb.Value_TimestampValue:
		return v.TimestampValue.AsTime()
	default:
		return nil
	}
}


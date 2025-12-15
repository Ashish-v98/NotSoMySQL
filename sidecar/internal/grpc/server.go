package grpc

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/yourorg/ai-query-sidecar/internal/ai"
	"github.com/yourorg/ai-query-sidecar/internal/database"
	"github.com/yourorg/ai-query-sidecar/internal/s3"
	pb "github.com/notsoMySQL/sidecar-client"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// SidecarServer implements the SidecarService gRPC server
type SidecarServer struct {
	pb.UnimplementedSidecarServiceServer
	db              database.Database
	aiClient        ai.AIClient
	s3Client        *s3.Client
	dbName          string
	aiProvider      string
	aiModel         string
	resultSizeLimit int
}

// NewSidecarServer creates a new SidecarService gRPC server
func NewSidecarServer(db database.Database, aiClient ai.AIClient, s3Client *s3.Client, dbName, aiProvider, aiModel string, resultSizeLimit int) *SidecarServer {
	return &SidecarServer{
		db:              db,
		aiClient:        aiClient,
		s3Client:        s3Client,
		dbName:          dbName,
		aiProvider:      aiProvider,
		aiModel:         aiModel,
		resultSizeLimit: resultSizeLimit,
	}
}

// Health returns the health status of the sidecar
func (s *SidecarServer) Health(ctx context.Context, req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	return &pb.HealthCheckResponse{
		Status:     "healthy",
		Timestamp:  timestamppb.Now(),
		AiProvider: s.aiProvider,
		AiModel:    s.aiModel,
		DbType:     string(s.db.Type()),
		DbName:     s.dbName,
	}, nil
}

// ExecuteQuery handles natural language query execution
// Generates SQL with AI, executes it, and returns results (inline or S3 path)
func (s *SidecarServer) ExecuteQuery(ctx context.Context, req *pb.SidecarQueryRequest) (*pb.SidecarQueryResponse, error) {
	log.Printf("Received query: %s", req.Query)

	queryID := fmt.Sprintf("q-%d", time.Now().UnixNano())
	startTime := time.Now()

	// Get database schema
	schema, err := s.db.GetSchema(ctx, s.dbName)
	if err != nil {
		log.Printf("Failed to get schema: %v", err)
		return &pb.SidecarQueryResponse{
			QueryId: queryID,
			Error:   fmt.Sprintf("Failed to get database schema: %v", err),
		}, nil
	}

	const maxRetries = 3
	var generatedSQL string
	var results *database.QueryResult
	var lastError error

	// Retry loop for AI + query execution
	for attempt := 1; attempt <= maxRetries; attempt++ {
		log.Printf("Attempt %d/%d: Generating SQL...", attempt, maxRetries)

		// Build prompt - include previous error if retrying
		userQuery := req.Query
		if attempt > 1 && lastError != nil {
			userQuery = fmt.Sprintf("%s\n\nPREVIOUS ATTEMPT FAILED WITH ERROR: %s\nPlease fix the SQL query.", req.Query, lastError.Error())
		}

		// Generate SQL using AI
		generatedSQL, err = s.aiClient.GenerateSQL(ctx, schema, userQuery)
		if err != nil {
			lastError = fmt.Errorf("AI generation failed: %v", err)
			log.Printf("Attempt %d failed: %v", attempt, lastError)
			time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
			continue
		}

		log.Printf("Attempt %d: Generated SQL: %s", attempt, generatedSQL)

		// Validate SQL (basic check for mutations)
		if !isSelectQuery(generatedSQL) {
			lastError = fmt.Errorf("generated query is not a SELECT statement")
			log.Printf("Attempt %d failed: %v", attempt, lastError)
			continue
		}

		// Execute the generated SQL
		log.Printf("Attempt %d: Executing query...", attempt)
		results, err = s.db.ExecuteQuery(ctx, generatedSQL)
		if err != nil {
			lastError = fmt.Errorf("query execution failed: %v", err)
			log.Printf("Attempt %d failed: %v", attempt, lastError)
			time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
			continue
		}

		// Success!
		log.Printf("Query executed successfully on attempt %d, returned %d rows", attempt, results.Count)
		break
	}

	// If all retries failed
	if results == nil {
		return &pb.SidecarQueryResponse{
			QueryId:        queryID,
			GeneratedQuery: generatedSQL,
			Error:          fmt.Sprintf("Query failed after %d attempts: %v", maxRetries, lastError),
		}, nil
	}

	// Convert results to proto format
	protoResult := convertToProtoResult(results)

	// Calculate metadata
	duration := time.Since(startTime)
	metadata := &pb.QueryMetadata{
		ExecutionTime: timestamppb.New(startTime),
		DurationMs:    int32(duration.Milliseconds()),
		FromCache:     false,
		AiModelUsed:   s.aiModel,
	}

	// Check if results are large, upload to S3 if needed
	resultJSON, _ := json.Marshal(results)
	resultSize := len(resultJSON)

	if resultSize > s.resultSizeLimit && s.s3Client != nil {
		// Upload to S3 for large results
		log.Printf("Result size %d bytes exceeds limit %d bytes, uploading to S3", resultSize, s.resultSizeLimit)
		s3Path, err := s.s3Client.UploadQueryResult(ctx, queryID, results)
		if err != nil {
			log.Printf("Failed to upload to S3: %v, returning inline result", err)
			// Fallback to inline result
			return &pb.SidecarQueryResponse{
				QueryId:        queryID,
				GeneratedQuery: generatedSQL,
				ResultLocation: &pb.SidecarQueryResponse_InlineResult{
					InlineResult: protoResult,
				},
				Metadata: metadata,
			}, nil
		}

		// Return S3 path
		log.Printf("Query results uploaded to S3: %s", s3Path)
		return &pb.SidecarQueryResponse{
			QueryId:        queryID,
			GeneratedQuery: generatedSQL,
			ResultLocation: &pb.SidecarQueryResponse_S3Path{
				S3Path: s3Path,
			},
			Metadata: metadata,
		}, nil
	}

	// Return inline results for small datasets
	log.Printf("Result size %d bytes, returning inline", resultSize)
	return &pb.SidecarQueryResponse{
		QueryId:        queryID,
		GeneratedQuery: generatedSQL,
		ResultLocation: &pb.SidecarQueryResponse_InlineResult{
			InlineResult: protoResult,
		},
		Metadata: metadata,
	}, nil
}

// ExecuteDirectQuery executes a pre-generated SQL query (no AI generation)
// Used for saved queries and dashboard widgets
func (s *SidecarServer) ExecuteDirectQuery(ctx context.Context, req *pb.DirectQueryRequest) (*pb.SidecarQueryResponse, error) {
	log.Printf("Received direct query: %s", req.Sql)

	queryID := fmt.Sprintf("q-%d", time.Now().UnixNano())
	startTime := time.Now()

	// Validate SQL (basic check for mutations)
	if !isSelectQuery(req.Sql) {
		return &pb.SidecarQueryResponse{
			QueryId:        queryID,
			GeneratedQuery: req.Sql,
			Error:          "Only SELECT queries are allowed",
		}, nil
	}

	// Execute the SQL directly
	results, err := s.db.ExecuteQuery(ctx, req.Sql)
	if err != nil {
		return &pb.SidecarQueryResponse{
			QueryId:        queryID,
			GeneratedQuery: req.Sql,
			Error:          fmt.Sprintf("Query execution failed: %v", err),
		}, nil
	}

	log.Printf("Direct query executed successfully, returned %d rows", results.Count)

	// Convert results to proto format
	protoResult := convertToProtoResult(results)

	// Calculate metadata
	duration := time.Since(startTime)
	metadata := &pb.QueryMetadata{
		ExecutionTime: timestamppb.New(startTime),
		DurationMs:    int32(duration.Milliseconds()),
		FromCache:     false,
	}

	// Check if results are large, upload to S3 if needed
	resultJSON, _ := json.Marshal(results)
	resultSize := len(resultJSON)

	if resultSize > s.resultSizeLimit && s.s3Client != nil {
		// Upload to S3 for large results
		log.Printf("Result size %d bytes exceeds limit %d bytes, uploading to S3", resultSize, s.resultSizeLimit)
		s3Path, err := s.s3Client.UploadQueryResult(ctx, queryID, results)
		if err != nil {
			log.Printf("Failed to upload to S3: %v, returning inline result", err)
			// Fallback to inline result
			return &pb.SidecarQueryResponse{
				QueryId:        queryID,
				GeneratedQuery: req.Sql,
				ResultLocation: &pb.SidecarQueryResponse_InlineResult{
					InlineResult: protoResult,
				},
				Metadata: metadata,
			}, nil
		}

		// Return S3 path
		log.Printf("Query results uploaded to S3: %s", s3Path)
		return &pb.SidecarQueryResponse{
			QueryId:        queryID,
			GeneratedQuery: req.Sql,
			ResultLocation: &pb.SidecarQueryResponse_S3Path{
				S3Path: s3Path,
			},
			Metadata: metadata,
		}, nil
	}

	// Return inline results for small datasets
	log.Printf("Result size %d bytes, returning inline", resultSize)
	return &pb.SidecarQueryResponse{
		QueryId:        queryID,
		GeneratedQuery: req.Sql,
		ResultLocation: &pb.SidecarQueryResponse_InlineResult{
			InlineResult: protoResult,
		},
		Metadata: metadata,
	}, nil
}

// GetSchema retrieves the database schema
func (s *SidecarServer) GetSchema(ctx context.Context, req *pb.SidecarSchemaRequest) (*pb.SchemaResponse, error) {
	log.Printf("Received schema request")

	schema, err := s.db.GetSchema(ctx, s.dbName)
	if err != nil {
		return nil, fmt.Errorf("failed to get schema: %v", err)
	}

	// Return schema as string (used for AI context and can be parsed later if needed)
	return &pb.SchemaResponse{
		SchemaText:    schema,
		SchemaVersion: "1.0",
		LastUpdated:   timestamppb.Now(),
	}, nil
}

// Helper functions

// isSelectQuery validates that the query is a SELECT statement
func isSelectQuery(sql string) bool {
	// Simple validation - check if query starts with SELECT
	// In production, use a proper SQL parser
	trimmed := strings.TrimSpace(sql)
	upperSQL := strings.ToUpper(trimmed)
	return strings.HasPrefix(upperSQL, "SELECT")
}

// convertToProtoResult converts database.QueryResult to pb.QueryResult
func convertToProtoResult(dbResult *database.QueryResult) *pb.QueryResult {
	var protoRows []*pb.Row

	for _, row := range dbResult.Rows {
		protoRow := &pb.Row{
			Values: make(map[string]*pb.Value),
		}

		for colName, colValue := range row {
			protoRow.Values[colName] = convertToProtoValue(colValue)
		}

		protoRows = append(protoRows, protoRow)
	}

	return &pb.QueryResult{
		Columns:  dbResult.Columns,
		Rows:     protoRows,
		RowCount: int32(dbResult.Count),
	}
}

// convertToProtoValue converts a Go value to pb.Value
func convertToProtoValue(value interface{}) *pb.Value {
	if value == nil {
		return &pb.Value{IsNull: true}
	}

	switch v := value.(type) {
	case string:
		return &pb.Value{Value: &pb.Value_StringValue{StringValue: v}}
	case int, int8, int16, int32, int64:
		return &pb.Value{Value: &pb.Value_IntValue{IntValue: toInt64(v)}}
	case float32, float64:
		return &pb.Value{Value: &pb.Value_DoubleValue{DoubleValue: toFloat64(v)}}
	case bool:
		return &pb.Value{Value: &pb.Value_BoolValue{BoolValue: v}}
	case []byte:
		return &pb.Value{Value: &pb.Value_BytesValue{BytesValue: v}}
	case time.Time:
		return &pb.Value{Value: &pb.Value_TimestampValue{TimestampValue: timestamppb.New(v)}}
	default:
		// Fallback to string representation
		return &pb.Value{Value: &pb.Value_StringValue{StringValue: fmt.Sprintf("%v", v)}}
	}
}

// toInt64 converts various int types to int64
func toInt64(v interface{}) int64 {
	switch val := v.(type) {
	case int:
		return int64(val)
	case int8:
		return int64(val)
	case int16:
		return int64(val)
	case int32:
		return int64(val)
	case int64:
		return val
	default:
		return 0
	}
}

// toFloat64 converts various float types to float64
func toFloat64(v interface{}) float64 {
	switch val := v.(type) {
	case float32:
		return float64(val)
	case float64:
		return val
	default:
		return 0
	}
}

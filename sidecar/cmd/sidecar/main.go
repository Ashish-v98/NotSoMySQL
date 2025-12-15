package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/yourorg/ai-query-sidecar/internal/ai"
	"github.com/yourorg/ai-query-sidecar/internal/config"
	"github.com/yourorg/ai-query-sidecar/internal/database"
	grpcclient "github.com/yourorg/ai-query-sidecar/internal/grpc"
)

type Server struct {
	config     *config.Config
	db         database.Database
	aiClient   ai.AIClient
	grpcClient *grpcclient.Client
}

type QueryRequest struct {
	Query string `json:"query"`
}

type QueryResponse struct {
	GeneratedSQL string                `json:"generated_sql"`
	Results      *database.QueryResult `json:"results,omitempty"`
	Error        string                `json:"error,omitempty"`
}

func main() {
	ctx := context.Background()

	// Load configuration
	cfg, err := config.LoadFromEnv()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("Starting AI Query Sidecar...")
	log.Printf("AI Provider: %s", cfg.AI.Provider)
	log.Printf("Database Type: %s", cfg.Database.Type)
	log.Printf("Database: %s:%s/%s", cfg.Database.Host, cfg.Database.Port, cfg.Database.DBName)

	// Initialize database using factory
	port, _ := strconv.Atoi(cfg.Database.Port)
	db, err := database.NewDatabase(database.Config{
		Type:     database.DatabaseType(cfg.Database.Type),
		Host:     cfg.Database.Host,
		Port:     port,
		User:     cfg.Database.User,
		Password: cfg.Database.Password,
		DBName:   cfg.Database.DBName,
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	log.Printf("Connected to %s database successfully", db.Type())

	// Initialize AI client based on provider
	var aiClient ai.AIClient
	switch strings.ToLower(cfg.AI.Provider) {
	case "ollama":
		aiClient = ai.NewOllamaClient(cfg.AI.OllamaURL, cfg.AI.ModelID)
		log.Printf("Using Ollama: %s (URL: %s)", cfg.AI.ModelID, cfg.AI.OllamaURL)
	case "bedrock":
		var err error
		aiClient, err = ai.NewBedrockClient(ctx, cfg.AI.AWSRegion, cfg.AI.ModelID)
		if err != nil {
			log.Fatalf("Failed to create Bedrock client: %v", err)
		}
		log.Printf("Using Bedrock: %s (Region: %s)", cfg.AI.ModelID, cfg.AI.AWSRegion)
	default:
		log.Fatalf("Unknown AI provider: %s (use 'ollama' or 'bedrock')", cfg.AI.Provider)
	}
	log.Printf("AI client initialized successfully")

	// Initialize gRPC client for parent service (if enabled)
	var grpcClient *grpcclient.Client
	if cfg.Parent.Enabled {
		log.Printf("Connecting to parent service at %s...", cfg.Parent.Address)
		grpcClient, err = grpcclient.NewClient(
			cfg.Parent.Address,
			cfg.Sidecar.TeamID,
			cfg.Sidecar.ServiceName,
			cfg.Sidecar.Version,
			cfg.Server.Port, // HTTP port for parent callback
		)
		if err != nil {
			log.Printf("Warning: Failed to create gRPC client: %v", err)
		} else {
			// Set database info for registration
			dbPort, _ := strconv.Atoi(cfg.Database.Port)
			grpcClient.SetDatabaseInfo(db.Type(), cfg.Database.DBName, cfg.Database.Host, dbPort, 0)

			// Register with parent service
			if err := grpcClient.Register(ctx); err != nil {
				log.Printf("Warning: Failed to register with parent: %v", err)
			} else {
				// Start heartbeat loop (every 30 seconds)
				grpcClient.StartHeartbeat(30 * time.Second)
				log.Printf("Started heartbeat loop")
			}
		}
	} else {
		log.Printf("Parent service connection disabled (standalone mode)")
	}

	server := &Server{
		config:     cfg,
		db:         db,
		aiClient:   aiClient,
		grpcClient: grpcClient,
	}

	// Setup HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/health", server.healthHandler)
	mux.HandleFunc("/query", server.queryHandler)

	httpServer := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 45 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Server listening on port %s", cfg.Server.Port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Unregister from parent service
	if server.grpcClient != nil {
		if err := server.grpcClient.Unregister(ctx, "shutdown"); err != nil {
			log.Printf("Failed to unregister from parent: %v", err)
		}
		if err := server.grpcClient.Close(); err != nil {
			log.Printf("Failed to close gRPC client: %v", err)
		}
	}

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	response := map[string]interface{}{
		"status":      "healthy",
		"time":        time.Now().Format(time.RFC3339),
		"ai_provider": s.config.AI.Provider,
		"ai_model":    s.config.AI.ModelID,
		"team_id":     s.config.Sidecar.TeamID,
		"service":     s.config.Sidecar.ServiceName,
	}

	if s.grpcClient != nil {
		response["parent_connected"] = s.grpcClient.IsRegistered()
		response["sidecar_id"] = s.grpcClient.GetSidecarID()
	} else {
		response["parent_connected"] = false
	}

	json.NewEncoder(w).Encode(response)
}

func (s *Server) queryHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req QueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Query == "" {
		respondWithError(w, http.StatusBadRequest, "Query is required")
		return
	}

	log.Printf("Received query: %s", req.Query)

	ctx := r.Context()

	// Get database schema
	schema, err := s.db.GetSchema(ctx, s.config.Database.DBName)
	if err != nil {
		log.Printf("Failed to get schema: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to get database schema")
		return
	}

	const maxRetries = 3
	var generatedSQL string
	var results *database.QueryResult
	var lastError error
	var allErrors []string

	for attempt := 1; attempt <= maxRetries; attempt++ {
		log.Printf("Attempt %d/%d: Generating SQL with %s...", attempt, maxRetries, s.config.AI.Provider)

		// Build prompt - include previous error if retrying
		userQuery := req.Query
		if attempt > 1 && lastError != nil {
			userQuery = fmt.Sprintf("%s\n\nPREVIOUS ATTEMPT FAILED WITH ERROR: %s\nPlease fix the SQL query.", req.Query, lastError.Error())
		}

		// Generate SQL using AI
		generatedSQL, err = s.aiClient.GenerateSQL(ctx, schema, userQuery)
		if err != nil {
			lastError = fmt.Errorf("AI generation failed: %v", err)
			allErrors = append(allErrors, fmt.Sprintf("Attempt %d: %v", attempt, lastError))
			log.Printf("Attempt %d failed: %v", attempt, lastError)
			time.Sleep(time.Duration(attempt) * 500 * time.Millisecond) // Backoff
			continue
		}

		log.Printf("Attempt %d: Generated SQL: %s", attempt, generatedSQL)

		// Validate SQL (basic check for mutations)
		if !isSelectQuery(generatedSQL) {
			lastError = fmt.Errorf("generated query is not a SELECT statement")
			allErrors = append(allErrors, fmt.Sprintf("Attempt %d: %v", attempt, lastError))
			log.Printf("Attempt %d failed: %v", attempt, lastError)
			continue
		}

		// Execute the generated SQL
		log.Printf("Attempt %d: Executing query...", attempt)
		results, err = s.db.ExecuteQuery(ctx, generatedSQL)
		if err != nil {
			lastError = fmt.Errorf("query execution failed: %v", err)
			allErrors = append(allErrors, fmt.Sprintf("Attempt %d: %v", attempt, lastError))
			log.Printf("Attempt %d failed: %v", attempt, lastError)
			time.Sleep(time.Duration(attempt) * 500 * time.Millisecond) // Backoff
			continue
		}

		// Success!
		log.Printf("Query executed successfully on attempt %d, returned %d rows", attempt, results.Count)
		response := QueryResponse{
			GeneratedSQL: generatedSQL,
			Results:      results,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// All retries exhausted
	log.Printf("All %d attempts failed for query: %s", maxRetries, req.Query)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(QueryResponse{
		GeneratedSQL: generatedSQL,
		Error:        fmt.Sprintf("Query failed after %d attempts. Errors: %v", maxRetries, allErrors),
	})
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func isSelectQuery(sql string) bool {
	// Simple validation - check if query starts with SELECT
	// In production, use a proper SQL parser
	trimmed := strings.TrimSpace(sql)
	upperSQL := strings.ToUpper(trimmed)
	return strings.HasPrefix(upperSQL, "SELECT")
}

package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	pb "github.com/notsoMySQL/sidecar-client"
	"github.com/yourorg/ai-query-sidecar/internal/ai"
	"github.com/yourorg/ai-query-sidecar/internal/config"
	"github.com/yourorg/ai-query-sidecar/internal/database"
	grpcclient "github.com/yourorg/ai-query-sidecar/internal/grpc"
	"github.com/yourorg/ai-query-sidecar/internal/s3"
	"google.golang.org/grpc"
)

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

	// Initialize S3 client
	var s3Client *s3.Client
	if cfg.S3.Bucket != "" {
		s3Client, err = s3.NewClient(ctx, cfg.S3.Region, cfg.S3.Bucket)
		if err != nil {
			log.Printf("Warning: Failed to create S3 client: %v (large results will be returned inline)", err)
			s3Client = nil
		} else {
			log.Printf("S3 client initialized successfully (bucket: %s, region: %s)", cfg.S3.Bucket, cfg.S3.Region)
		}
	} else {
		log.Printf("S3 not configured, all results will be returned inline")
	}

	// Initialize gRPC client for parent service (if enabled)
	var grpcClient *grpcclient.Client
	if cfg.Parent.Enabled {
		log.Printf("Connecting to parent service at %s...", cfg.Parent.Address)
		grpcClient, err = grpcclient.NewClient(
			cfg.Parent.Address,
			cfg.Sidecar.TeamID,
			cfg.Sidecar.ServiceName,
			cfg.Sidecar.Version,
			cfg.Server.Port, // gtpc port for parent callback
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

	// Setup gRPC server for SidecarService
	grpcPort := os.Getenv("GRPC_PORT")
	if grpcPort == "" {
		grpcPort = cfg.Server.Port // Use same port as before (8080)
	}

	sidecarServer := grpcclient.NewSidecarServer(db, aiClient, s3Client, cfg.Database.DBName, cfg.AI.Provider, cfg.AI.ModelID, cfg.S3.ResultSizeLimit)

	grpcListener, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		log.Fatalf("Failed to listen on gRPC port %s: %v", grpcPort, err)
	}

	grpcServerOpts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(10 * 1024 * 1024), // 10MB
		grpc.MaxSendMsgSize(10 * 1024 * 1024), // 10MB
	}
	grpcSrv := grpc.NewServer(grpcServerOpts...)
	pb.RegisterSidecarServiceServer(grpcSrv, sidecarServer)

	// Start gRPC server in goroutine
	go func() {
		log.Printf("gRPC server (SidecarService) listening on port %s", grpcPort)
		if err := grpcSrv.Serve(grpcListener); err != nil {
			log.Fatalf("gRPC server error: %v", err)
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
	if grpcClient != nil {
		if err := grpcClient.Unregister(ctx, "shutdown"); err != nil {
			log.Printf("Failed to unregister from parent: %v", err)
		}
		if err := grpcClient.Close(); err != nil {
			log.Printf("Failed to close gRPC client: %v", err)
		}
	}

	// Shutdown gRPC server
	grpcSrv.GracefulStop()

	log.Println("Server exited")
}

package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/notsoMySQL/parent-service/internal/api"
	grpcServer "github.com/notsoMySQL/parent-service/internal/grpc"
	pb "github.com/notsoMySQL/sidecar-client"
	"google.golang.org/grpc"
)

func main() {
	log.Println("Starting AI Query Platform Parent Service...")

	// Get configuration from environment
	grpcPort := os.Getenv("GRPC_PORT")
	if grpcPort == "" {
		grpcPort = "9090"
	}

	restPort := os.Getenv("REST_PORT")
	if restPort == "" {
		restPort = "8090"
	}

	// Create gRPC server
	parentServer := grpcServer.NewServer()

	// Start gRPC server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", grpcPort))
	if err != nil {
		log.Fatalf("Failed to listen on port %s: %v", grpcPort, err)
	}

	opts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(10 * 1024 * 1024), // 10MB
		grpc.MaxSendMsgSize(10 * 1024 * 1024), // 10MB
	}

	grpcSrv := grpc.NewServer(opts...)
	pb.RegisterParentServiceServer(grpcSrv, parentServer)

	// Start gRPC serving in a goroutine
	go func() {
		log.Printf("gRPC server listening on port %s", grpcPort)
		if err := grpcSrv.Serve(lis); err != nil {
			log.Fatalf("Failed to serve gRPC: %v", err)
		}
	}()

	// Create and start REST API server
	restServer := api.NewRESTServer(parentServer)
	httpServer := &http.Server{
		Addr:         ":" + restPort,
		Handler:      restServer.GetRouter(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("REST API server listening on port %s", restPort)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to serve REST API: %v", err)
		}
	}()

	log.Println("Parent service started successfully")
	log.Printf("  - gRPC:     localhost:%s (for sidecars)", grpcPort)
	log.Printf("  - REST API: localhost:%s (for dashboard)", restPort)

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down parent service...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("REST server shutdown error: %v", err)
	}

	grpcSrv.GracefulStop()
	log.Println("Parent service stopped")
}

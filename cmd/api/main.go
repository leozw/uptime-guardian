package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/leozw/uptime-guardian/internal/api"
	"github.com/leozw/uptime-guardian/internal/api/handlers"
	"github.com/leozw/uptime-guardian/internal/api/middleware"
	"github.com/leozw/uptime-guardian/internal/config"
	"github.com/leozw/uptime-guardian/internal/db"
	"github.com/leozw/uptime-guardian/internal/metrics"
	"github.com/leozw/uptime-guardian/pkg/keycloak"
	"go.uber.org/zap"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Setup logger
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// Database connection
	database, err := db.NewConnection(cfg.Database.URL)
	if err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer database.Close()

	// Run migrations
	if err := db.RunMigrations(cfg.Database.URL); err != nil {
		logger.Fatal("Failed to run migrations", zap.Error(err))
	}

	// Initialize repositories
	repo := db.NewRepository(database)

	// Initialize Keycloak client
	keycloakClient := keycloak.NewClient(cfg.Keycloak)

	// Initialize metrics collector
	metricsCollector := metrics.NewCollector(cfg.Mimir)

	// Setup Gin
	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.Logger(logger))
	r.Use(middleware.CORS())

	// Setup handlers
	h := handlers.NewHandler(repo, metricsCollector, keycloakClient, logger)

	// Setup routes
	api.SetupRoutes(r, h, keycloakClient)

	// Start metrics exporter
	go metricsCollector.StartRemoteWrite(context.Background())

	// Start server
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", cfg.Server.Port),
		Handler: r,
	}

	// Graceful shutdown
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	logger.Info("Server started", zap.String("port", cfg.Server.Port))

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown", zap.Error(err))
	}

	logger.Info("Server exited")
}

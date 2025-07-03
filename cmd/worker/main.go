package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/leozw/uptime-guardian/internal/checks"
    "github.com/leozw/uptime-guardian/internal/config"
    "github.com/leozw/uptime-guardian/internal/db"
    "github.com/leozw/uptime-guardian/internal/metrics"
    "github.com/leozw/uptime-guardian/internal/scheduler"
    "go.uber.org/zap"
)

func main() {
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

    // Initialize repositories
    repo := db.NewRepository(database)

    // Initialize metrics collector
    metricsCollector := metrics.NewCollector(cfg.Mimir)

    // Initialize check runners
    checkRunners := map[string]checks.Runner{
        "http":   checks.NewHTTPChecker(),
        "ssl":    checks.NewSSLChecker(),
        "dns":    checks.NewDNSChecker(),
        "domain": checks.NewDomainChecker(),
    }

    // Initialize scheduler
    sched := scheduler.NewScheduler(repo, metricsCollector, checkRunners, logger, cfg)

    // Start scheduler
    ctx, cancel := context.WithCancel(context.Background())
    go sched.Start(ctx)

    // Start metrics exporter
    go metricsCollector.StartRemoteWrite(ctx)

    logger.Info("Worker started")

    // Wait for interrupt signal
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    logger.Info("Shutting down worker...")
    cancel()
    logger.Info("Worker exited")
}
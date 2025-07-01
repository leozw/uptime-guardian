package main

import (
    "context"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/leozw/uptime-guardian/internal/api"
    "github.com/leozw/uptime-guardian/internal/config"
    "github.com/leozw/uptime-guardian/internal/queue"
    "github.com/leozw/uptime-guardian/internal/storage/postgres"
    "github.com/leozw/uptime-guardian/internal/storage/redis"
)

func main() {
    cfg := config.Load()

    // Database
    db, err := postgres.NewConnection(cfg.DatabaseURL)
    if err != nil {
        log.Fatal("Failed to connect to database:", err)
    }
    defer db.Close()

    // Redis
    cache := redis.NewClient(cfg.RedisURL)
    defer cache.Close()

    // Queue
    jobQueue := queue.NewRedisQueue(cache.Client)

    // API Server
    server := api.NewServer(cfg, db, cache, jobQueue)

    srv := &http.Server{
        Addr:    ":" + cfg.Port,
        Handler: server.Router,
    }

    // Graceful shutdown
    go func() {
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Fatal("Failed to start server:", err)
        }
    }()

    log.Printf("API server started on port %s", cfg.Port)

    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    log.Println("Shutting down server...")

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    if err := srv.Shutdown(ctx); err != nil {
        log.Fatal("Server forced to shutdown:", err)
    }

    log.Println("Server exited")
}
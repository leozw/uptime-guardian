package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/google/uuid"
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

    ctx, cancel := context.WithCancel(context.Background())

    // Schedule checks every minute
    ticker := time.NewTicker(1 * time.Minute)
    defer ticker.Stop()

    go func() {
        for {
            select {
            case <-ctx.Done():
                return
            case <-ticker.C:
                scheduleDomainChecks(db, jobQueue)
            }
        }
    }()

    log.Println("Scheduler started")

    // Graceful shutdown
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    log.Println("Shutting down scheduler...")
    cancel()
    log.Println("Scheduler stopped")
}

func scheduleDomainChecks(db *postgres.DB, jobQueue *queue.RedisQueue) {
    domains, err := db.GetDomainsToCheck()
    if err != nil {
        log.Printf("Failed to get domains: %v", err)
        return
    }

    for _, domain := range domains {
        job := &queue.Job{
            ID:        uuid.New().String(),
            Type:      "domain_check",
            DomainID:  domain.ID.String(),
            TenantID:  domain.TenantID.String(),
            CreatedAt: time.Now(),
        }

        if err := jobQueue.Push(context.Background(), job); err != nil {
            log.Printf("Failed to queue job for domain %s: %v", domain.Name, err)
        }
    }

    log.Printf("Scheduled %d domain checks", len(domains))
}
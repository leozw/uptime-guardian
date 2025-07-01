package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "sync"
    "syscall"
    "time"

    "github.com/leozw/uptime-guardian/internal/checker"
    "github.com/leozw/uptime-guardian/internal/config"
    "github.com/leozw/uptime-guardian/internal/metrics"
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

    // Metrics
    metricsClient := metrics.NewMimirClient(cfg.MimirURL)

    // Analyzer
    analyzer := checker.NewAnalyzer()

    // Worker pool
    ctx, cancel := context.WithCancel(context.Background())
    var wg sync.WaitGroup

    numWorkers := cfg.WorkerCount
    if numWorkers == 0 {
        numWorkers = 5
    }

    for i := 0; i < numWorkers; i++ {
        wg.Add(1)
        go func(workerID int) {
            defer wg.Done()
            log.Printf("Worker %d started", workerID)

            for {
                select {
                case <-ctx.Done():
                    log.Printf("Worker %d stopping", workerID)
                    return
                default:
                    job, err := jobQueue.Pop(ctx, 5*time.Second)
                    if err != nil {
                        if err != queue.ErrTimeout {
                            log.Printf("Worker %d error: %v", workerID, err)
                        }
                        continue
                    }

                    processJob(job, db, analyzer, metricsClient)
                }
            }
        }(i)
    }

    // Graceful shutdown
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    log.Println("Shutting down workers...")
    cancel()
    wg.Wait()
    log.Println("All workers stopped")
}

func processJob(job *queue.Job, db *postgres.DB, analyzer *checker.Analyzer, metrics *metrics.MimirClient) {
    log.Printf("Processing job: %s for domain %s", job.Type, job.DomainID)

    domain, err := db.GetDomain(job.DomainID, job.TenantID)
    if err != nil {
        log.Printf("Failed to get domain: %v", err)
        return
    }

    results, err := analyzer.AnalyzeDomain(domain.Name, job.TenantID)
    if err != nil {
        log.Printf("Failed to analyze domain: %v", err)
        return
    }

    // Save results
    if err := db.SaveCheckResults(domain.ID, results); err != nil {
        log.Printf("Failed to save results: %v", err)
        return
    }

    // Send metrics
    if err := metrics.SendDomainMetrics(job.TenantID, domain.Name, results); err != nil {
        log.Printf("Failed to send metrics: %v", err)
    }

    log.Printf("Job completed for domain %s", domain.Name)
}
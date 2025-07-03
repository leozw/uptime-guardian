package scheduler

import (
	"context"
	"sync"
	"time"

	"github.com/leozw/uptime-guardian/internal/checks"
	"github.com/leozw/uptime-guardian/internal/config"
	"github.com/leozw/uptime-guardian/internal/db"
	"github.com/leozw/uptime-guardian/internal/metrics"
	"go.uber.org/zap"
)

type Scheduler struct {
	repo         *db.Repository
	metrics      *metrics.Collector
	checkRunners map[string]checks.Runner
	logger       *zap.Logger
	config       *config.Config
	workers      []*Worker
	wg           sync.WaitGroup
}

func NewScheduler(repo *db.Repository, metrics *metrics.Collector, runners map[string]checks.Runner, logger *zap.Logger, cfg *config.Config) *Scheduler {
	return &Scheduler{
		repo:         repo,
		metrics:      metrics,
		checkRunners: runners,
		logger:       logger,
		config:       cfg,
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	s.logger.Info("Starting scheduler", zap.Int("worker_count", s.config.Scheduler.WorkerCount))

	// Start workers
	workQueue := make(chan *CheckJob, 1000)
	s.workers = make([]*Worker, s.config.Scheduler.WorkerCount)

	for i := 0; i < s.config.Scheduler.WorkerCount; i++ {
		worker := NewWorker(i, workQueue, s.repo, s.metrics, s.checkRunners, s.logger)
		s.workers[i] = worker
		s.wg.Add(1)
		go func(w *Worker) {
			defer s.wg.Done()
			w.Start(ctx)
		}(worker)
	}

	// Schedule checks
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Stopping scheduler")
			close(workQueue)
			s.wg.Wait()
			return
		case <-ticker.C:
			s.scheduleChecks(workQueue)
		}
	}
}

func (s *Scheduler) scheduleChecks(workQueue chan<- *CheckJob) {
	monitors, err := s.repo.GetMonitorsToCheck()
	if err != nil {
		s.logger.Error("Failed to get monitors to check", zap.Error(err))
		return
	}

	for _, monitor := range monitors {
		// Create job for each region
		for _, region := range monitor.Regions {
			job := &CheckJob{
				Monitor: monitor,
				Region:  region,
			}

			select {
			case workQueue <- job:
				s.logger.Debug("Scheduled check",
					zap.String("monitor_id", monitor.ID),
					zap.String("region", region),
				)
			default:
				s.logger.Warn("Work queue full, dropping check",
					zap.String("monitor_id", monitor.ID),
					zap.String("region", region),
				)
			}
		}
	}
}

type CheckJob struct {
	Monitor *db.Monitor
	Region  string
}

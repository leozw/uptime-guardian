package scheduler

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/leozw/uptime-guardian/internal/checks"
	"github.com/leozw/uptime-guardian/internal/db"
	"github.com/leozw/uptime-guardian/internal/incidents"
	"github.com/leozw/uptime-guardian/internal/metrics"
	"go.uber.org/zap"
)

type Worker struct {
	id              int
	workQueue       <-chan *CheckJob
	repo            *db.Repository
	metrics         *metrics.Collector
	checkRunners    map[string]checks.Runner
	logger          *zap.Logger
	incidentService *incidents.Service
}

func NewWorker(id int, workQueue <-chan *CheckJob, repo *db.Repository, metrics *metrics.Collector, runners map[string]checks.Runner, logger *zap.Logger) *Worker {
	return &Worker{
		id:              id,
		workQueue:       workQueue,
		repo:            repo,
		metrics:         metrics,
		checkRunners:    runners,
		logger:          logger.With(zap.Int("worker_id", id)),
		incidentService: incidents.NewService(repo, logger),
	}
}

func (w *Worker) Start(ctx context.Context) {
	w.logger.Info("Worker started")

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("Worker stopped")
			return
		case job, ok := <-w.workQueue:
			if !ok {
				w.logger.Info("Work queue closed")
				return
			}
			w.processJob(job)
		}
	}
}

func (w *Worker) processJob(job *CheckJob) {
	start := time.Now()

	w.logger.Debug("Processing check",
		zap.String("monitor_id", job.Monitor.ID),
		zap.String("monitor_type", string(job.Monitor.Type)),
		zap.String("region", job.Region),
	)

	// Get appropriate checker
	checker, ok := w.checkRunners[string(job.Monitor.Type)]
	if !ok {
		w.logger.Error("No checker found for monitor type",
			zap.String("monitor_type", string(job.Monitor.Type)),
		)
		return
	}

	// Execute check
	result := checker.Check(job.Monitor, job.Region)
	result.ID = uuid.New().String()
	result.CheckedAt = time.Now()

	// Save result
	if err := w.repo.SaveCheckResult(result); err != nil {
		w.logger.Error("Failed to save check result",
			zap.Error(err),
			zap.String("monitor_id", job.Monitor.ID),
		)
		return
	}

	// Record metrics
	w.metrics.RecordCheck(result, job.Monitor)

	// Process incidents
	if err := w.incidentService.CreateOrUpdateIncident(job.Monitor, result); err != nil {
		w.logger.Error("Failed to process incident",
			zap.Error(err),
			zap.String("monitor_id", job.Monitor.ID),
		)
	}

	// Process notifications if needed
	if result.Status == db.StatusDown {
		w.processNotifications(job.Monitor, result)
	}

	w.logger.Debug("Check completed",
		zap.String("monitor_id", job.Monitor.ID),
		zap.String("status", string(result.Status)),
		zap.Duration("duration", time.Since(start)),
	)
}

func (w *Worker) processNotifications(monitor *db.Monitor, result *db.CheckResult) {
	// TODO: Implement notification logic
	// Check if we need to send notifications based on:
	// - monitor.NotificationConf.OnFailureCount
	// - Previous check results
	// - Active incidents

	w.logger.Info("Processing notifications",
		zap.String("monitor_id", monitor.ID),
		zap.String("status", string(result.Status)),
	)

	// For now, just log that we would send notifications
	if monitor.NotificationConf.Channels != nil {
		for _, channel := range monitor.NotificationConf.Channels {
			if channel.Enabled {
				w.logger.Info("Would send notification",
					zap.String("channel_type", channel.Type),
					zap.String("monitor_id", monitor.ID),
					zap.String("monitor_name", monitor.Name),
					zap.String("status", string(result.Status)),
				)
			}
		}
	}
}

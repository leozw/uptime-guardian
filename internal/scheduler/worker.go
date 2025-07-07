package scheduler

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/leozw/uptime-guardian/internal/checks"
	"github.com/leozw/uptime-guardian/internal/db"
	"github.com/leozw/uptime-guardian/internal/groups"
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
	groupService    *groups.Service
}

func NewWorker(id int, workQueue <-chan *CheckJob, repo *db.Repository, metrics *metrics.Collector, runners map[string]checks.Runner, logger *zap.Logger) *Worker {
	return &Worker{
		id:              id,
		workQueue:       workQueue,
		repo:            repo,
		metrics:         metrics,
		checkRunners:    runners,
		logger:          logger.With(zap.Int("worker_id", id)),
		incidentService: incidents.NewService(repo, logger, metrics),
		groupService:    groups.NewService(repo, logger, metrics),
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

	// Update groups this monitor belongs to - CORRIGIDO
	groups, err := w.repo.GetMonitorGroups(job.Monitor.ID)
	if err != nil {
		w.logger.Debug("No groups found for monitor or error getting groups",
			zap.String("monitor_id", job.Monitor.ID),
			zap.Error(err),
		)
	} else {
		w.logger.Debug("Found groups for monitor",
			zap.String("monitor_id", job.Monitor.ID),
			zap.Int("groups_count", len(groups)),
		)

		for _, group := range groups {
			w.logger.Debug("Processing group update",
				zap.String("group_id", group.ID),
				zap.String("group_name", group.Name),
				zap.String("group_tenant_id", group.TenantID),
				zap.String("monitor_id", job.Monitor.ID),
			)

			// CORREÇÃO: Passar o tenant_id do monitor (que é correto)
			// ao invés de string vazia ou o tenant_id do grupo
			if err := w.groupService.UpdateGroupStatus(group.ID, job.Monitor.TenantID); err != nil {
				// Log como warning ao invés de error, para não poluir os logs
				w.logger.Warn("Failed to update group status",
					zap.Error(err),
					zap.String("group_id", group.ID),
					zap.String("group_name", group.Name),
					zap.String("monitor_id", job.Monitor.ID),
				)
			} else {
				w.logger.Debug("Successfully updated group status",
					zap.String("group_id", group.ID),
					zap.String("group_name", group.Name),
					zap.String("monitor_id", job.Monitor.ID),
				)
			}
		}
	}

	// Process notifications if needed
	if result.Status == db.StatusDown || result.Status == db.StatusDegraded {
		w.processNotifications(job.Monitor, result)
	}

	w.logger.Debug("Check completed",
		zap.String("monitor_id", job.Monitor.ID),
		zap.String("status", string(result.Status)),
		zap.Duration("duration", time.Since(start)),
	)
}

func (w *Worker) processNotifications(monitor *db.Monitor, result *db.CheckResult) {
	notificationStart := time.Now()

	w.logger.Info("Processing notifications",
		zap.String("monitor_id", monitor.ID),
		zap.String("status", string(result.Status)),
	)

	// Get incident information
	incident, err := w.repo.GetActiveIncident(monitor.ID)
	if err != nil {
		w.logger.Error("Failed to get active incident for notifications", zap.Error(err))
		return
	}

	// Check if we should send notification based on failure count
	if incident != nil && incident.AffectedChecks >= monitor.NotificationConf.OnFailureCount {
		// Check if we've already sent notifications
		if incident.NotificationsSent == 0 ||
			(monitor.NotificationConf.ReminderInterval > 0 &&
				incident.AffectedChecks%monitor.NotificationConf.ReminderInterval == 0) {

			for _, channel := range monitor.NotificationConf.Channels {
				if channel.Enabled {
					// Simulate notification sending
					success := w.sendNotification(channel, monitor, result, incident)

					// Record notification metrics
					latency := time.Since(notificationStart).Seconds()
					w.metrics.RecordNotificationSent(
						monitor.TenantID,
						monitor.ID,
						channel.Type,
						success,
						latency,
					)

					if success {
						incident.NotificationsSent++
					}
				}
			}

			// Update incident with notification count
			if err := w.repo.UpdateIncident(incident); err != nil {
				w.logger.Error("Failed to update incident notification count", zap.Error(err))
			}
		}
	}
}

func (w *Worker) sendNotification(channel db.NotificationChannel, monitor *db.Monitor, result *db.CheckResult, incident *db.Incident) bool {
	// TODO: Implement actual notification sending based on channel type
	// For now, just log and simulate

	w.logger.Info("Sending notification",
		zap.String("channel_type", channel.Type),
		zap.String("monitor_id", monitor.ID),
		zap.String("monitor_name", monitor.Name),
		zap.String("status", string(result.Status)),
		zap.String("incident_id", incident.ID),
		zap.Int("downtime_minutes", incident.DowntimeMinutes),
	)

	// Simulate success/failure (90% success rate)
	return time.Now().UnixNano()%10 != 0
}

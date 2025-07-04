package incidents

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/leozw/uptime-guardian/internal/db"
	"github.com/leozw/uptime-guardian/internal/metrics"
	"go.uber.org/zap"
)

type Service struct {
	repo    *db.Repository
	logger  *zap.Logger
	metrics *metrics.Collector
}

func NewService(repo *db.Repository, logger *zap.Logger, metrics *metrics.Collector) *Service {
	return &Service{
		repo:    repo,
		logger:  logger,
		metrics: metrics,
	}
}

// CreateOrUpdateIncident cria um novo incidente ou atualiza um existente
func (s *Service) CreateOrUpdateIncident(monitor *db.Monitor, result *db.CheckResult) error {
	// Verifica se já existe um incidente aberto
	activeIncident, err := s.repo.GetActiveIncident(monitor.ID)
	if err != nil && err.Error() != "no active incident" {
		return fmt.Errorf("failed to get active incident: %w", err)
	}

	if result.Status == db.StatusDown || result.Status == db.StatusDegraded {
		if activeIncident == nil {
			// Criar novo incidente
			incident := &db.Incident{
				ID:                uuid.New().String(),
				MonitorID:         monitor.ID,
				TenantID:          monitor.TenantID,
				StartedAt:         result.CheckedAt,
				Severity:          s.determineSeverity(result.Status),
				DowntimeMinutes:   0,
				AffectedChecks:    1,
				NotificationsSent: 0,
			}

			if err := s.repo.CreateIncident(incident); err != nil {
				return fmt.Errorf("failed to create incident: %w", err)
			}

			// Criar evento de detecção
			event := &db.IncidentEvent{
				ID:          uuid.New().String(),
				IncidentID:  incident.ID,
				EventType:   db.IncidentEventDetected,
				EventTime:   time.Now(),
				Description: fmt.Sprintf("Monitor down detected: %s", result.Error),
				Metadata: map[string]interface{}{
					"status_code":      result.StatusCode,
					"response_time_ms": result.ResponseTimeMs,
					"error":            result.Error,
				},
			}

			if err := s.repo.CreateIncidentEvent(event); err != nil {
				s.logger.Error("Failed to create incident event", zap.Error(err))
			}

			// Record metrics for new incident
			s.metrics.RecordIncidentCreated(incident, monitor)

			s.logger.Info("Created new incident",
				zap.String("incident_id", incident.ID),
				zap.String("monitor_id", monitor.ID),
			)
		} else {
			// Atualizar incidente existente
			activeIncident.AffectedChecks++
			activeIncident.DowntimeMinutes = int(time.Since(activeIncident.StartedAt).Minutes())

			if err := s.repo.UpdateIncident(activeIncident); err != nil {
				return fmt.Errorf("failed to update incident: %w", err)
			}
		}
	} else if result.Status == db.StatusUp && activeIncident != nil {
		// Resolver incidente
		now := time.Now()
		activeIncident.ResolvedAt = &now
		activeIncident.DowntimeMinutes = int(now.Sub(activeIncident.StartedAt).Minutes())

		if err := s.repo.UpdateIncident(activeIncident); err != nil {
			return fmt.Errorf("failed to resolve incident: %w", err)
		}

		// Criar evento de resolução
		event := &db.IncidentEvent{
			ID:          uuid.New().String(),
			IncidentID:  activeIncident.ID,
			EventType:   db.IncidentEventResolved,
			EventTime:   now,
			Description: "Monitor recovered and is now operational",
			Metadata: map[string]interface{}{
				"downtime_minutes": activeIncident.DowntimeMinutes,
				"affected_checks":  activeIncident.AffectedChecks,
			},
		}

		if err := s.repo.CreateIncidentEvent(event); err != nil {
			s.logger.Error("Failed to create resolution event", zap.Error(err))
		}

		// Record metrics for resolved incident
		s.metrics.RecordIncidentResolved(activeIncident, monitor)

		s.logger.Info("Resolved incident",
			zap.String("incident_id", activeIncident.ID),
			zap.String("monitor_id", monitor.ID),
			zap.Int("downtime_minutes", activeIncident.DowntimeMinutes),
		)
	}

	return nil
}

func (s *Service) determineSeverity(status db.CheckStatus) string {
	switch status {
	case db.StatusDown:
		return "critical"
	case db.StatusDegraded:
		return "warning"
	default:
		return "info"
	}
}

// AcknowledgeIncident marca um incidente como reconhecido
func (s *Service) AcknowledgeIncident(incidentID, tenantID, userEmail string) error {
	incident, err := s.repo.GetIncident(incidentID, tenantID)
	if err != nil {
		return fmt.Errorf("failed to get incident: %w", err)
	}

	if incident.AcknowledgedAt != nil {
		return fmt.Errorf("incident already acknowledged")
	}

	// Get monitor for metrics
	monitor, err := s.repo.GetMonitorByID(incident.MonitorID)
	if err != nil {
		s.logger.Error("Failed to get monitor for metrics", zap.Error(err))
	}

	now := time.Now()
	incident.AcknowledgedAt = &now
	incident.AcknowledgedBy = &userEmail

	if err := s.repo.UpdateIncident(incident); err != nil {
		return fmt.Errorf("failed to acknowledge incident: %w", err)
	}

	// Criar evento de acknowledgment
	event := &db.IncidentEvent{
		ID:          uuid.New().String(),
		IncidentID:  incident.ID,
		EventType:   db.IncidentEventAcknowledged,
		EventTime:   now,
		Description: fmt.Sprintf("Incident acknowledged by %s", userEmail),
		CreatedBy:   &userEmail,
	}

	if err := s.repo.CreateIncidentEvent(event); err != nil {
		return err
	}

	// Record acknowledgment metrics
	if monitor != nil {
		s.metrics.RecordIncidentAcknowledged(incident, monitor)
	}

	return nil
}

// AddIncidentComment adiciona um comentário ao incidente
func (s *Service) AddIncidentComment(incidentID, tenantID, userEmail, comment string) error {
	incident, err := s.repo.GetIncident(incidentID, tenantID)
	if err != nil {
		return fmt.Errorf("failed to get incident: %w", err)
	}

	event := &db.IncidentEvent{
		ID:          uuid.New().String(),
		IncidentID:  incident.ID,
		EventType:   db.IncidentEventComment,
		EventTime:   time.Now(),
		Description: comment,
		CreatedBy:   &userEmail,
	}

	return s.repo.CreateIncidentEvent(event)
}

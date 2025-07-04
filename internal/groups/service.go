package groups

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

// UpdateGroupStatus calculates and updates the status for a monitor group
func (s *Service) UpdateGroupStatus(groupID string) error {
	// Get group
	group, err := s.repo.GetMonitorGroup(groupID, "")
	if err != nil {
		return fmt.Errorf("failed to get monitor group: %w", err)
	}

	// Get members
	members, err := s.repo.GetGroupMembers(groupID)
	if err != nil {
		return fmt.Errorf("failed to get group members: %w", err)
	}

	if len(members) == 0 {
		// No members, set status as unknown
		status := &db.MonitorGroupStatus{
			GroupID:       groupID,
			OverallStatus: db.StatusDegraded,
			HealthScore:   0,
			LastCheck:     time.Now(),
			Message:       "No monitors in group",
		}
		return s.repo.SaveGroupStatus(status)
	}

	// Calculate status
	status := s.calculateGroupStatus(group, members)

	// Save status
	if err := s.repo.SaveGroupStatus(status); err != nil {
		return fmt.Errorf("failed to save group status: %w", err)
	}

	// Record metrics
	s.recordGroupMetrics(group, status)

	// Check for incidents
	if err := s.checkGroupIncidents(group, status); err != nil {
		s.logger.Error("Failed to check group incidents", zap.Error(err))
	}

	return nil
}

func (s *Service) calculateGroupStatus(group *db.MonitorGroup, members []*db.MonitorGroupMember) *db.MonitorGroupStatus {
	status := &db.MonitorGroupStatus{
		GroupID:   group.ID,
		LastCheck: time.Now(),
	}

	// Get status for each member
	memberStatuses := make(map[string]db.CheckStatus)
	memberDetails := make(map[string]*db.MonitorStatus)

	for _, member := range members {
		monitorStatus, err := s.repo.GetMonitorStatus(member.MonitorID, group.TenantID)
		if err == nil {
			memberStatuses[member.MonitorID] = monitorStatus.Status
			memberDetails[member.MonitorID] = monitorStatus

			switch monitorStatus.Status {
			case db.StatusUp:
				status.MonitorsUp++
			case db.StatusDown:
				status.MonitorsDown++
				if member.IsCritical {
					status.CriticalMonitorsDown++
				}
			case db.StatusDegraded:
				status.MonitorsDegraded++
			}
		} else {
			// If we can't get status, assume degraded
			memberStatuses[member.MonitorID] = db.StatusDegraded
			status.MonitorsDegraded++
		}
	}

	// Convert members for calculation
	group.Members = make([]db.MonitorGroupMember, len(members))
	for i, m := range members {
		group.Members[i] = *m
	}

	// Calculate health score and overall status
	status.HealthScore = group.CalculateHealthScore(memberStatuses)
	status.OverallStatus = group.DetermineOverallStatus(memberStatuses)

	// Generate detailed message
	status.Message = s.generateStatusMessage(status, members, memberDetails)

	return status
}

func (s *Service) generateStatusMessage(status *db.MonitorGroupStatus, members []*db.MonitorGroupMember, memberDetails map[string]*db.MonitorStatus) string {
	if status.OverallStatus == db.StatusUp {
		return "All monitors operational"
	}

	if status.CriticalMonitorsDown > 0 {
		// Find which critical monitors are down
		var criticalDown []string
		for _, member := range members {
			if member.IsCritical {
				if detail, ok := memberDetails[member.MonitorID]; ok && detail.Status == db.StatusDown {
					if member.Monitor != nil {
						criticalDown = append(criticalDown, member.Monitor.Name)
					}
				}
			}
		}

		if len(criticalDown) > 0 {
			return fmt.Sprintf("Critical monitors down: %v", criticalDown)
		}
		return "Critical monitors are down"
	}

	if status.MonitorsDown > 0 {
		return fmt.Sprintf("%d monitor(s) down, %d operational", status.MonitorsDown, status.MonitorsUp)
	}

	if status.MonitorsDegraded > 0 {
		return fmt.Sprintf("%d monitor(s) degraded", status.MonitorsDegraded)
	}

	return "Unknown status"
}

func (s *Service) recordGroupMetrics(group *db.MonitorGroup, status *db.MonitorGroupStatus) {
	// Record health score
	s.metrics.RecordGroupHealthScore(group.TenantID, group.ID, group.Name, status.HealthScore)

	// Record overall status
	statusValue := 0.0
	if status.OverallStatus == db.StatusUp {
		statusValue = 1.0
	} else if status.OverallStatus == db.StatusDegraded {
		statusValue = 0.5
	}
	s.metrics.RecordGroupStatus(group.TenantID, group.ID, group.Name, statusValue)

	// Record monitor counts
	s.metrics.RecordGroupMonitorCounts(
		group.TenantID,
		group.ID,
		group.Name,
		status.MonitorsUp,
		status.MonitorsDown,
		status.MonitorsDegraded,
		status.CriticalMonitorsDown,
	)
}

func (s *Service) checkGroupIncidents(group *db.MonitorGroup, status *db.MonitorGroupStatus) error {
	// Get active incident if any
	activeIncident, err := s.repo.GetActiveGroupIncident(group.ID)
	if err != nil && err.Error() != "no active group incident" {
		return fmt.Errorf("failed to get active incident: %w", err)
	}

	// Check alert rules
	rules, err := s.repo.GetGroupAlertRules(group.ID)
	if err != nil {
		return fmt.Errorf("failed to get alert rules: %w", err)
	}

	shouldAlert := false
	var triggeredRule *db.MonitorGroupAlertRule

	for _, rule := range rules {
		if s.evaluateAlertRule(rule, status) {
			shouldAlert = true
			triggeredRule = rule
			break
		}
	}

	if shouldAlert && activeIncident == nil {
		// Create new incident
		incident := &db.MonitorGroupIncident{
			ID:                 uuid.New().String(),
			GroupID:            group.ID,
			TenantID:           group.TenantID,
			StartedAt:          time.Now(),
			Severity:           s.determineSeverity(status),
			HealthScoreAtStart: &status.HealthScore,
			NotificationsSent:  0,
		}

		// Collect affected monitors
		affectedMonitors := []string{}
		members, _ := s.repo.GetGroupMembers(group.ID)
		for _, member := range members {
			monitorStatus, err := s.repo.GetMonitorStatus(member.MonitorID, group.TenantID)
			if err == nil && (monitorStatus.Status == db.StatusDown || monitorStatus.Status == db.StatusDegraded) {
				affectedMonitors = append(affectedMonitors, member.MonitorID)
			}
		}
		incident.AffectedMonitors = affectedMonitors

		if err := s.repo.CreateGroupIncident(incident); err != nil {
			return fmt.Errorf("failed to create group incident: %w", err)
		}

		// Send notifications
		if triggeredRule != nil {
			s.sendGroupNotifications(group, incident, triggeredRule, status)
		}

		// Record metrics
		s.metrics.RecordGroupIncidentCreated(incident, group)

		s.logger.Info("Created new group incident",
			zap.String("incident_id", incident.ID),
			zap.String("group_id", group.ID),
			zap.Float64("health_score", status.HealthScore),
		)

	} else if !shouldAlert && activeIncident != nil {
		// Resolve incident
		now := time.Now()
		activeIncident.ResolvedAt = &now

		if err := s.repo.UpdateGroupIncident(activeIncident); err != nil {
			return fmt.Errorf("failed to resolve incident: %w", err)
		}

		// Record metrics
		s.metrics.RecordGroupIncidentResolved(activeIncident, group)

		s.logger.Info("Resolved group incident",
			zap.String("incident_id", activeIncident.ID),
			zap.String("group_id", group.ID),
		)
	}

	return nil
}

func (s *Service) evaluateAlertRule(rule *db.MonitorGroupAlertRule, status *db.MonitorGroupStatus) bool {
	switch rule.TriggerCondition {
	case db.TriggerHealthScoreBelow:
		if rule.ThresholdValue != nil {
			return status.HealthScore < *rule.ThresholdValue
		}
	case db.TriggerAnyCriticalDown:
		return status.CriticalMonitorsDown > 0
	case db.TriggerPercentageDown:
		if rule.ThresholdValue != nil {
			totalMonitors := status.MonitorsUp + status.MonitorsDown + status.MonitorsDegraded
			if totalMonitors > 0 {
				downPercentage := float64(status.MonitorsDown) / float64(totalMonitors) * 100
				return downPercentage >= *rule.ThresholdValue
			}
		}
	case db.TriggerAllDown:
		return status.MonitorsUp == 0 && status.MonitorsDown > 0
	}
	return false
}

func (s *Service) determineSeverity(status *db.MonitorGroupStatus) string {
	if status.CriticalMonitorsDown > 0 || status.HealthScore < 50 {
		return "critical"
	}
	if status.MonitorsDown > 0 || status.HealthScore < 80 {
		return "warning"
	}
	return "info"
}

func (s *Service) sendGroupNotifications(group *db.MonitorGroup, incident *db.MonitorGroupIncident, rule *db.MonitorGroupAlertRule, status *db.MonitorGroupStatus) {
	// Use rule's notification channels if available, otherwise use group's default
	channels := rule.NotificationChannels
	if len(channels) == 0 && group.NotificationConf.Channels != nil {
		channels = group.NotificationConf.Channels
	}

	for _, channel := range channels {
		if channel.Enabled {
			// TODO: Implement actual notification sending
			s.logger.Info("Sending group notification",
				zap.String("channel_type", channel.Type),
				zap.String("group_id", group.ID),
				zap.String("group_name", group.Name),
				zap.String("incident_id", incident.ID),
				zap.Float64("health_score", status.HealthScore),
				zap.String("message", status.Message),
			)

			// Record notification metrics
			s.metrics.RecordNotificationSent(
				group.TenantID,
				group.ID,
				channel.Type,
				true, // Simulate success
				0.1,  // Simulated latency
			)
		}
	}

	// Update incident notification count
	incident.NotificationsSent++
	s.repo.UpdateGroupIncident(incident)
}

// UpdateAllGroupStatuses updates status for all groups in a tenant
func (s *Service) UpdateAllGroupStatuses(tenantID string) error {
	groups, err := s.repo.GetMonitorGroupsByTenant(tenantID, 1000, 0)
	if err != nil {
		return fmt.Errorf("failed to get monitor groups: %w", err)
	}

	for _, group := range groups {
		if group.Enabled {
			if err := s.UpdateGroupStatus(group.ID); err != nil {
				s.logger.Error("Failed to update group status",
					zap.String("group_id", group.ID),
					zap.Error(err),
				)
			}
		}
	}

	return nil
}

// CalculateGroupSLA calculates SLA for a monitor group
func (s *Service) CalculateGroupSLA(groupID string, periodStart, periodEnd time.Time) (*db.MonitorGroupSLAReport, error) {
	group, err := s.repo.GetMonitorGroup(groupID, "")
	if err != nil {
		return nil, fmt.Errorf("failed to get monitor group: %w", err)
	}

	// Get SLO configuration
	slo, err := s.repo.GetGroupSLO(groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to get group SLO: %w", err)
	}

	// Get members
	members, err := s.repo.GetGroupMembers(groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to get group members: %w", err)
	}

	// Calculate based on method
	var uptimePercentage float64
	var healthScoreSum float64
	var dataPoints int

	switch slo.CalculationMethod {
	case db.CalculationMethodWeightedAverage:
		// Calculate weighted average of member uptimes
		totalWeight := 0.0
		weightedUptime := 0.0

		for _, member := range members {
			// Get member's SLA for the period
			checks, err := s.repo.GetCheckResultsInPeriod(member.MonitorID, periodStart, periodEnd)
			if err != nil {
				continue
			}

			upCount := 0
			for _, check := range checks {
				if check.Status == db.StatusUp {
					upCount++
				}
			}

			if len(checks) > 0 {
				memberUptime := float64(upCount) / float64(len(checks)) * 100
				weightedUptime += memberUptime * member.Weight
				totalWeight += member.Weight
			}
		}

		if totalWeight > 0 {
			uptimePercentage = weightedUptime / totalWeight
		}

	case db.CalculationMethodWorstCase:
		// Take the worst performing monitor
		worstUptime := 100.0

		for _, member := range members {
			checks, err := s.repo.GetCheckResultsInPeriod(member.MonitorID, periodStart, periodEnd)
			if err != nil {
				continue
			}

			upCount := 0
			for _, check := range checks {
				if check.Status == db.StatusUp {
					upCount++
				}
			}

			if len(checks) > 0 {
				memberUptime := float64(upCount) / float64(len(checks)) * 100
				if memberUptime < worstUptime {
					worstUptime = memberUptime
				}
			}
		}

		uptimePercentage = worstUptime

	case db.CalculationMethodCriticalOnly:
		// Only consider critical monitors
		criticalCount := 0
		criticalUptime := 0.0

		for _, member := range members {
			if !member.IsCritical {
				continue
			}

			checks, err := s.repo.GetCheckResultsInPeriod(member.MonitorID, periodStart, periodEnd)
			if err != nil {
				continue
			}

			upCount := 0
			for _, check := range checks {
				if check.Status == db.StatusUp {
					upCount++
				}
			}

			if len(checks) > 0 {
				memberUptime := float64(upCount) / float64(len(checks)) * 100
				criticalUptime += memberUptime
				criticalCount++
			}
		}

		if criticalCount > 0 {
			uptimePercentage = criticalUptime / float64(criticalCount)
		}
	}

	// Calculate average health score
	// This would need to be calculated from historical status snapshots
	// For now, we'll use current health score as approximation
	currentStatus, err := s.repo.GetGroupStatus(groupID)
	if err == nil {
		healthScoreSum = currentStatus.HealthScore
		dataPoints = 1
	}

	// Count incidents in period
	incidents, err := s.repo.GetGroupIncidents(groupID, group.TenantID, 1000)
	incidentCount := 0
	downtimeMinutes := 0

	for _, incident := range incidents {
		if incident.StartedAt.After(periodStart) && incident.StartedAt.Before(periodEnd) {
			incidentCount++

			// Calculate downtime
			endTime := periodEnd
			if incident.ResolvedAt != nil && incident.ResolvedAt.Before(periodEnd) {
				endTime = *incident.ResolvedAt
			}
			downtimeMinutes += int(endTime.Sub(incident.StartedAt).Minutes())
		}
	}

	// Average health score
	avgHealthScore := 0.0
	if dataPoints > 0 {
		avgHealthScore = healthScoreSum / float64(dataPoints)
	}

	// Check if SLO was met
	sloMet := uptimePercentage >= slo.TargetUptimePercentage

	report := &db.MonitorGroupSLAReport{
		ID:                 uuid.New().String(),
		GroupID:            groupID,
		TenantID:           group.TenantID,
		PeriodStart:        periodStart,
		PeriodEnd:          periodEnd,
		HealthScoreAverage: avgHealthScore,
		UptimePercentage:   uptimePercentage,
		DowntimeMinutes:    downtimeMinutes,
		IncidentsCount:     incidentCount,
		SLOMet:             sloMet,
		CreatedAt:          time.Now(),
	}

	return report, nil
}

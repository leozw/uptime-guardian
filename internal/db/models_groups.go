package db

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

// MonitorGroup represents a logical grouping of monitors
type MonitorGroup struct {
	ID               string             `json:"id" db:"id"`
	TenantID         string             `json:"-" db:"tenant_id"`
	Name             string             `json:"name" db:"name"`
	Description      string             `json:"description" db:"description"`
	Enabled          bool               `json:"enabled" db:"enabled"`
	Tags             JSONB              `json:"tags" db:"tags"`
	NotificationConf NotificationConfig `json:"notification_config" db:"notification_config"`
	CreatedAt        time.Time          `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time          `json:"updated_at" db:"updated_at"`
	CreatedBy        string             `json:"created_by" db:"created_by"`
	// Computed fields
	Members []MonitorGroupMember `json:"members,omitempty" db:"-"`
	Status  *MonitorGroupStatus  `json:"status,omitempty" db:"-"`
}

// MonitorGroupMember represents a monitor's membership in a group
type MonitorGroupMember struct {
	ID         string    `json:"id" db:"id"`
	GroupID    string    `json:"group_id" db:"group_id"`
	MonitorID  string    `json:"monitor_id" db:"monitor_id"`
	Weight     float64   `json:"weight" db:"weight"`
	IsCritical bool      `json:"is_critical" db:"is_critical"`
	AddedAt    time.Time `json:"added_at" db:"added_at"`
	// Computed fields
	Monitor *Monitor `json:"monitor,omitempty" db:"-"`
}

// MonitorGroupStatus represents the current status of a monitor group
type MonitorGroupStatus struct {
	GroupID              string      `json:"group_id" db:"group_id"`
	OverallStatus        CheckStatus `json:"overall_status" db:"overall_status"`
	HealthScore          float64     `json:"health_score" db:"health_score"`
	MonitorsUp           int         `json:"monitors_up" db:"monitors_up"`
	MonitorsDown         int         `json:"monitors_down" db:"monitors_down"`
	MonitorsDegraded     int         `json:"monitors_degraded" db:"monitors_degraded"`
	CriticalMonitorsDown int         `json:"critical_monitors_down" db:"critical_monitors_down"`
	LastCheck            time.Time   `json:"last_check" db:"last_check"`
	Message              string      `json:"message" db:"message"`
}

// MonitorGroupSLO represents SLO configuration for a group
type MonitorGroupSLO struct {
	ID                     string    `json:"id" db:"id"`
	GroupID                string    `json:"group_id" db:"group_id"`
	TenantID               string    `json:"-" db:"tenant_id"`
	TargetUptimePercentage float64   `json:"target_uptime_percentage" db:"target_uptime_percentage"`
	MeasurementPeriodDays  int       `json:"measurement_period_days" db:"measurement_period_days"`
	CalculationMethod      string    `json:"calculation_method" db:"calculation_method"`
	CreatedAt              time.Time `json:"created_at" db:"created_at"`
	UpdatedAt              time.Time `json:"updated_at" db:"updated_at"`
}

// Calculation methods for group SLO
const (
	CalculationMethodWeightedAverage = "weighted_average"
	CalculationMethodWorstCase       = "worst_case"
	CalculationMethodCriticalOnly    = "critical_only"
)

// MonitorGroupAlertRule represents alert rules for a group
type MonitorGroupAlertRule struct {
	ID                   string                `json:"id" db:"id"`
	GroupID              string                `json:"group_id" db:"group_id"`
	Name                 string                `json:"name" db:"name"`
	Enabled              bool                  `json:"enabled" db:"enabled"`
	TriggerCondition     string                `json:"trigger_condition" db:"trigger_condition"`
	ThresholdValue       *float64              `json:"threshold_value" db:"threshold_value"`
	NotificationChannels []NotificationChannel `json:"notification_channels" db:"notification_channels"`
	CooldownMinutes      int                   `json:"cooldown_minutes" db:"cooldown_minutes"`
	CreatedAt            time.Time             `json:"created_at" db:"created_at"`
	UpdatedAt            time.Time             `json:"updated_at" db:"updated_at"`
}

// Trigger conditions for group alerts
const (
	TriggerHealthScoreBelow = "health_score_below"
	TriggerAnyCriticalDown  = "any_critical_down"
	TriggerPercentageDown   = "percentage_down"
	TriggerAllDown          = "all_down"
)

// MonitorGroupIncident represents an incident at the group level
type MonitorGroupIncident struct {
	ID                 string     `json:"id" db:"id"`
	GroupID            string     `json:"group_id" db:"group_id"`
	TenantID           string     `json:"-" db:"tenant_id"`
	StartedAt          time.Time  `json:"started_at" db:"started_at"`
	ResolvedAt         *time.Time `json:"resolved_at" db:"resolved_at"`
	Severity           string     `json:"severity" db:"severity"`
	AffectedMonitors   []string   `json:"affected_monitors" db:"affected_monitors"`
	RootCauseMonitorID *string    `json:"root_cause_monitor_id" db:"root_cause_monitor_id"`
	NotificationsSent  int        `json:"notifications_sent" db:"notifications_sent"`
	HealthScoreAtStart *float64   `json:"health_score_at_start" db:"health_score_at_start"`
	AcknowledgedAt     *time.Time `json:"acknowledged_at" db:"acknowledged_at"`
	AcknowledgedBy     *string    `json:"acknowledged_by" db:"acknowledged_by"`
}

// MonitorGroupSLAReport represents SLA report for a group
type MonitorGroupSLAReport struct {
	ID                 string    `json:"id" db:"id"`
	GroupID            string    `json:"group_id" db:"group_id"`
	TenantID           string    `json:"-" db:"tenant_id"`
	PeriodStart        time.Time `json:"period_start" db:"period_start"`
	PeriodEnd          time.Time `json:"period_end" db:"period_end"`
	HealthScoreAverage float64   `json:"health_score_average" db:"health_score_average"`
	UptimePercentage   float64   `json:"uptime_percentage" db:"uptime_percentage"`
	DowntimeMinutes    int       `json:"downtime_minutes" db:"downtime_minutes"`
	IncidentsCount     int       `json:"incidents_count" db:"incidents_count"`
	SLOMet             bool      `json:"slo_met" db:"slo_met"`
	CreatedAt          time.Time `json:"created_at" db:"created_at"`
}

// Value implementations for custom types
func (mga MonitorGroupAlertRule) Value() (driver.Value, error) {
	return json.Marshal(mga)
}

func (mga *MonitorGroupAlertRule) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	return json.Unmarshal(value.([]byte), mga)
}

// AffectedMonitors custom type for JSON array
type AffectedMonitors []string

func (am AffectedMonitors) Value() (driver.Value, error) {
	return json.Marshal(am)
}

func (am *AffectedMonitors) Scan(value interface{}) error {
	if value == nil {
		*am = []string{}
		return nil
	}
	return json.Unmarshal(value.([]byte), am)
}

// Helper methods

// CalculateHealthScore calculates the health score based on member statuses and weights
func (g *MonitorGroup) CalculateHealthScore(memberStatuses map[string]CheckStatus) float64 {
	if len(g.Members) == 0 {
		return 0.0
	}

	totalWeight := 0.0
	weightedScore := 0.0

	for _, member := range g.Members {
		totalWeight += member.Weight

		if status, ok := memberStatuses[member.MonitorID]; ok {
			switch status {
			case StatusUp:
				weightedScore += member.Weight * 100.0
			case StatusDegraded:
				weightedScore += member.Weight * 50.0
			case StatusDown:
				// 0 points for down
			}
		}
	}

	if totalWeight == 0 {
		return 0.0
	}

	return weightedScore / totalWeight
}

// DetermineOverallStatus determines the overall status based on member statuses
func (g *MonitorGroup) DetermineOverallStatus(memberStatuses map[string]CheckStatus) CheckStatus {
	hasCriticalDown := false
	hasAnyDown := false
	hasDegraded := false

	for _, member := range g.Members {
		if status, ok := memberStatuses[member.MonitorID]; ok {
			if status == StatusDown {
				hasAnyDown = true
				if member.IsCritical {
					hasCriticalDown = true
				}
			} else if status == StatusDegraded {
				hasDegraded = true
			}
		}
	}

	if hasCriticalDown {
		return StatusDown
	}
	if hasAnyDown || hasDegraded {
		return StatusDegraded
	}
	return StatusUp
}

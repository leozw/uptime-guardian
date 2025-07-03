type MonitorSLO struct {
    ID                      string    `json:"id" db:"id"`
    MonitorID               string    `json:"monitor_id" db:"monitor_id"`
    TenantID                string    `json:"-" db:"tenant_id"`
    TargetUptimePercentage  float64   `json:"target_uptime_percentage" db:"target_uptime_percentage"`
    MeasurementPeriodDays   int       `json:"measurement_period_days" db:"measurement_period_days"`
    CreatedAt               time.Time `json:"created_at" db:"created_at"`
    UpdatedAt               time.Time `json:"updated_at" db:"updated_at"`
}

type SLAReport struct {
    ID                   string    `json:"id" db:"id"`
    MonitorID            string    `json:"monitor_id" db:"monitor_id"`
    TenantID             string    `json:"-" db:"tenant_id"`
    PeriodStart          time.Time `json:"period_start" db:"period_start"`
    PeriodEnd            time.Time `json:"period_end" db:"period_end"`
    TotalChecks          int       `json:"total_checks" db:"total_checks"`
    SuccessfulChecks     int       `json:"successful_checks" db:"successful_checks"`
    FailedChecks         int       `json:"failed_checks" db:"failed_checks"`
    UptimePercentage     float64   `json:"uptime_percentage" db:"uptime_percentage"`
    DowntimeMinutes      int       `json:"downtime_minutes" db:"downtime_minutes"`
    AverageResponseTimeMs *int      `json:"average_response_time_ms" db:"average_response_time_ms"`
    SLOMet               bool      `json:"slo_met" db:"slo_met"`
    CreatedAt            time.Time `json:"created_at" db:"created_at"`
}

type Incident struct {
    ID                  string     `json:"id" db:"id"`
    MonitorID           string     `json:"monitor_id" db:"monitor_id"`
    TenantID            string     `json:"-" db:"tenant_id"`
    StartedAt           time.Time  `json:"started_at" db:"started_at"`
    ResolvedAt          *time.Time `json:"resolved_at" db:"resolved_at"`
    Severity            string     `json:"severity" db:"severity"`
    NotificationsSent   int        `json:"notifications_sent" db:"notifications_sent"`
    DowntimeMinutes     int        `json:"downtime_minutes" db:"downtime_minutes"`
    AffectedChecks      int        `json:"affected_checks" db:"affected_checks"`
    RootCause           *string    `json:"root_cause" db:"root_cause"`
    ImpactDescription   *string    `json:"impact_description" db:"impact_description"`
    ResolutionNotes     *string    `json:"resolution_notes" db:"resolution_notes"`
    AcknowledgedAt      *time.Time `json:"acknowledged_at" db:"acknowledged_at"`
    AcknowledgedBy      *string    `json:"acknowledged_by" db:"acknowledged_by"`
}

type IncidentEvent struct {
    ID          string                 `json:"id" db:"id"`
    IncidentID  string                 `json:"incident_id" db:"incident_id"`
    EventType   string                 `json:"event_type" db:"event_type"`
    EventTime   time.Time              `json:"event_time" db:"event_time"`
    Description string                 `json:"description" db:"description"`
    CreatedBy   *string                `json:"created_by" db:"created_by"`
    Metadata    map[string]interface{} `json:"metadata" db:"metadata"`
}

// Enums para incident events
const (
    IncidentEventDetected      = "detected"
    IncidentEventAcknowledged  = "acknowledged"
    IncidentEventInvestigating = "investigating"
    IncidentEventResolved      = "resolved"
    IncidentEventComment       = "comment"
)
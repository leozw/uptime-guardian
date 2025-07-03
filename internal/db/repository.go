package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

type Repository struct {
	db *sqlx.DB
}

func NewConnection(databaseURL string) (*sqlx.DB, error) {
	db, err := sqlx.Connect("postgres", databaseURL)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	return db, nil
}

func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

// Monitor operations
func (r *Repository) CreateMonitor(m *Monitor) error {
	query := `
        INSERT INTO monitors (
            id, tenant_id, name, type, target, enabled, 
            interval, timeout, regions, config, notification_config, 
            tags, created_at, updated_at, created_by
        ) VALUES (
            :id, :tenant_id, :name, :type, :target, :enabled,
            :interval, :timeout, :regions, :config, :notification_config,
            :tags, :created_at, :updated_at, :created_by
        )`

	_, err := r.db.NamedExec(query, m)
	return err
}

func (r *Repository) GetMonitor(id, tenantID string) (*Monitor, error) {
	var m Monitor
	query := `SELECT * FROM monitors WHERE id = $1 AND tenant_id = $2`
	err := r.db.Get(&m, query, id, tenantID)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("monitor not found")
	}
	return &m, err
}

func (r *Repository) GetMonitorsByTenant(tenantID string, limit, offset int) ([]*Monitor, error) {
	monitors := []*Monitor{}
	query := `
        SELECT * FROM monitors 
        WHERE tenant_id = $1 
        ORDER BY created_at DESC 
        LIMIT $2 OFFSET $3`

	err := r.db.Select(&monitors, query, tenantID, limit, offset)
	return monitors, err
}

func (r *Repository) UpdateMonitor(m *Monitor) error {
	query := `
        UPDATE monitors SET
            name = :name,
            type = :type,
            target = :target,
            enabled = :enabled,
            interval = :interval,
            timeout = :timeout,
            regions = :regions,
            config = :config,
            notification_config = :notification_config,
            tags = :tags,
            updated_at = :updated_at
        WHERE id = :id AND tenant_id = :tenant_id`

	_, err := r.db.NamedExec(query, m)
	return err
}

func (r *Repository) DeleteMonitor(id, tenantID string) error {
	query := `DELETE FROM monitors WHERE id = $1 AND tenant_id = $2`
	_, err := r.db.Exec(query, id, tenantID)
	return err
}

func (r *Repository) CountMonitorsByTenant(tenantID string) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM monitors WHERE tenant_id = $1`
	err := r.db.Get(&count, query, tenantID)
	return count, err
}

// Check scheduling
func (r *Repository) GetMonitorsToCheck() ([]*Monitor, error) {
	monitors := []*Monitor{}
	query := `
        SELECT m.* FROM monitors m
        LEFT JOIN monitor_last_status s ON m.id = s.monitor_id
        WHERE m.enabled = true 
        AND (
            s.last_check IS NULL 
            OR s.last_check + (m.interval || ' seconds')::interval < NOW()
        )`

	err := r.db.Select(&monitors, query)
	return monitors, err
}

// Check results
func (r *Repository) SaveCheckResult(result *CheckResult) error {
	tx, err := r.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Insert check result
	query := `
        INSERT INTO check_results (
            id, monitor_id, tenant_id, status, response_time_ms,
            status_code, error, details, region, checked_at
        ) VALUES (
            :id, :monitor_id, :tenant_id, :status, :response_time_ms,
            :status_code, :error, :details, :region, :checked_at
        )`

	_, err = tx.NamedExec(query, result)
	if err != nil {
		return err
	}

	// Update last status
	statusQuery := `
        INSERT INTO monitor_last_status (
            monitor_id, status, message, last_check, response_time_ms
        ) VALUES (
            $1, $2, $3, $4, $5
        ) ON CONFLICT (monitor_id) DO UPDATE SET
            status = $2,
            message = $3,
            last_check = $4,
            response_time_ms = $5`

	message := ""
	if result.Error != "" {
		message = result.Error
	}

	_, err = tx.Exec(statusQuery,
		result.MonitorID,
		result.Status,
		message,
		result.CheckedAt,
		result.ResponseTimeMs,
	)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (r *Repository) GetMonitorStatus(monitorID, tenantID string) (*MonitorStatus, error) {
	var status MonitorStatus
	query := `
        SELECT s.* FROM monitor_last_status s
        JOIN monitors m ON s.monitor_id = m.id
        WHERE s.monitor_id = $1 AND m.tenant_id = $2`

	err := r.db.Get(&status, query, monitorID, tenantID)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("status not found")
	}
	return &status, err
}

func (r *Repository) GetCheckHistory(monitorID, tenantID string, limit int) ([]*CheckResult, error) {
	results := []*CheckResult{}
	query := `
        SELECT r.* FROM check_results r
        JOIN monitors m ON r.monitor_id = m.id
        WHERE r.monitor_id = $1 AND m.tenant_id = $2
        ORDER BY r.checked_at DESC
        LIMIT $3`

	err := r.db.Select(&results, query, monitorID, tenantID, limit)
	return results, err
}

// Ping checks database connection
func (r *Repository) Ping() error {
	return r.db.Ping()
}

// Incident operations
func (r *Repository) CreateIncident(incident *Incident) error {
	query := `
        INSERT INTO incidents (
            id, monitor_id, tenant_id, started_at, severity,
            downtime_minutes, affected_checks, notifications_sent
        ) VALUES (
            :id, :monitor_id, :tenant_id, :started_at, :severity,
            :downtime_minutes, :affected_checks, :notifications_sent
        )`

	_, err := r.db.NamedExec(query, incident)
	return err
}

func (r *Repository) GetActiveIncident(monitorID string) (*Incident, error) {
	var incident Incident
	query := `
        SELECT * FROM incidents 
        WHERE monitor_id = $1 AND resolved_at IS NULL
        ORDER BY started_at DESC
        LIMIT 1`

	err := r.db.Get(&incident, query, monitorID)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no active incident")
	}
	return &incident, err
}

func (r *Repository) GetIncident(id, tenantID string) (*Incident, error) {
	var incident Incident
	query := `SELECT * FROM incidents WHERE id = $1 AND tenant_id = $2`
	err := r.db.Get(&incident, query, id, tenantID)
	return &incident, err
}

func (r *Repository) UpdateIncident(incident *Incident) error {
	query := `
        UPDATE incidents SET
            resolved_at = :resolved_at,
            downtime_minutes = :downtime_minutes,
            affected_checks = :affected_checks,
            notifications_sent = :notifications_sent,
            root_cause = :root_cause,
            impact_description = :impact_description,
            resolution_notes = :resolution_notes,
            acknowledged_at = :acknowledged_at,
            acknowledged_by = :acknowledged_by
        WHERE id = :id`

	_, err := r.db.NamedExec(query, incident)
	return err
}

func (r *Repository) GetIncidentsByMonitor(monitorID, tenantID string, limit int) ([]*Incident, error) {
	incidents := []*Incident{}
	query := `
        SELECT i.* FROM incidents i
        JOIN monitors m ON i.monitor_id = m.id
        WHERE i.monitor_id = $1 AND m.tenant_id = $2
        ORDER BY i.started_at DESC
        LIMIT $3`

	err := r.db.Select(&incidents, query, monitorID, tenantID, limit)
	return incidents, err
}

// Incident Event operations
func (r *Repository) CreateIncidentEvent(event *IncidentEvent) error {
	query := `
        INSERT INTO incident_events (
            id, incident_id, event_type, event_time,
            description, created_by, metadata
        ) VALUES (
            :id, :incident_id, :event_type, :event_time,
            :description, :created_by, :metadata
        )`

	_, err := r.db.NamedExec(query, event)
	return err
}

func (r *Repository) GetIncidentEvents(incidentID string) ([]*IncidentEvent, error) {
	events := []*IncidentEvent{}
	query := `
        SELECT * FROM incident_events 
        WHERE incident_id = $1 
        ORDER BY event_time ASC`

	err := r.db.Select(&events, query, incidentID)
	return events, err
}

// SLO operations
func (r *Repository) CreateOrUpdateSLO(slo *MonitorSLO) error {
	query := `
        INSERT INTO monitor_slos (
            id, monitor_id, tenant_id, target_uptime_percentage,
            measurement_period_days, created_at, updated_at
        ) VALUES (
            :id, :monitor_id, :tenant_id, :target_uptime_percentage,
            :measurement_period_days, :created_at, :updated_at
        ) ON CONFLICT (monitor_id) DO UPDATE SET
            target_uptime_percentage = :target_uptime_percentage,
            measurement_period_days = :measurement_period_days,
            updated_at = :updated_at`

	_, err := r.db.NamedExec(query, slo)
	return err
}

func (r *Repository) GetMonitorSLO(monitorID string) (*MonitorSLO, error) {
	var slo MonitorSLO
	query := `SELECT * FROM monitor_slos WHERE monitor_id = $1`
	err := r.db.Get(&slo, query, monitorID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &slo, err
}

// SLA Report operations
func (r *Repository) CreateSLAReport(report *SLAReport) error {
	query := `
        INSERT INTO sla_reports (
            id, monitor_id, tenant_id, period_start, period_end,
            total_checks, successful_checks, failed_checks,
            uptime_percentage, downtime_minutes, average_response_time_ms,
            slo_met, created_at
        ) VALUES (
            :id, :monitor_id, :tenant_id, :period_start, :period_end,
            :total_checks, :successful_checks, :failed_checks,
            :uptime_percentage, :downtime_minutes, :average_response_time_ms,
            :slo_met, :created_at
        ) ON CONFLICT (monitor_id, period_start, period_end) DO UPDATE SET
            total_checks = :total_checks,
            successful_checks = :successful_checks,
            failed_checks = :failed_checks,
            uptime_percentage = :uptime_percentage,
            downtime_minutes = :downtime_minutes,
            average_response_time_ms = :average_response_time_ms,
            slo_met = :slo_met`

	_, err := r.db.NamedExec(query, report)
	return err
}

func (r *Repository) GetSLAReports(monitorID string, limit int) ([]*SLAReport, error) {
	reports := []*SLAReport{}
	query := `
        SELECT * FROM sla_reports 
        WHERE monitor_id = $1 
        ORDER BY period_start DESC 
        LIMIT $2`

	err := r.db.Select(&reports, query, monitorID, limit)
	return reports, err
}

func (r *Repository) GetCheckResultsInPeriod(monitorID string, start, end time.Time) ([]*CheckResult, error) {
	results := []*CheckResult{}
	query := `
        SELECT * FROM check_results 
        WHERE monitor_id = $1 
        AND checked_at >= $2 
        AND checked_at <= $3 
        ORDER BY checked_at ASC`

	err := r.db.Select(&results, query, monitorID, start, end)
	return results, err
}

func (r *Repository) GetMonitorByID(id string) (*Monitor, error) {
	var m Monitor
	query := `SELECT * FROM monitors WHERE id = $1`
	err := r.db.Get(&m, query, id)
	return &m, err
}

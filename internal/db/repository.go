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
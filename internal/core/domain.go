package core

import (
    "time"
    "github.com/google/uuid"
)

type Domain struct {
    ID            uuid.UUID         `json:"id" db:"id"`
    TenantID      uuid.UUID         `json:"tenant_id" db:"tenant_id"`
    Name          string            `json:"name" db:"name"`
    
    // Auto-discovered
    IPs           []string          `json:"ips" db:"ips"`
    HasSSL        bool              `json:"has_ssl" db:"has_ssl"`
    MXRecords     []string          `json:"mx_records" db:"mx_records"`
    Nameservers   []string          `json:"nameservers" db:"nameservers"`
    
    // Monitoring config
    CheckInterval time.Duration     `json:"check_interval" db:"check_interval"`
    Enabled       bool              `json:"enabled" db:"enabled"`
    
    // Health
    HealthScore   int               `json:"health_score" db:"health_score"`
    LastCheckAt   *time.Time        `json:"last_check_at" db:"last_check_at"`
    
    // Metadata
    Labels        map[string]string `json:"labels" db:"labels"`
    CreatedAt     time.Time         `json:"created_at" db:"created_at"`
    UpdatedAt     time.Time         `json:"updated_at" db:"updated_at"`
}

type DomainHealth struct {
    OverallScore int                    `json:"overall_score"`
    Breakdown    map[string]int         `json:"breakdown"`
    Issues       []HealthIssue          `json:"issues"`
    LastCheck    time.Time              `json:"last_check"`
}

type HealthIssue struct {
    Severity    string `json:"severity"` // critical, warning, info
    Category    string `json:"category"`
    Description string `json:"description"`
    Impact      string `json:"impact"`
}
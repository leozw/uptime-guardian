package core

import (
    "time"
    "github.com/google/uuid"
)

type Tenant struct {
    ID               uuid.UUID `json:"id" db:"id"`
    Name             string    `json:"name" db:"name"`
    Email            string    `json:"email" db:"email"`
    APIKey           string    `json:"-" db:"api_key"`
    MimirTenantID    string    `json:"mimir_tenant_id" db:"mimir_tenant_id"`
    
    // Limits
    MaxDomains       int       `json:"max_domains" db:"max_domains"`
    CheckIntervalMin int       `json:"check_interval_min" db:"check_interval_min"`
    
    // Metadata
    IsActive         bool      `json:"is_active" db:"is_active"`
    CreatedAt        time.Time `json:"created_at" db:"created_at"`
    UpdatedAt        time.Time `json:"updated_at" db:"updated_at"`
}

type TenantStats struct {
    DomainCount      int `json:"domain_count"`
    TotalChecks      int `json:"total_checks"`
    FailedChecks     int `json:"failed_checks"`
    AverageHealth    int `json:"average_health"`
}
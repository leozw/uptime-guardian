package db

import (
    "database/sql/driver"
    "encoding/json"
    "time"
)

type MonitorType string

const (
    MonitorTypeHTTP   MonitorType = "http"
    MonitorTypeSSL    MonitorType = "ssl"
    MonitorTypeDNS    MonitorType = "dns"
    MonitorTypeDomain MonitorType = "domain"
)

type CheckStatus string

const (
    StatusUp      CheckStatus = "up"
    StatusDown    CheckStatus = "down"
    StatusDegraded CheckStatus = "degraded"
)

type Monitor struct {
    ID               string             `json:"id" db:"id"`
    TenantID         string             `json:"-" db:"tenant_id"`
    Name             string             `json:"name" db:"name"`
    Type             MonitorType        `json:"type" db:"type"`
    Target           string             `json:"target" db:"target"`
    Enabled          bool               `json:"enabled" db:"enabled"`
    Interval         int                `json:"interval" db:"interval"`
    Timeout          int                `json:"timeout" db:"timeout"`
    Regions          StringSlice        `json:"regions" db:"regions"`
    Config           MonitorConfig      `json:"config" db:"config"`
    NotificationConf NotificationConfig `json:"notification_config" db:"notification_config"`
    Tags             JSONB              `json:"tags" db:"tags"`
    CreatedAt        time.Time          `json:"created_at" db:"created_at"`
    UpdatedAt        time.Time          `json:"updated_at" db:"updated_at"`
    CreatedBy        string             `json:"created_by" db:"created_by"`
}

type MonitorConfig struct {
    // HTTP Check
    Method              string            `json:"method,omitempty"`
    Headers             map[string]string `json:"headers,omitempty"`
    Body                string            `json:"body,omitempty"`
    ExpectedStatusCodes []int             `json:"expected_status_codes,omitempty"`
    SearchString        string            `json:"search_string,omitempty"`
    BasicAuth           *BasicAuth        `json:"basic_auth,omitempty"`
    FollowRedirects     bool              `json:"follow_redirects,omitempty"`
    
    // SSL Check
    CheckExpiry         bool              `json:"check_expiry,omitempty"`
    MinDaysBeforeExpiry int               `json:"min_days_before_expiry,omitempty"`
    
    // DNS Check
    RecordType          string            `json:"record_type,omitempty"`
    ExpectedValues      []string          `json:"expected_values,omitempty"`
    
    // Domain Check
    DomainMinDaysBeforeExpiry int        `json:"domain_min_days_before_expiry,omitempty"`
}

type BasicAuth struct {
    Username string `json:"username"`
    Password string `json:"password"`
}

type NotificationConfig struct {
    Channels         []NotificationChannel `json:"channels"`
    OnFailureCount   int                   `json:"on_failure_count"`
    OnRecovery       bool                  `json:"on_recovery"`
    ReminderInterval int                   `json:"reminder_interval"`
}

type NotificationChannel struct {
    Type    string                 `json:"type"`
    Config  map[string]interface{} `json:"config"`
    Enabled bool                   `json:"enabled"`
}

type CheckResult struct {
    ID             string                 `json:"id" db:"id"`
    MonitorID      string                 `json:"monitor_id" db:"monitor_id"`
    TenantID       string                 `json:"-" db:"tenant_id"`
    Status         CheckStatus            `json:"status" db:"status"`
    ResponseTimeMs int                    `json:"response_time_ms" db:"response_time_ms"`
    StatusCode     int                    `json:"status_code,omitempty" db:"status_code"`
    Error          string                 `json:"error,omitempty" db:"error"`
    Details        JSONB                  `json:"details,omitempty" db:"details"`
    Region         string                 `json:"region" db:"region"`
    CheckedAt      time.Time              `json:"checked_at" db:"checked_at"`
}

type MonitorStatus struct {
    MonitorID      string     `json:"monitor_id" db:"monitor_id"`
    Status         CheckStatus `json:"status" db:"status"`
    Message        string      `json:"message" db:"message"`
    LastCheck      time.Time   `json:"last_check" db:"last_check"`
    ResponseTimeMs int         `json:"response_time_ms" db:"response_time_ms"`
    SSLExpiryDays  *int        `json:"ssl_expiry_days,omitempty" db:"ssl_expiry_days"`
}

// Custom types for PostgreSQL arrays and JSONB
type StringSlice []string

func (s StringSlice) Value() (driver.Value, error) {
    return json.Marshal(s)
}

func (s *StringSlice) Scan(value interface{}) error {
    if value == nil {
        *s = []string{}
        return nil
    }
    return json.Unmarshal(value.([]byte), s)
}

type JSONB map[string]interface{}

func (j JSONB) Value() (driver.Value, error) {
    return json.Marshal(j)
}

func (j *JSONB) Scan(value interface{}) error {
    if value == nil {
        *j = make(map[string]interface{})
        return nil
    }
    return json.Unmarshal(value.([]byte), j)
}

// Value implementations for custom types
func (mc MonitorConfig) Value() (driver.Value, error) {
    return json.Marshal(mc)
}

func (mc *MonitorConfig) Scan(value interface{}) error {
    if value == nil {
        return nil
    }
    return json.Unmarshal(value.([]byte), mc)
}

func (nc NotificationConfig) Value() (driver.Value, error) {
    return json.Marshal(nc)
}

func (nc *NotificationConfig) Scan(value interface{}) error {
    if value == nil {
        return nil
    }
    return json.Unmarshal(value.([]byte), nc)
}
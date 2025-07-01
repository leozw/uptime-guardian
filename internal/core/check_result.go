package core

import (
    "encoding/json"
    "time"
    "github.com/google/uuid"
)

type CheckResult struct {
    ID           uuid.UUID       `json:"id" db:"id"`
    DomainID     uuid.UUID       `json:"domain_id" db:"domain_id"`
    CheckType    string          `json:"check_type" db:"check_type"`
    Success      bool            `json:"success" db:"success"`
    
    // Metrics
    ResponseTime float64         `json:"response_time_ms" db:"response_time_ms"`
    Details      json.RawMessage `json:"details" db:"details"`
    ErrorMessage *string         `json:"error_message,omitempty" db:"error_message"`
    
    CheckedAt    time.Time       `json:"checked_at" db:"checked_at"`
}

// Check type specific results
type SSLCheckDetails struct {
    Grade            string    `json:"grade"`
    DaysToExpiry     int       `json:"days_to_expiry"`
    Issuer           string    `json:"issuer"`
    Subject          string    `json:"subject"`
    ValidFrom        time.Time `json:"valid_from"`
    ValidTo          time.Time `json:"valid_to"`
    Protocol         string    `json:"protocol"`
    CipherSuite      string    `json:"cipher_suite"`
    CertificateChain []string  `json:"certificate_chain"`
}

type DNSCheckDetails struct {
    ARecords         []string          `json:"a_records"`
    AAAARecords      []string          `json:"aaaa_records"`
    MXRecords        []MXRecord        `json:"mx_records"`
    TXTRecords       []string          `json:"txt_records"`
    NSRecords        []string          `json:"ns_records"`
    CNAMERecord      *string           `json:"cname_record,omitempty"`
    SOARecord        *SOARecord        `json:"soa_record,omitempty"`
    HasDNSSEC        bool              `json:"has_dnssec"`
    ResponseTime     float64           `json:"response_time_ms"`
}

type MXRecord struct {
    Priority int    `json:"priority"`
    Host     string `json:"host"`
}

type SOARecord struct {
    PrimaryNS string `json:"primary_ns"`
    Email     string `json:"email"`
    Serial    uint32 `json:"serial"`
    Refresh   uint32 `json:"refresh"`
    Retry     uint32 `json:"retry"`
    Expire    uint32 `json:"expire"`
    Minimum   uint32 `json:"minimum"`
}

type HTTPCheckDetails struct {
    StatusCode       int               `json:"status_code"`
    ResponseHeaders  map[string]string `json:"response_headers"`
    ResponseTime     float64           `json:"response_time_ms"`
    BodySize         int64             `json:"body_size_bytes"`
    RedirectChain    []string          `json:"redirect_chain,omitempty"`
    SecurityHeaders  SecurityHeaders   `json:"security_headers"`
}

type SecurityHeaders struct {
    StrictTransportSecurity bool `json:"strict_transport_security"`
    XContentTypeOptions     bool `json:"x_content_type_options"`
    XFrameOptions           bool `json:"x_frame_options"`
    ContentSecurityPolicy   bool `json:"content_security_policy"`
    XSSProtection          bool `json:"xss_protection"`
}

type WHOISCheckDetails struct {
    DomainExpiry     *time.Time `json:"domain_expiry,omitempty"`
    DaysToExpiry     int        `json:"days_to_expiry"`
    Registrar        string     `json:"registrar"`
    CreatedDate      *time.Time `json:"created_date,omitempty"`
    UpdatedDate      *time.Time `json:"updated_date,omitempty"`
    Status           []string   `json:"status"`
}
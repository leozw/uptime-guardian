package checks

import (
    "fmt"
    "strings"
    "time"

    "github.com/likexian/whois"
    "github.com/leozw/uptime-guardian/internal/db"
)

type DomainChecker struct{}

func NewDomainChecker() *DomainChecker {
    return &DomainChecker{}
}

func (d *DomainChecker) Check(monitor *db.Monitor, region string) *db.CheckResult {
    result := &db.CheckResult{
        MonitorID: monitor.ID,
        TenantID:  monitor.TenantID,
        Region:    region,
        Details:   make(db.JSONB),
    }
    
    // Clean domain name
    domain := monitor.Target
    domain = strings.TrimPrefix(domain, "http://")
    domain = strings.TrimPrefix(domain, "https://")
    domain = strings.Split(domain, "/")[0]
    
    // Perform WHOIS lookup
    start := time.Now()
    whoisResult, err := whois.Whois(domain)
    duration := time.Since(start)
    
    result.ResponseTimeMs = int(duration.Milliseconds())
    
    if err != nil {
        result.Status = db.StatusDown
        result.Error = fmt.Sprintf("WHOIS lookup failed: %v", err)
        return result
    }
    
    // Parse WHOIS data
    expiryDate := d.extractExpiryDate(whoisResult)
    if expiryDate.IsZero() {
        result.Status = db.StatusDegraded
        result.Error = "Could not extract expiry date from WHOIS data"
        result.Details["whois_data"] = whoisResult
        return result
    }
    
    result.Details["expiry_date"] = expiryDate.Format(time.RFC3339)
    
    // Check if domain is expired
    now := time.Now()
    if now.After(expiryDate) {
        result.Status = db.StatusDown
        result.Error = "Domain has expired"
        return result
    }
    
    // Calculate days until expiry
    daysUntilExpiry := int(expiryDate.Sub(now).Hours() / 24)
    result.Details["days_until_expiry"] = daysUntilExpiry
    
    // Check expiry warning
    if monitor.Config.DomainMinDaysBeforeExpiry > 0 {
        if daysUntilExpiry < monitor.Config.DomainMinDaysBeforeExpiry {
            result.Status = db.StatusDegraded
            result.Error = fmt.Sprintf("Domain expires in %d days", daysUntilExpiry)
            return result
        }
    }
    
    result.Status = db.StatusUp
    return result
}

func (d *DomainChecker) extractExpiryDate(whoisData string) time.Time {
    // Common patterns for expiry date in WHOIS data
    patterns := []string{
        "Registry Expiry Date:",
        "Registrar Registration Expiration Date:",
        "Expiry Date:",
        "Expiration Date:",
        "Expires:",
        "Expiry:",
        "paid-till:",
    }
    
    lines := strings.Split(whoisData, "\n")
    for _, line := range lines {
        line = strings.TrimSpace(line)
        for _, pattern := range patterns {
            if strings.HasPrefix(strings.ToLower(line), strings.ToLower(pattern)) {
                dateStr := strings.TrimSpace(strings.TrimPrefix(line, pattern))
                
                // Try various date formats
                formats := []string{
                    time.RFC3339,
                    "2006-01-02T15:04:05Z",
                    "2006-01-02 15:04:05",
                    "2006-01-02",
                    "02-Jan-2006",
                    "2006.01.02",
                }
                
                for _, format := range formats {
                    if t, err := time.Parse(format, dateStr); err == nil {
                        return t
                    }
                }
            }
        }
    }
    
    return time.Time{}
}
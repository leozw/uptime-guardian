package checks

import (
    "crypto/tls"
    "fmt"
    "net"
    "net/url"
    "time"

    "github.com/leozw/uptime-guardian/internal/db"
)

type SSLChecker struct{}

func NewSSLChecker() *SSLChecker {
    return &SSLChecker{}
}

func (s *SSLChecker) Check(monitor *db.Monitor, region string) *db.CheckResult {
    result := &db.CheckResult{
        MonitorID: monitor.ID,
        TenantID:  monitor.TenantID,
        Region:    region,
        Details:   make(db.JSONB),
    }
    
    // Parse URL to get hostname
    u, err := url.Parse(monitor.Target)
    if err != nil {
        result.Status = db.StatusDown
        result.Error = fmt.Sprintf("Invalid URL: %v", err)
        return result
    }
    
    hostname := u.Hostname()
    port := u.Port()
    if port == "" {
        port = "443"
    }
    
    // Connect with timeout
    dialer := &net.Dialer{
        Timeout: time.Duration(monitor.Timeout) * time.Second,
    }
    
    start := time.Now()
    conn, err := tls.DialWithDialer(dialer, "tcp", fmt.Sprintf("%s:%s", hostname, port), &tls.Config{
        ServerName: hostname,
    })
    duration := time.Since(start)
    
    result.ResponseTimeMs = int(duration.Milliseconds())
    
    if err != nil {
        result.Status = db.StatusDown
        result.Error = fmt.Sprintf("SSL connection failed: %v", err)
        return result
    }
    defer conn.Close()
    
    // Get certificate details
    certs := conn.ConnectionState().PeerCertificates
    if len(certs) == 0 {
        result.Status = db.StatusDown
        result.Error = "No certificates found"
        return result
    }
    
    cert := certs[0]
    
    // Check certificate validity
    now := time.Now()
    if now.Before(cert.NotBefore) {
        result.Status = db.StatusDown
        result.Error = "Certificate not yet valid"
        return result
    }
    
    daysUntilExpiry := int(cert.NotAfter.Sub(now).Hours() / 24)
    result.Details["days_until_expiry"] = daysUntilExpiry
    result.Details["issuer"] = cert.Issuer.String()
    result.Details["subject"] = cert.Subject.String()
    result.Details["not_after"] = cert.NotAfter.Format(time.RFC3339)
    
    if now.After(cert.NotAfter) {
        result.Status = db.StatusDown
        result.Error = "Certificate has expired"
        return result
    }
    
    // Check expiry warning
    if monitor.Config.CheckExpiry && monitor.Config.MinDaysBeforeExpiry > 0 {
        if daysUntilExpiry < monitor.Config.MinDaysBeforeExpiry {
            result.Status = db.StatusDegraded
            result.Error = fmt.Sprintf("Certificate expires in %d days", daysUntilExpiry)
            return result
        }
    }
    
    result.Status = db.StatusUp
    return result
}
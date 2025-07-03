package checks

import (
    "crypto/tls"
    "fmt"
    "io"
    "net/http"
    "strings"
    "time"

    "github.com/leozw/uptime-guardian/internal/db"
)

type HTTPChecker struct {
    client *http.Client
}

func NewHTTPChecker() *HTTPChecker {
    return &HTTPChecker{
        client: &http.Client{
            Timeout: 30 * time.Second,
            Transport: &http.Transport{
                TLSClientConfig: &tls.Config{
                    InsecureSkipVerify: false,
                },
            },
            CheckRedirect: func(req *http.Request, via []*http.Request) error {
                if len(via) >= 10 {
                    return fmt.Errorf("stopped after 10 redirects")
                }
                return nil
            },
        },
    }
}

func (h *HTTPChecker) Check(monitor *db.Monitor, region string) *db.CheckResult {
    result := &db.CheckResult{
        MonitorID: monitor.ID,
        TenantID:  monitor.TenantID,
        Region:    region,
    }
    
    // Create request
    method := monitor.Config.Method
    if method == "" {
        method = "GET"
    }
    
    var body io.Reader
    if monitor.Config.Body != "" {
        body = strings.NewReader(monitor.Config.Body)
    }
    
    req, err := http.NewRequest(method, monitor.Target, body)
    if err != nil {
        result.Status = db.StatusDown
        result.Error = fmt.Sprintf("Failed to create request: %v", err)
        return result
    }
    
    // Set headers
    for k, v := range monitor.Config.Headers {
        req.Header.Set(k, v)
    }
    
    // Basic auth
    if monitor.Config.BasicAuth != nil {
        req.SetBasicAuth(monitor.Config.BasicAuth.Username, monitor.Config.BasicAuth.Password)
    }
    
    // Set custom timeout
    client := h.client
    if monitor.Timeout > 0 {
        client = &http.Client{
            Timeout: time.Duration(monitor.Timeout) * time.Second,
            Transport: h.client.Transport,
            CheckRedirect: h.client.CheckRedirect,
        }
    }
    
    // Execute request
    start := time.Now()
    resp, err := client.Do(req)
    duration := time.Since(start)
    
    if err != nil {
        result.Status = db.StatusDown
        result.Error = fmt.Sprintf("Request failed: %v", err)
        result.ResponseTimeMs = int(duration.Milliseconds())
        return result
    }
    defer resp.Body.Close()
    
    result.StatusCode = resp.StatusCode
    result.ResponseTimeMs = int(duration.Milliseconds())
    
    // Check expected status codes
    expectedCodes := monitor.Config.ExpectedStatusCodes
    if len(expectedCodes) == 0 {
        expectedCodes = []int{200}
    }
    
    statusOK := false
    for _, code := range expectedCodes {
        if resp.StatusCode == code {
            statusOK = true
            break
        }
    }
    
    if !statusOK {
        result.Status = db.StatusDown
        result.Error = fmt.Sprintf("Unexpected status code: %d", resp.StatusCode)
        return result
    }
    
    // Check body content if required
    if monitor.Config.SearchString != "" {
        bodyBytes, err := io.ReadAll(resp.Body)
        if err != nil {
            result.Status = db.StatusDegraded
            result.Error = fmt.Sprintf("Failed to read response body: %v", err)
            return result
        }
        
        if !strings.Contains(string(bodyBytes), monitor.Config.SearchString) {
            result.Status = db.StatusDown
            result.Error = fmt.Sprintf("Search string not found in response")
            return result
        }
    }
    
    result.Status = db.StatusUp
    return result
}
package checker

import (
    "context"
    "fmt"
    "io"
    "net/http"
    "net/http/httptrace"
    "strings"
    "time"

    "github.com/leozw/uptime-guardian/internal/core"
)

type HTTPChecker struct {
    client *http.Client
}

func NewHTTPChecker() *HTTPChecker {
    return &HTTPChecker{
        client: &http.Client{
            Timeout: 30 * time.Second,
            CheckRedirect: func(req *http.Request, via []*http.Request) error {
                if len(via) >= 10 {
                    return fmt.Errorf("too many redirects")
                }
                return nil
            },
        },
    }
}

func (h *HTTPChecker) Check(domain string) (*core.HTTPCheckDetails, error) {
    url := "https://" + domain
    
    // Try HTTPS first, fall back to HTTP
    details, err := h.checkURL(url)
    if err != nil {
        url = "http://" + domain
        details, err = h.checkURL(url)
    }
    
    return details, err
}

func (h *HTTPChecker) checkURL(url string) (*core.HTTPCheckDetails, error) {
    details := &core.HTTPCheckDetails{
        ResponseHeaders: make(map[string]string),
        SecurityHeaders: core.SecurityHeaders{},
    }

    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return nil, err
    }

    // Add trace to measure timing
    var start time.Time
    trace := &httptrace.ClientTrace{
        GotFirstResponseByte: func() {
            details.ResponseTime = float64(time.Since(start).Milliseconds())
        },
    }
    
    ctx := httptrace.WithClientTrace(context.Background(), trace)
    req = req.WithContext(ctx)
    
    // User agent
    req.Header.Set("User-Agent", "DomainMonitor/1.0")
    
    start = time.Now()
    resp, err := h.client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("request failed: %w", err)
    }
    defer resp.Body.Close()

    // If no timing was captured (no body), use total time
    if details.ResponseTime == 0 {
        details.ResponseTime = float64(time.Since(start).Milliseconds())
    }

    details.StatusCode = resp.StatusCode

    // Copy headers
    for k, v := range resp.Header {
        if len(v) > 0 {
            details.ResponseHeaders[k] = v[0]
        }
    }

    // Check security headers
    h.checkSecurityHeaders(resp.Header, &details.SecurityHeaders)

    // Read body to get size
    body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024)) // Limit to 1MB
    if err == nil {
        details.BodySize = int64(len(body))
    }

    // Check status
    if resp.StatusCode >= 400 {
        return details, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
    }

    return details, nil
}

func (h *HTTPChecker) checkSecurityHeaders(headers http.Header, security *core.SecurityHeaders) {
    // HSTS
    if hsts := headers.Get("Strict-Transport-Security"); hsts != "" {
        security.StrictTransportSecurity = true
    }

    // X-Content-Type-Options
    if xcto := headers.Get("X-Content-Type-Options"); strings.EqualFold(xcto, "nosniff") {
        security.XContentTypeOptions = true
    }

    // X-Frame-Options
    if xfo := headers.Get("X-Frame-Options"); xfo != "" {
        security.XFrameOptions = true
    }

    // Content-Security-Policy
    if csp := headers.Get("Content-Security-Policy"); csp != "" {
        security.ContentSecurityPolicy = true
    }

    // X-XSS-Protection (deprecated but still checked)
    if xss := headers.Get("X-XSS-Protection"); xss != "" {
        security.XSSProtection = true
    }
}
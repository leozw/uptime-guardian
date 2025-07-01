package checker

import (
    "encoding/json"
    "sync"
    "time"

    "github.com/leozw/uptime-guardian/internal/core"
)

type Analyzer struct {
    dns   *DNSChecker
    ssl   *SSLChecker
    http  *HTTPChecker
    whois *WHOISChecker
}

func NewAnalyzer() *Analyzer {
    return &Analyzer{
        dns:   NewDNSChecker(),
        ssl:   NewSSLChecker(),
        http:  NewHTTPChecker(),
        whois: NewWHOISChecker(),
    }
}

type DomainAnalysis struct {
    Domain      string                         `json:"domain"`
    Timestamp   time.Time                      `json:"timestamp"`
    HealthScore int                            `json:"health_score"`
    Checks      map[string]*core.CheckResult   `json:"checks"`
}

func (a *Analyzer) AnalyzeDomain(domain, tenantID string) (*DomainAnalysis, error) {
    analysis := &DomainAnalysis{
        Domain:    domain,
        Timestamp: time.Now(),
        Checks:    make(map[string]*core.CheckResult),
    }

    var wg sync.WaitGroup
    var mu sync.Mutex
    checkTypes := []string{"dns", "ssl", "http", "whois"}

    for _, checkType := range checkTypes {
        wg.Add(1)
        go func(ct string) {
            defer wg.Done()

            var result *core.CheckResult
            startTime := time.Now()

            switch ct {
            case "dns":
                details, err := a.dns.Check(domain)
                result = createCheckResult(ct, details, err, startTime)
            case "ssl":
                details, err := a.ssl.Check(domain)
                result = createCheckResult(ct, details, err, startTime)
            case "http":
                details, err := a.http.Check(domain)
                result = createCheckResult(ct, details, err, startTime)
            case "whois":
                details, err := a.whois.Check(domain)
                result = createCheckResult(ct, details, err, startTime)
            }

            mu.Lock()
            analysis.Checks[ct] = result
            mu.Unlock()
        }(checkType)
    }

    wg.Wait()

    // Calculate health score
    analysis.HealthScore = a.calculateHealthScore(analysis.Checks)

    return analysis, nil
}

func createCheckResult(checkType string, details interface{}, err error, startTime time.Time) *core.CheckResult {
    result := &core.CheckResult{
        CheckType:    checkType,
        Success:      err == nil,
        ResponseTime: float64(time.Since(startTime).Milliseconds()),
        CheckedAt:    time.Now(),
    }

    if err != nil {
        errMsg := err.Error()
        result.ErrorMessage = &errMsg
    } else if details != nil {
        detailsJSON, _ := json.Marshal(details)
        result.Details = detailsJSON
    }

    return result
}

func (a *Analyzer) calculateHealthScore(checks map[string]*core.CheckResult) int {
    score := 100
    weights := map[string]int{
        "dns":   30,
        "ssl":   30,
        "http":  25,
        "whois": 15,
    }

    for checkType, weight := range weights {
        if check, ok := checks[checkType]; ok && !check.Success {
            score -= weight
        }
    }

    if score < 0 {
        score = 0
    }

    return score
}
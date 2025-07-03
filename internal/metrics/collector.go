package metrics

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
    "github.com/leozw/uptime-guardian/internal/config"
    "github.com/leozw/uptime-guardian/internal/db"
)

type Collector struct {
    config *config.MimirConfig
    
    // Metrics
    checkDuration      *prometheus.HistogramVec
    checkUp            *prometheus.GaugeVec
    checksTotal        *prometheus.CounterVec
    sslDaysUntilExpiry *prometheus.GaugeVec
    dnsLookupDuration  *prometheus.HistogramVec
}

func NewCollector(cfg config.MimirConfig) *Collector {
    return &Collector{
        config: &cfg,
        
        checkDuration: promauto.NewHistogramVec(
            prometheus.HistogramOpts{
                Name: "uptime_check_duration_seconds",
                Help: "Duration of uptime checks in seconds",
                Buckets: prometheus.DefBuckets,
            },
            []string{"tenant_id", "monitor_id", "type", "region"},
        ),
        
        checkUp: promauto.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "uptime_check_up",
                Help: "Whether the check is up (1) or down (0)",
            },
            []string{"tenant_id", "monitor_id", "type", "region"},
        ),
        
        checksTotal: promauto.NewCounterVec(
            prometheus.CounterOpts{
                Name: "uptime_checks_total",
                Help: "Total number of checks performed",
            },
            []string{"tenant_id", "monitor_id", "type", "region", "status"},
        ),
        
        sslDaysUntilExpiry: promauto.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "ssl_cert_days_until_expiry",
                Help: "Days until SSL certificate expires",
            },
            []string{"tenant_id", "domain"},
        ),
        
        dnsLookupDuration: promauto.NewHistogramVec(
            prometheus.HistogramOpts{
                Name: "dns_lookup_duration_seconds",
                Help: "Duration of DNS lookups in seconds",
                Buckets: prometheus.DefBuckets,
            },
            []string{"tenant_id", "domain", "record_type"},
        ),
    }
}

func (c *Collector) RecordCheck(result *db.CheckResult, monitor *db.Monitor) {
    labels := prometheus.Labels{
        "tenant_id":  result.TenantID,
        "monitor_id": result.MonitorID,
        "type":       string(monitor.Type),
        "region":     result.Region,
    }
    
    // Record duration
    c.checkDuration.With(labels).Observe(float64(result.ResponseTimeMs) / 1000)
    
    // Record up/down status
    upValue := 0.0
    if result.Status == db.StatusUp {
        upValue = 1.0
    }
    c.checkUp.With(labels).Set(upValue)
    
    // Increment total counter
    labelsWithStatus := prometheus.Labels{
        "tenant_id":  result.TenantID,
        "monitor_id": result.MonitorID,
        "type":       string(monitor.Type),
        "region":     result.Region,
        "status":     string(result.Status),
    }
    c.checksTotal.With(labelsWithStatus).Inc()
    
    // SSL specific metrics
    if monitor.Type == db.MonitorTypeSSL {
        if days, ok := result.Details["days_until_expiry"].(float64); ok {
            c.sslDaysUntilExpiry.With(prometheus.Labels{
                "tenant_id": result.TenantID,
                "domain":    monitor.Target,
            }).Set(days)
        }
    }
}
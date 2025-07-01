package metrics

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    domainHealthScore = promauto.NewGaugeVec(prometheus.GaugeOpts{
        Name: "domain_health_score",
        Help: "Overall health score of a domain (0-100)",
    }, []string{"domain", "tenant_id"})

    sslDaysRemaining = promauto.NewGaugeVec(prometheus.GaugeOpts{
        Name: "domain_ssl_days_remaining",
        Help: "Days until SSL certificate expires",
    }, []string{"domain", "tenant_id"})

    domainExpirationDays = promauto.NewGaugeVec(prometheus.GaugeOpts{
        Name: "domain_expiration_days",
        Help: "Days until domain expires",
    }, []string{"domain", "tenant_id"})

    dnsRecordCount = promauto.NewGaugeVec(prometheus.GaugeOpts{
        Name: "domain_dns_record_count",
        Help: "Number of DNS records by type",
    }, []string{"domain", "tenant_id", "record_type"})

    httpResponseTime = promauto.NewHistogramVec(prometheus.HistogramOpts{
        Name:    "domain_http_response_time_seconds",
        Help:    "HTTP response time in seconds",
        Buckets: prometheus.DefBuckets,
    }, []string{"domain", "tenant_id"})

    checkDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
        Name:    "domain_check_duration_seconds",
        Help:    "Time taken to perform domain check",
        Buckets: prometheus.DefBuckets,
    }, []string{"domain", "tenant_id", "check_type"})

    checkSuccess = promauto.NewGaugeVec(prometheus.GaugeOpts{
        Name: "domain_check_success",
        Help: "Whether the check succeeded (1) or failed (0)",
    }, []string{"domain", "tenant_id", "check_type"})
)
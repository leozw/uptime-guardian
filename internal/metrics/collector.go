package metrics

import (
	"github.com/leozw/uptime-guardian/internal/config"
	"github.com/leozw/uptime-guardian/internal/db"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Collector struct {
	config *config.MimirConfig

	// Métricas genéricas
	checkDuration     *prometheus.HistogramVec
	checkUp           *prometheus.GaugeVec
	checksTotal       *prometheus.CounterVec
	checkResponseCode *prometheus.GaugeVec

	// Métricas SSL
	sslDaysUntilExpiry *prometheus.GaugeVec
	sslCertValid       *prometheus.GaugeVec

	// Métricas DNS
	dnsLookupDuration    *prometheus.HistogramVec
	dnsRecordCount       *prometheus.GaugeVec
	dnsResolutionSuccess *prometheus.GaugeVec

	// Métricas de Domain
	domainDaysUntilExpiry *prometheus.GaugeVec
	domainValid           *prometheus.GaugeVec
}

func NewCollector(cfg config.MimirConfig) *Collector {
	return &Collector{
		config: &cfg,

		checkDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "uptime_check_duration_seconds",
				Help:    "Duration of uptime checks in seconds",
				Buckets: []float64{.025, .05, .1, .25, .5, 1, 2.5, 5, 10},
			},
			[]string{"tenant_id", "monitor_id", "monitor_name", "type", "target", "region"},
		),

		checkUp: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "uptime_check_up",
				Help: "Whether the check is up (1) or down (0)",
			},
			[]string{"tenant_id", "monitor_id", "monitor_name", "type", "target", "region"},
		),

		checksTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "uptime_checks_total",
				Help: "Total number of checks performed",
			},
			[]string{"tenant_id", "monitor_id", "monitor_name", "type", "target", "region", "status"},
		),

		checkResponseCode: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "uptime_http_response_code",
				Help: "HTTP response code of the last check",
			},
			[]string{"tenant_id", "monitor_id", "monitor_name", "target", "region"},
		),

		// SSL específicas
		sslDaysUntilExpiry: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "ssl_cert_days_until_expiry",
				Help: "Days until SSL certificate expires",
			},
			[]string{"tenant_id", "monitor_id", "monitor_name", "domain", "issuer"},
		),

		sslCertValid: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "ssl_cert_valid",
				Help: "Whether the SSL certificate is valid (1) or not (0)",
			},
			[]string{"tenant_id", "monitor_id", "monitor_name", "domain"},
		),

		// DNS específicas
		dnsLookupDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "dns_lookup_duration_seconds",
				Help:    "Duration of DNS lookups in seconds",
				Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
			},
			[]string{"tenant_id", "monitor_id", "monitor_name", "domain", "record_type", "region"},
		),

		dnsRecordCount: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "dns_record_count",
				Help: "Number of DNS records found",
			},
			[]string{"tenant_id", "monitor_id", "monitor_name", "domain", "record_type"},
		),

		dnsResolutionSuccess: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "dns_resolution_success",
				Help: "Whether DNS resolution was successful (1) or not (0)",
			},
			[]string{"tenant_id", "monitor_id", "monitor_name", "domain", "record_type"},
		),

		// Domain específicas
		domainDaysUntilExpiry: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "domain_days_until_expiry",
				Help: "Days until domain expires",
			},
			[]string{"tenant_id", "monitor_id", "monitor_name", "domain"},
		),

		domainValid: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "domain_valid",
				Help: "Whether the domain is valid (1) or not (0)",
			},
			[]string{"tenant_id", "monitor_id", "monitor_name", "domain"},
		),
	}
}

func (c *Collector) RecordCheck(result *db.CheckResult, monitor *db.Monitor) {
	baseLabels := prometheus.Labels{
		"tenant_id":    result.TenantID,
		"monitor_id":   result.MonitorID,
		"monitor_name": monitor.Name,
		"type":         string(monitor.Type),
		"target":       monitor.Target,
		"region":       result.Region,
	}

	// Métricas genéricas
	c.checkDuration.With(baseLabels).Observe(float64(result.ResponseTimeMs) / 1000)

	upValue := 0.0
	if result.Status == db.StatusUp {
		upValue = 1.0
	}
	c.checkUp.With(baseLabels).Set(upValue)

	// Contador com status
	labelsWithStatus := prometheus.Labels{
		"tenant_id":    result.TenantID,
		"monitor_id":   result.MonitorID,
		"monitor_name": monitor.Name,
		"type":         string(monitor.Type),
		"target":       monitor.Target,
		"region":       result.Region,
		"status":       string(result.Status),
	}
	c.checksTotal.With(labelsWithStatus).Inc()

	// Métricas específicas por tipo
	switch monitor.Type {
	case db.MonitorTypeHTTP:
		if result.StatusCode > 0 {
			c.checkResponseCode.With(prometheus.Labels{
				"tenant_id":    result.TenantID,
				"monitor_id":   result.MonitorID,
				"monitor_name": monitor.Name,
				"target":       monitor.Target,
				"region":       result.Region,
			}).Set(float64(result.StatusCode))
		}

	case db.MonitorTypeSSL:
		if days, ok := result.Details["days_until_expiry"].(float64); ok {
			issuer := ""
			if iss, ok := result.Details["issuer"].(string); ok {
				issuer = iss
			}

			c.sslDaysUntilExpiry.With(prometheus.Labels{
				"tenant_id":    result.TenantID,
				"monitor_id":   result.MonitorID,
				"monitor_name": monitor.Name,
				"domain":       monitor.Target,
				"issuer":       issuer,
			}).Set(days)
		}

		validValue := 0.0
		if result.Status == db.StatusUp {
			validValue = 1.0
		}
		c.sslCertValid.With(prometheus.Labels{
			"tenant_id":    result.TenantID,
			"monitor_id":   result.MonitorID,
			"monitor_name": monitor.Name,
			"domain":       monitor.Target,
		}).Set(validValue)

	case db.MonitorTypeDNS:
		recordType := monitor.Config.RecordType
		if recordType == "" {
			recordType = "A"
		}

		// Duração do lookup
		c.dnsLookupDuration.With(prometheus.Labels{
			"tenant_id":    result.TenantID,
			"monitor_id":   result.MonitorID,
			"monitor_name": monitor.Name,
			"domain":       monitor.Target,
			"record_type":  recordType,
			"region":       result.Region,
		}).Observe(float64(result.ResponseTimeMs) / 1000)

		// Contagem de registros
		if count, ok := result.Details["record_count"].(int); ok {
			c.dnsRecordCount.With(prometheus.Labels{
				"tenant_id":    result.TenantID,
				"monitor_id":   result.MonitorID,
				"monitor_name": monitor.Name,
				"domain":       monitor.Target,
				"record_type":  recordType,
			}).Set(float64(count))
		}

		// Sucesso da resolução
		successValue := 0.0
		if result.Status == db.StatusUp {
			successValue = 1.0
		}
		c.dnsResolutionSuccess.With(prometheus.Labels{
			"tenant_id":    result.TenantID,
			"monitor_id":   result.MonitorID,
			"monitor_name": monitor.Name,
			"domain":       monitor.Target,
			"record_type":  recordType,
		}).Set(successValue)

	case db.MonitorTypeDomain:
		if days, ok := result.Details["days_until_expiry"].(float64); ok {
			c.domainDaysUntilExpiry.With(prometheus.Labels{
				"tenant_id":    result.TenantID,
				"monitor_id":   result.MonitorID,
				"monitor_name": monitor.Name,
				"domain":       monitor.Target,
			}).Set(days)
		}

		validValue := 0.0
		if result.Status == db.StatusUp {
			validValue = 1.0
		}
		c.domainValid.With(prometheus.Labels{
			"tenant_id":    result.TenantID,
			"monitor_id":   result.MonitorID,
			"monitor_name": monitor.Name,
			"domain":       monitor.Target,
		}).Set(validValue)
	}
}

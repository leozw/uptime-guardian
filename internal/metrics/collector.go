package metrics

import (
	"github.com/leozw/uptime-guardian/internal/config"
	"github.com/leozw/uptime-guardian/internal/db"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Collector struct {
	config *config.MimirConfig

	// Métricas genéricas existentes
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

	// === NOVAS MÉTRICAS ===

	// SLA/SLO Metrics
	slaUptimePercentage    *prometheus.GaugeVec
	slaTargetPercentage    *prometheus.GaugeVec
	sloBudgetRemaining     *prometheus.GaugeVec
	sloViolation           *prometheus.GaugeVec
	monthlyDowntimeMinutes *prometheus.GaugeVec

	// Incident Metrics
	incidentsTotal   *prometheus.CounterVec
	incidentDuration *prometheus.HistogramVec
	incidentsActive  *prometheus.GaugeVec
	incidentMTTR     *prometheus.HistogramVec // Mean Time To Recovery
	incidentMTTA     *prometheus.HistogramVec // Mean Time To Acknowledge

	// Monitor Management Metrics
	monitorsTotal   *prometheus.GaugeVec
	monitorsEnabled *prometheus.GaugeVec
	monitorsByType  *prometheus.GaugeVec

	// Notification Metrics
	notificationsSent   *prometheus.CounterVec
	notificationsFailed *prometheus.CounterVec
	notificationLatency *prometheus.HistogramVec

	// System Health Metrics
	lastCheckTimestamp *prometheus.GaugeVec
	checksScheduled    *prometheus.GaugeVec
	checksQueueSize    *prometheus.GaugeVec
	workerUtilization  *prometheus.GaugeVec

	// Monitor Group Metrics
	groupHealthScore      *prometheus.GaugeVec
	groupStatus           *prometheus.GaugeVec
	groupMonitorsUp       *prometheus.GaugeVec
	groupMonitorsDown     *prometheus.GaugeVec
	groupMonitorsDegraded *prometheus.GaugeVec
	groupCriticalDown     *prometheus.GaugeVec
	groupIncidentsTotal   *prometheus.CounterVec
	groupIncidentsActive  *prometheus.GaugeVec
	groupSLAPercentage    *prometheus.GaugeVec
}

func NewCollector(cfg config.MimirConfig) *Collector {
	return &Collector{
		config: &cfg,

		// Métricas existentes
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
			[]string{"tenant_id", "monitor_id", "monitor_name", "target", "issuer"},
		),

		sslCertValid: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "ssl_cert_valid",
				Help: "Whether the SSL certificate is valid (1) or not (0)",
			},
			[]string{"tenant_id", "monitor_id", "monitor_name", "target"},
		),

		// DNS específicas
		dnsLookupDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "dns_lookup_duration_seconds",
				Help:    "Duration of DNS lookups in seconds",
				Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
			},
			[]string{"tenant_id", "monitor_id", "monitor_name", "target", "record_type", "region"},
		),

		dnsRecordCount: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "dns_record_count",
				Help: "Number of DNS records found",
			},
			[]string{"tenant_id", "monitor_id", "monitor_name", "target", "record_type"},
		),

		dnsResolutionSuccess: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "dns_resolution_success",
				Help: "Whether DNS resolution was successful (1) or not (0)",
			},
			[]string{"tenant_id", "monitor_id", "monitor_name", "target", "record_type"},
		),

		// Domain específicas
		domainDaysUntilExpiry: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "domain_days_until_expiry",
				Help: "Days until domain expires",
			},
			[]string{"tenant_id", "monitor_id", "monitor_name", "target"},
		),

		domainValid: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "domain_valid",
				Help: "Whether the domain is valid (1) or not (0)",
			},
			[]string{"tenant_id", "monitor_id", "monitor_name", "target"},
		),

		// === NOVAS MÉTRICAS ===

		// SLA/SLO Metrics
		slaUptimePercentage: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "uptime_sla_percentage",
				Help: "Current SLA uptime percentage",
			},
			[]string{"tenant_id", "monitor_id", "monitor_name", "period"},
		),

		slaTargetPercentage: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "uptime_sla_target_percentage",
				Help: "SLA target percentage configured",
			},
			[]string{"tenant_id", "monitor_id", "monitor_name"},
		),

		sloBudgetRemaining: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "uptime_slo_error_budget_remaining_minutes",
				Help: "Remaining error budget in minutes",
			},
			[]string{"tenant_id", "monitor_id", "monitor_name", "period"},
		),

		sloViolation: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "uptime_slo_violation",
				Help: "Whether SLO is currently violated (1) or not (0)",
			},
			[]string{"tenant_id", "monitor_id", "monitor_name"},
		),

		monthlyDowntimeMinutes: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "uptime_monthly_downtime_minutes",
				Help: "Total downtime in minutes for the current month",
			},
			[]string{"tenant_id", "monitor_id", "monitor_name"},
		),

		// Incident Metrics
		incidentsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "uptime_incidents_total",
				Help: "Total number of incidents",
			},
			[]string{"tenant_id", "monitor_id", "monitor_name", "severity"},
		),

		incidentDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "uptime_incident_duration_minutes",
				Help:    "Duration of incidents in minutes",
				Buckets: []float64{1, 5, 10, 30, 60, 120, 360, 720, 1440},
			},
			[]string{"tenant_id", "monitor_id", "monitor_name", "severity"},
		),

		incidentsActive: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "uptime_incidents_active",
				Help: "Number of currently active incidents",
			},
			[]string{"tenant_id", "monitor_id", "monitor_name", "severity"},
		),

		incidentMTTR: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "uptime_incident_mttr_minutes",
				Help:    "Mean Time To Recovery in minutes",
				Buckets: []float64{1, 5, 10, 30, 60, 120, 360, 720, 1440},
			},
			[]string{"tenant_id", "monitor_id", "monitor_name"},
		),

		incidentMTTA: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "uptime_incident_mtta_minutes",
				Help:    "Mean Time To Acknowledge in minutes",
				Buckets: []float64{1, 5, 10, 15, 30, 60, 120, 240},
			},
			[]string{"tenant_id", "monitor_id", "monitor_name"},
		),

		// Monitor Management Metrics
		monitorsTotal: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "uptime_monitors_total",
				Help: "Total number of monitors",
			},
			[]string{"tenant_id"},
		),

		monitorsEnabled: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "uptime_monitors_enabled",
				Help: "Number of enabled monitors",
			},
			[]string{"tenant_id"},
		),

		monitorsByType: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "uptime_monitors_by_type",
				Help: "Number of monitors by type",
			},
			[]string{"tenant_id", "type"},
		),

		// Notification Metrics
		notificationsSent: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "uptime_notifications_sent_total",
				Help: "Total number of notifications sent",
			},
			[]string{"tenant_id", "monitor_id", "channel_type", "status"},
		),

		notificationsFailed: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "uptime_notifications_failed_total",
				Help: "Total number of failed notifications",
			},
			[]string{"tenant_id", "monitor_id", "channel_type", "reason"},
		),

		notificationLatency: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "uptime_notification_latency_seconds",
				Help:    "Notification delivery latency",
				Buckets: []float64{.1, .25, .5, 1, 2.5, 5, 10, 30},
			},
			[]string{"tenant_id", "channel_type"},
		),

		// System Health Metrics
		lastCheckTimestamp: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "uptime_last_check_timestamp",
				Help: "Timestamp of the last check for a monitor",
			},
			[]string{"tenant_id", "monitor_id", "monitor_name"},
		),

		checksScheduled: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "uptime_checks_scheduled",
				Help: "Number of checks currently scheduled",
			},
			[]string{"tenant_id"},
		),

		checksQueueSize: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "uptime_checks_queue_size",
				Help: "Current size of the checks queue",
			},
			[]string{"worker_pool"},
		),

		workerUtilization: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "uptime_worker_utilization_percentage",
				Help: "Worker pool utilization percentage",
			},
			[]string{"worker_pool"},
		),

		// Monitor Group Metrics
		groupHealthScore: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "uptime_group_health_score",
				Help: "Health score of monitor groups (0-100)",
			},
			[]string{"tenant_id", "group_id", "group_name"},
		),

		groupStatus: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "uptime_group_status",
				Help: "Overall status of monitor groups (1=up, 0.5=degraded, 0=down)",
			},
			[]string{"tenant_id", "group_id", "group_name"},
		),

		groupMonitorsUp: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "uptime_group_monitors_up",
				Help: "Number of monitors up in the group",
			},
			[]string{"tenant_id", "group_id", "group_name"},
		),

		groupMonitorsDown: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "uptime_group_monitors_down",
				Help: "Number of monitors down in the group",
			},
			[]string{"tenant_id", "group_id", "group_name"},
		),

		groupMonitorsDegraded: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "uptime_group_monitors_degraded",
				Help: "Number of monitors degraded in the group",
			},
			[]string{"tenant_id", "group_id", "group_name"},
		),

		groupCriticalDown: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "uptime_group_critical_monitors_down",
				Help: "Number of critical monitors down in the group",
			},
			[]string{"tenant_id", "group_id", "group_name"},
		),

		groupIncidentsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "uptime_group_incidents_total",
				Help: "Total number of group incidents",
			},
			[]string{"tenant_id", "group_id", "group_name", "severity"},
		),

		groupIncidentsActive: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "uptime_group_incidents_active",
				Help: "Number of currently active group incidents",
			},
			[]string{"tenant_id", "group_id", "group_name", "severity"},
		),

		groupSLAPercentage: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "uptime_group_sla_percentage",
				Help: "Current SLA percentage for monitor groups",
			},
			[]string{"tenant_id", "group_id", "group_name", "calculation_method"},
		),
	}
}

// Existing RecordCheck method remains the same...
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

	// Update last check timestamp
	c.lastCheckTimestamp.With(prometheus.Labels{
		"tenant_id":    result.TenantID,
		"monitor_id":   result.MonitorID,
		"monitor_name": monitor.Name,
	}).SetToCurrentTime()

	// Métricas específicas por tipo (mantém o código existente)
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
				"target":       monitor.Target,
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
			"target":       monitor.Target,
		}).Set(validValue)

	case db.MonitorTypeDNS:
		recordType := monitor.Config.RecordType
		if recordType == "" {
			recordType = "A"
		}

		c.dnsLookupDuration.With(prometheus.Labels{
			"tenant_id":    result.TenantID,
			"monitor_id":   result.MonitorID,
			"monitor_name": monitor.Name,
			"target":       monitor.Target,
			"record_type":  recordType,
			"region":       result.Region,
		}).Observe(float64(result.ResponseTimeMs) / 1000)

		if count, ok := result.Details["record_count"].(int); ok {
			c.dnsRecordCount.With(prometheus.Labels{
				"tenant_id":    result.TenantID,
				"monitor_id":   result.MonitorID,
				"monitor_name": monitor.Name,
				"target":       monitor.Target,
				"record_type":  recordType,
			}).Set(float64(count))
		}

		successValue := 0.0
		if result.Status == db.StatusUp {
			successValue = 1.0
		}
		c.dnsResolutionSuccess.With(prometheus.Labels{
			"tenant_id":    result.TenantID,
			"monitor_id":   result.MonitorID,
			"monitor_name": monitor.Name,
			"target":       monitor.Target,
			"record_type":  recordType,
		}).Set(successValue)

	case db.MonitorTypeDomain:
		if days, ok := result.Details["days_until_expiry"].(float64); ok {
			c.domainDaysUntilExpiry.With(prometheus.Labels{
				"tenant_id":    result.TenantID,
				"monitor_id":   result.MonitorID,
				"monitor_name": monitor.Name,
				"target":       monitor.Target,
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
			"target":       monitor.Target,
		}).Set(validValue)
	}
}

// === NEW METHODS FOR THE NEW METRICS ===

// RecordSLAMetrics records SLA-related metrics
func (c *Collector) RecordSLAMetrics(report *db.SLAReport, monitor *db.Monitor, slo *db.MonitorSLO) {
	labels := prometheus.Labels{
		"tenant_id":    monitor.TenantID,
		"monitor_id":   monitor.ID,
		"monitor_name": monitor.Name,
		"period":       "monthly",
	}

	c.slaUptimePercentage.With(labels).Set(report.UptimePercentage)
	c.monthlyDowntimeMinutes.With(labels).Set(float64(report.DowntimeMinutes))

	if slo != nil {
		c.slaTargetPercentage.With(prometheus.Labels{
			"tenant_id":    monitor.TenantID,
			"monitor_id":   monitor.ID,
			"monitor_name": monitor.Name,
		}).Set(slo.TargetUptimePercentage)

		// Calculate error budget
		totalMinutesInPeriod := slo.MeasurementPeriodDays * 24 * 60
		allowedDowntime := float64(totalMinutesInPeriod) * (1 - slo.TargetUptimePercentage/100)
		budgetRemaining := allowedDowntime - float64(report.DowntimeMinutes)

		c.sloBudgetRemaining.With(labels).Set(budgetRemaining)

		violationValue := 0.0
		if !report.SLOMet {
			violationValue = 1.0
		}
		c.sloViolation.With(prometheus.Labels{
			"tenant_id":    monitor.TenantID,
			"monitor_id":   monitor.ID,
			"monitor_name": monitor.Name,
		}).Set(violationValue)
	}
}

// RecordIncidentCreated records a new incident
func (c *Collector) RecordIncidentCreated(incident *db.Incident, monitor *db.Monitor) {
	c.incidentsTotal.With(prometheus.Labels{
		"tenant_id":    incident.TenantID,
		"monitor_id":   incident.MonitorID,
		"monitor_name": monitor.Name,
		"severity":     incident.Severity,
	}).Inc()

	c.incidentsActive.With(prometheus.Labels{
		"tenant_id":    incident.TenantID,
		"monitor_id":   incident.MonitorID,
		"monitor_name": monitor.Name,
		"severity":     incident.Severity,
	}).Inc()
}

// RecordIncidentResolved records incident resolution
func (c *Collector) RecordIncidentResolved(incident *db.Incident, monitor *db.Monitor) {
	c.incidentsActive.With(prometheus.Labels{
		"tenant_id":    incident.TenantID,
		"monitor_id":   incident.MonitorID,
		"monitor_name": monitor.Name,
		"severity":     incident.Severity,
	}).Dec()

	c.incidentDuration.With(prometheus.Labels{
		"tenant_id":    incident.TenantID,
		"monitor_id":   incident.MonitorID,
		"monitor_name": monitor.Name,
		"severity":     incident.Severity,
	}).Observe(float64(incident.DowntimeMinutes))

	c.incidentMTTR.With(prometheus.Labels{
		"tenant_id":    incident.TenantID,
		"monitor_id":   incident.MonitorID,
		"monitor_name": monitor.Name,
	}).Observe(float64(incident.DowntimeMinutes))
}

// UpdateActiveIncidentCount updates the count of active incidents for a tenant
func (c *Collector) UpdateActiveIncidentCount(tenantID string, count int) {
	// This would typically be called periodically to update the gauge
	// You'd need to aggregate by monitor_id and severity
}

// RecordScheduledChecks records the number of scheduled checks
func (c *Collector) RecordScheduledChecks(tenantID string, count int) {
	c.checksScheduled.With(prometheus.Labels{
		"tenant_id": tenantID,
	}).Set(float64(count))
}

// RecordIncidentAcknowledged records incident acknowledgment
func (c *Collector) RecordIncidentAcknowledged(incident *db.Incident, monitor *db.Monitor) {
	if incident.AcknowledgedAt != nil {
		mtta := incident.AcknowledgedAt.Sub(incident.StartedAt).Minutes()
		c.incidentMTTA.With(prometheus.Labels{
			"tenant_id":    incident.TenantID,
			"monitor_id":   incident.MonitorID,
			"monitor_name": monitor.Name,
		}).Observe(mtta)
	}
}

// RecordNotification records notification metrics
func (c *Collector) RecordNotificationSent(tenantID, monitorID, channelType string, success bool, latencySeconds float64) {
	status := "success"
	if !success {
		status = "failed"
	}

	c.notificationsSent.With(prometheus.Labels{
		"tenant_id":    tenantID,
		"monitor_id":   monitorID,
		"channel_type": channelType,
		"status":       status,
	}).Inc()

	if !success {
		c.notificationsFailed.With(prometheus.Labels{
			"tenant_id":    tenantID,
			"monitor_id":   monitorID,
			"channel_type": channelType,
			"reason":       "delivery_failed",
		}).Inc()
	}

	c.notificationLatency.With(prometheus.Labels{
		"tenant_id":    tenantID,
		"channel_type": channelType,
	}).Observe(latencySeconds)
}

// RecordMonitorStats records monitor statistics
func (c *Collector) RecordMonitorStats(tenantID string, totalMonitors, enabledMonitors int, monitorsByType map[string]int) {
	c.monitorsTotal.With(prometheus.Labels{
		"tenant_id": tenantID,
	}).Set(float64(totalMonitors))

	c.monitorsEnabled.With(prometheus.Labels{
		"tenant_id": tenantID,
	}).Set(float64(enabledMonitors))

	for monitorType, count := range monitorsByType {
		c.monitorsByType.With(prometheus.Labels{
			"tenant_id": tenantID,
			"type":      monitorType,
		}).Set(float64(count))
	}
}

// RecordWorkerMetrics records worker pool metrics
func (c *Collector) RecordWorkerMetrics(poolName string, queueSize int, utilization float64) {
	c.checksQueueSize.With(prometheus.Labels{
		"worker_pool": poolName,
	}).Set(float64(queueSize))

	c.workerUtilization.With(prometheus.Labels{
		"worker_pool": poolName,
	}).Set(utilization)
}

// === MONITOR GROUP METRICS ===

// RecordGroupHealthScore records the health score for a monitor group
func (c *Collector) RecordGroupHealthScore(tenantID, groupID, groupName string, healthScore float64) {
	c.groupHealthScore.With(prometheus.Labels{
		"tenant_id":  tenantID,
		"group_id":   groupID,
		"group_name": groupName,
	}).Set(healthScore)
}

// RecordGroupStatus records the overall status for a monitor group
func (c *Collector) RecordGroupStatus(tenantID, groupID, groupName string, statusValue float64) {
	c.groupStatus.With(prometheus.Labels{
		"tenant_id":  tenantID,
		"group_id":   groupID,
		"group_name": groupName,
	}).Set(statusValue)
}

// RecordGroupMonitorCounts records monitor counts for a group
func (c *Collector) RecordGroupMonitorCounts(tenantID, groupID, groupName string, up, down, degraded, criticalDown int) {
	c.groupMonitorsUp.With(prometheus.Labels{
		"tenant_id":  tenantID,
		"group_id":   groupID,
		"group_name": groupName,
	}).Set(float64(up))

	c.groupMonitorsDown.With(prometheus.Labels{
		"tenant_id":  tenantID,
		"group_id":   groupID,
		"group_name": groupName,
	}).Set(float64(down))

	c.groupMonitorsDegraded.With(prometheus.Labels{
		"tenant_id":  tenantID,
		"group_id":   groupID,
		"group_name": groupName,
	}).Set(float64(degraded))

	c.groupCriticalDown.With(prometheus.Labels{
		"tenant_id":  tenantID,
		"group_id":   groupID,
		"group_name": groupName,
	}).Set(float64(criticalDown))
}

// RecordGroupIncidentCreated records a new group incident
func (c *Collector) RecordGroupIncidentCreated(incident *db.MonitorGroupIncident, group *db.MonitorGroup) {
	c.groupIncidentsTotal.With(prometheus.Labels{
		"tenant_id":  incident.TenantID,
		"group_id":   incident.GroupID,
		"group_name": group.Name,
		"severity":   incident.Severity,
	}).Inc()

	c.groupIncidentsActive.With(prometheus.Labels{
		"tenant_id":  incident.TenantID,
		"group_id":   incident.GroupID,
		"group_name": group.Name,
		"severity":   incident.Severity,
	}).Inc()
}

// RecordGroupIncidentResolved records group incident resolution
func (c *Collector) RecordGroupIncidentResolved(incident *db.MonitorGroupIncident, group *db.MonitorGroup) {
	c.groupIncidentsActive.With(prometheus.Labels{
		"tenant_id":  incident.TenantID,
		"group_id":   incident.GroupID,
		"group_name": group.Name,
		"severity":   incident.Severity,
	}).Dec()
}

// RecordGroupSLA records SLA metrics for a group
func (c *Collector) RecordGroupSLA(report *db.MonitorGroupSLAReport, group *db.MonitorGroup, method string) {
	c.groupSLAPercentage.With(prometheus.Labels{
		"tenant_id":          group.TenantID,
		"group_id":           group.ID,
		"group_name":         group.Name,
		"calculation_method": method,
	}).Set(report.UptimePercentage)
}

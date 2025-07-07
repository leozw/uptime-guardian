package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/leozw/uptime-guardian/internal/db"
	"github.com/leozw/uptime-guardian/internal/sla"
	"go.uber.org/zap"
)

// GetMetricsSummary returns a comprehensive metrics summary for dashboards
func (h *Handler) GetMetricsSummary(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	// Time range parameters
	rangeParam := c.DefaultQuery("range", "24h")
	endTime := time.Now()
	var startTime time.Time

	switch rangeParam {
	case "1h":
		startTime = endTime.Add(-1 * time.Hour)
	case "24h":
		startTime = endTime.Add(-24 * time.Hour)
	case "7d":
		startTime = endTime.Add(-7 * 24 * time.Hour)
	case "30d":
		startTime = endTime.Add(-30 * 24 * time.Hour)
	default:
		startTime = endTime.Add(-24 * time.Hour)
	}

	// Get all monitors for tenant
	monitors, err := h.repo.GetMonitorsByTenant(tenantID, 1000, 0)
	if err != nil {
		h.logger.Error("Failed to get monitors", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get monitors"})
		return
	}

	// Calculate statistics
	totalMonitors := len(monitors)
	enabledMonitors := 0
	monitorsByType := make(map[string]int)
	monitorsByStatus := map[string]int{
		"up":       0,
		"down":     0,
		"degraded": 0,
		"unknown":  0,
	}

	// SLA metrics
	slaViolations := 0
	averageUptime := 0.0
	totalUptimePercentage := 0.0
	monitorsWithSLA := 0

	// Incident metrics
	activeIncidents := 0
	totalIncidents := 0
	averageMTTR := 0.0
	totalMTTR := 0.0
	incidentsWithMTTR := 0

	for _, monitor := range monitors {
		// Count enabled monitors
		if monitor.Enabled {
			enabledMonitors++
		}

		// Count by type
		monitorsByType[string(monitor.Type)]++

		// Get current status
		status, err := h.repo.GetMonitorStatus(monitor.ID, tenantID)
		if err != nil {
			monitorsByStatus["unknown"]++
		} else {
			monitorsByStatus[string(status.Status)]++
		}

		// Get SLA for current month
		slaCalc := sla.NewCalculator(h.repo, h.logger)
		currentSLA, err := slaCalc.GetCurrentMonthSLA(monitor.ID)
		if err == nil && currentSLA != nil {
			monitorsWithSLA++
			totalUptimePercentage += currentSLA.UptimePercentage

			// Check for SLA violations
			slo, _ := h.repo.GetMonitorSLO(monitor.ID)
			if slo != nil && currentSLA.UptimePercentage < slo.TargetUptimePercentage {
				slaViolations++
			}
		}

		// Get incidents for this monitor
		incidents, err := h.repo.GetIncidentsByMonitor(monitor.ID, tenantID, 100)
		if err == nil {
			for _, incident := range incidents {
				// Count active incidents
				if incident.ResolvedAt == nil {
					activeIncidents++
				}

				// Count incidents in time range
				if incident.StartedAt.After(startTime) {
					totalIncidents++

					// Calculate MTTR
					if incident.ResolvedAt != nil {
						mttr := incident.ResolvedAt.Sub(incident.StartedAt).Minutes()
						totalMTTR += mttr
						incidentsWithMTTR++
					}
				}
			}
		}
	}

	// Calculate averages
	if monitorsWithSLA > 0 {
		averageUptime = totalUptimePercentage / float64(monitorsWithSLA)
	}
	if incidentsWithMTTR > 0 {
		averageMTTR = totalMTTR / float64(incidentsWithMTTR)
	}

	// Update metrics in Prometheus
	h.metrics.RecordMonitorStats(tenantID, totalMonitors, enabledMonitors, monitorsByType)

	// Response
	response := gin.H{
		"overview": gin.H{
			"total_monitors":     totalMonitors,
			"enabled_monitors":   enabledMonitors,
			"monitors_by_type":   monitorsByType,
			"monitors_by_status": monitorsByStatus,
		},
		"sla": gin.H{
			"average_uptime_percentage": averageUptime,
			"sla_violations":            slaViolations,
			"monitors_with_sla":         monitorsWithSLA,
		},
		"incidents": gin.H{
			"active_incidents":     activeIncidents,
			"total_incidents":      totalIncidents,
			"average_mttr_minutes": averageMTTR,
		},
		"time_range": gin.H{
			"start": startTime.Format(time.RFC3339),
			"end":   endTime.Format(time.RFC3339),
		},
	}

	c.JSON(http.StatusOK, response)
}

// GetMonitorMetrics returns detailed metrics for a specific monitor
func (h *Handler) GetMonitorMetrics(c *gin.Context) {
	monitorID := c.Param("id")
	tenantID := c.GetString("tenant_id")

	// Verify monitor belongs to tenant
	monitor, err := h.repo.GetMonitor(monitorID, tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Monitor not found"})
		return
	}

	// Get current status
	status, _ := h.repo.GetMonitorStatus(monitorID, tenantID)

	// Get SLA calculator
	slaCalc := sla.NewCalculator(h.repo, h.logger)

	// Get current month SLA
	currentSLA, err := slaCalc.GetCurrentMonthSLA(monitorID)
	if err != nil {
		h.logger.Error("Failed to calculate current SLA", zap.Error(err))
	}

	// Get SLA history (last 12 months)
	slaHistory, err := slaCalc.GetSLAHistory(monitorID, 12)
	if err != nil {
		h.logger.Error("Failed to get SLA history", zap.Error(err))
		slaHistory = []*db.SLAReport{}
	}

	// Get SLO configuration
	slo, _ := h.repo.GetMonitorSLO(monitorID)

	// Get recent incidents
	incidents, err := h.repo.GetIncidentsByMonitor(monitorID, tenantID, 10)
	if err != nil {
		h.logger.Error("Failed to get incidents", zap.Error(err))
		incidents = []*db.Incident{}
	}

	// Get check history for response time trends
	checkHistory, err := h.repo.GetCheckHistory(monitorID, tenantID, 100)
	if err != nil {
		h.logger.Error("Failed to get check history", zap.Error(err))
		checkHistory = []*db.CheckResult{}
	}

	// Calculate response time statistics
	var totalResponseTime int64
	var successfulChecks int
	responseTimes := make([]int, 0)

	for _, check := range checkHistory {
		if check.Status == db.StatusUp {
			successfulChecks++
			totalResponseTime += int64(check.ResponseTimeMs)
			responseTimes = append(responseTimes, check.ResponseTimeMs)
		}
	}

	avgResponseTime := 0
	if successfulChecks > 0 {
		avgResponseTime = int(totalResponseTime / int64(successfulChecks))
	}

	// Calculate percentiles (simplified - in production use a proper algorithm)
	p95ResponseTime := 0
	p99ResponseTime := 0
	if len(responseTimes) > 0 {
		// Sort response times
		// This is simplified - use a proper percentile calculation
		if len(responseTimes) >= 20 {
			p95Index := int(float64(len(responseTimes)) * 0.95)
			p99Index := int(float64(len(responseTimes)) * 0.99)
			p95ResponseTime = responseTimes[p95Index]
			p99ResponseTime = responseTimes[p99Index]
		}
	}

	// Update SLA metrics in Prometheus
	if currentSLA != nil && monitor != nil {
		h.metrics.RecordSLAMetrics(currentSLA, monitor, slo)
	}

	response := gin.H{
		"monitor":        monitor,
		"current_status": status,
		"sla": gin.H{
			"current":           currentSLA,
			"history":           slaHistory,
			"slo_configuration": slo,
		},
		"performance": gin.H{
			"average_response_time_ms": avgResponseTime,
			"p95_response_time_ms":     p95ResponseTime,
			"p99_response_time_ms":     p99ResponseTime,
			"total_checks":             len(checkHistory),
			"successful_checks":        successfulChecks,
		},
		"incidents": gin.H{
			"recent":       incidents,
			"total_count":  len(incidents),
			"active_count": countActiveIncidents(incidents),
		},
	}

	c.JSON(http.StatusOK, response)
}

// GetTenantMetrics returns aggregated metrics for all monitors of a tenant
func (h *Handler) GetTenantMetrics(c *gin.Context) {
	// This endpoint could aggregate data across all monitors
	// and provide tenant-level insights

	// For now, redirect to metrics summary
	h.GetMetricsSummary(c)
}

// Helper function to count active incidents
func countActiveIncidents(incidents []*db.Incident) int {
	count := 0
	for _, incident := range incidents {
		if incident.ResolvedAt == nil {
			count++
		}
	}
	return count
}

// RecordCheckQueueMetrics is called by the scheduler to update queue metrics
func (h *Handler) RecordCheckQueueMetrics(queueSize int, workerUtilization float64) {
	h.metrics.RecordWorkerMetrics("default", queueSize, workerUtilization)
}

// GetMonitorsPerformance returns performance metrics for all monitors
func (h *Handler) GetMonitorsPerformance(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	
	// Time range parameters
	rangeParam := c.DefaultQuery("range", "24h")
	endTime := time.Now()
	var startTime time.Time
	
	switch rangeParam {
	case "1h":
		startTime = endTime.Add(-1 * time.Hour)
	case "24h":
		startTime = endTime.Add(-24 * time.Hour)
	case "7d":
		startTime = endTime.Add(-7 * 24 * time.Hour)
	default:
		startTime = endTime.Add(-24 * time.Hour)
	}
	
	// Get all monitors for tenant
	monitors, err := h.repo.GetMonitorsByTenant(tenantID, 1000, 0)
	if err != nil {
		h.logger.Error("Failed to get monitors", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get monitors"})
		return
	}
	
	var performanceData []gin.H
	var totalResponseTime int64
	var totalChecks int
	
	for _, monitor := range monitors {
		if !monitor.Enabled {
			continue
		}
		
		// Get recent check history
		checkHistory, err := h.repo.GetCheckHistoryInPeriod(monitor.ID, tenantID, startTime, endTime)
		if err != nil {
			h.logger.Error("Failed to get check history", zap.Error(err))
			continue
		}
		
		if len(checkHistory) == 0 {
			continue
		}
		
		// Calculate stats for this monitor
		var monitorResponseTime int64
		successfulChecks := 0
		
		for _, check := range checkHistory {
			if check.Status == db.StatusUp {
				successfulChecks++
				monitorResponseTime += int64(check.ResponseTimeMs)
				totalResponseTime += int64(check.ResponseTimeMs)
			}
		}
		
		totalChecks += successfulChecks
		
		avgResponseTime := 0
		if successfulChecks > 0 {
			avgResponseTime = int(monitorResponseTime / int64(successfulChecks))
		}
		
		performanceData = append(performanceData, gin.H{
			"monitor_id":               monitor.ID,
			"monitor_name":             monitor.Name,
			"monitor_type":             monitor.Type,
			"target":                   monitor.Target,
			"average_response_time_ms": avgResponseTime,
			"total_checks":             len(checkHistory),
			"successful_checks":        successfulChecks,
			"uptime_percentage":        float64(successfulChecks) / float64(len(checkHistory)) * 100,
		})
	}
	
	// Overall average
	overallAverage := 0
	if totalChecks > 0 {
		overallAverage = int(totalResponseTime / int64(totalChecks))
	}
	
	c.JSON(http.StatusOK, gin.H{
		"summary": gin.H{
			"overall_average_response_time_ms": overallAverage,
			"total_monitors":                   len(performanceData),
			"total_successful_checks":          totalChecks,
			"time_range": gin.H{
				"start": startTime.Format(time.RFC3339),
				"end":   endTime.Format(time.RFC3339),
			},
		},
		"monitors": performanceData,
	})
}
package api

import (
	"github.com/gin-gonic/gin"
	"github.com/leozw/uptime-guardian/internal/api/handlers"
	"github.com/leozw/uptime-guardian/internal/api/middleware"
	"github.com/leozw/uptime-guardian/pkg/keycloak"
)

func SetupRoutes(r *gin.Engine, h *handlers.Handler, kc *keycloak.Client) {
	// Health check
	r.GET("/health", h.Health)
	r.GET("/ready", h.Ready)

	// API v1
	v1 := r.Group("/api/v1")
	v1.Use(middleware.Auth(kc), middleware.Tenant())

	// Monitors CRUD
	monitors := v1.Group("/monitors")
	{
		monitors.GET("", h.ListMonitors)
		monitors.POST("", h.CreateMonitor)
		monitors.GET("/:id", h.GetMonitor)
		monitors.PUT("/:id", h.UpdateMonitor)
		monitors.DELETE("/:id", h.DeleteMonitor)
		monitors.POST("/:id/enable", h.EnableMonitor)
		monitors.POST("/:id/disable", h.DisableMonitor)
		monitors.POST("/:id/test", h.TestMonitor)

		// Status and history
		monitors.GET("/:id/status", h.GetMonitorStatus)
		monitors.GET("/:id/history", h.GetMonitorHistory)
		monitors.GET("/:id/incidents", h.GetMonitorIncidents)
		monitors.GET("/:id/grafana", h.GetGrafanaLink)

		// SLA/SLO endpoints
		monitors.GET("/:id/sla", h.GetMonitorSLA)
		monitors.POST("/:id/slo", h.SetMonitorSLO)
	}

	// Bulk operations
	v1.POST("/monitors/bulk", h.BulkCreateMonitors)
	v1.PUT("/monitors/bulk", h.BulkUpdateMonitors)

	// Notification channels
	notifications := v1.Group("/notifications")
	{
		notifications.GET("/channels", h.ListNotificationChannels)
		notifications.POST("/channels", h.CreateNotificationChannel)
		notifications.PUT("/channels/:id", h.UpdateNotificationChannel)
		notifications.DELETE("/channels/:id", h.DeleteNotificationChannel)
		notifications.POST("/test", h.TestNotification)
	}

	// Monitor Groups
	groups := v1.Group("/monitor-groups")
	{
		groups.GET("", h.ListMonitorGroups)
		groups.POST("", h.CreateMonitorGroup)
		groups.GET("/:id", h.GetMonitorGroup)
		groups.PUT("/:id", h.UpdateMonitorGroup)
		groups.DELETE("/:id", h.DeleteMonitorGroup)

		// Group members
		groups.POST("/:id/monitors", h.AddMonitorToGroup)
		groups.DELETE("/:id/monitors/:monitor_id", h.RemoveMonitorFromGroup)

		// Group status and monitoring
		groups.GET("/:id/status", h.GetMonitorGroupStatus)
		groups.GET("/:id/incidents", h.GetMonitorGroupIncidents)
		groups.POST("/:id/slo", h.SetMonitorGroupSLO)

		// Alert rules
		groups.POST("/:id/alert-rules", h.CreateGroupAlertRule)
	}

	// Dashboard/Overview
	v1.GET("/overview", h.GetOverview)
	v1.GET("/status-page", h.GetStatusPage)

	// Settings
	v1.GET("/settings", h.GetSettings)
	v1.PUT("/settings", h.UpdateSettings)

	// Metrics endpoints
	v1.GET("/metrics/summary", h.GetMetricsSummary)
	v1.GET("/metrics/tenant", h.GetTenantMetrics)
	monitors.GET("/:id/metrics", h.GetMonitorMetrics)

	// Incident management
	incidents := v1.Group("/incidents")
	{
		incidents.GET("/:incident_id", h.GetIncidentDetails)
		incidents.POST("/:incident_id/acknowledge", h.AcknowledgeIncident)
		incidents.POST("/:incident_id/comment", h.AddIncidentComment)
	}
}

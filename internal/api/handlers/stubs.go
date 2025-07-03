package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Métodos ainda não implementados - stubs

func (h *Handler) TestMonitor(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented yet"})
}

func (h *Handler) GetGrafanaLink(c *gin.Context) {
	monitorID := c.Param("id")
	tenantID := c.GetString("tenant_id")

	// TODO: Implementar geração de link do Grafana
	grafanaURL := "https://grafana.elvenobservability.com/d/uptime-guardian/monitor?var-monitor_id=" + monitorID + "&var-tenant_id=" + tenantID

	c.JSON(http.StatusOK, gin.H{
		"url": grafanaURL,
	})
}

func (h *Handler) BulkCreateMonitors(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented yet"})
}

func (h *Handler) BulkUpdateMonitors(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented yet"})
}

func (h *Handler) ListNotificationChannels(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented yet"})
}

func (h *Handler) CreateNotificationChannel(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented yet"})
}

func (h *Handler) UpdateNotificationChannel(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented yet"})
}

func (h *Handler) DeleteNotificationChannel(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented yet"})
}

func (h *Handler) TestNotification(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented yet"})
}

func (h *Handler) GetOverview(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	// TODO: Implementar overview completo
	total, _ := h.repo.CountMonitorsByTenant(tenantID)

	c.JSON(http.StatusOK, gin.H{
		"total_monitors":   total,
		"monitors_up":      0,
		"monitors_down":    0,
		"incidents_active": 0,
	})
}

func (h *Handler) GetStatusPage(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented yet"})
}

func (h *Handler) GetSettings(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented yet"})
}

func (h *Handler) UpdateSettings(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented yet"})
}

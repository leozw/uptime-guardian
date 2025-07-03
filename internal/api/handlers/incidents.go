package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/leozw/uptime-guardian/internal/db"
	"github.com/leozw/uptime-guardian/internal/incidents"
	"github.com/leozw/uptime-guardian/internal/sla"
	"go.uber.org/zap"
)

func (h *Handler) GetMonitorIncidents(c *gin.Context) {
	monitorID := c.Param("id")
	tenantID := c.GetString("tenant_id")

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit < 1 || limit > 100 {
		limit = 50
	}

	incidents, err := h.repo.GetIncidentsByMonitor(monitorID, tenantID, limit)
	if err != nil {
		h.logger.Error("Failed to get incidents", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"incidents": incidents})
}

func (h *Handler) GetIncidentDetails(c *gin.Context) {
	incidentID := c.Param("incident_id")
	tenantID := c.GetString("tenant_id")

	incident, err := h.repo.GetIncident(incidentID, tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Incident not found"})
		return
	}

	events, err := h.repo.GetIncidentEvents(incidentID)
	if err != nil {
		h.logger.Error("Failed to get incident events", zap.Error(err))
		events = []*db.IncidentEvent{}
	}

	c.JSON(http.StatusOK, gin.H{
		"incident": incident,
		"events":   events,
	})
}

func (h *Handler) AcknowledgeIncident(c *gin.Context) {
	incidentID := c.Param("incident_id")
	tenantID := c.GetString("tenant_id")
	userEmail := c.GetString("user_email")

	incidentService := incidents.NewService(h.repo, h.logger)

	if err := incidentService.AcknowledgeIncident(incidentID, tenantID, userEmail); err != nil {
		h.logger.Error("Failed to acknowledge incident", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Incident acknowledged"})
}

func (h *Handler) AddIncidentComment(c *gin.Context) {
	incidentID := c.Param("incident_id")
	tenantID := c.GetString("tenant_id")
	userEmail := c.GetString("user_email")

	var req struct {
		Comment string `json:"comment" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	incidentService := incidents.NewService(h.repo, h.logger)

	if err := incidentService.AddIncidentComment(incidentID, tenantID, userEmail, req.Comment); err != nil {
		h.logger.Error("Failed to add incident comment", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Comment added"})
}

func (h *Handler) GetMonitorSLA(c *gin.Context) {
	monitorID := c.Param("id")
	tenantID := c.GetString("tenant_id")

	// Verificar se o monitor pertence ao tenant
	monitor, err := h.repo.GetMonitor(monitorID, tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Monitor not found"})
		return
	}

	calc := sla.NewCalculator(h.repo, h.logger)

	// SLA do mês atual
	currentSLA, err := calc.GetCurrentMonthSLA(monitorID)
	if err != nil {
		h.logger.Error("Failed to calculate current SLA", zap.Error(err))
		currentSLA = nil
	}

	// Histórico de SLA
	history, err := calc.GetSLAHistory(monitorID, 12) // últimos 12 meses
	if err != nil {
		h.logger.Error("Failed to get SLA history", zap.Error(err))
		history = []*db.SLAReport{}
	}

	// SLO configurado
	slo, _ := h.repo.GetMonitorSLO(monitorID)

	c.JSON(http.StatusOK, gin.H{
		"monitor":     monitor,
		"slo":         slo,
		"current_sla": currentSLA,
		"history":     history,
	})
}

func (h *Handler) SetMonitorSLO(c *gin.Context) {
	monitorID := c.Param("id")
	tenantID := c.GetString("tenant_id")

	// Verificar se o monitor pertence ao tenant
	_, err := h.repo.GetMonitor(monitorID, tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Monitor not found"})
		return
	}

	var req struct {
		TargetUptimePercentage float64 `json:"target_uptime_percentage" binding:"required,min=0,max=100"`
		MeasurementPeriodDays  int     `json:"measurement_period_days" binding:"required,min=1"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	slo := &db.MonitorSLO{
		ID:                     uuid.New().String(),
		MonitorID:              monitorID,
		TenantID:               tenantID,
		TargetUptimePercentage: req.TargetUptimePercentage,
		MeasurementPeriodDays:  req.MeasurementPeriodDays,
		CreatedAt:              time.Now(),
		UpdatedAt:              time.Now(),
	}

	if err := h.repo.CreateOrUpdateSLO(slo); err != nil {
		h.logger.Error("Failed to set SLO", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set SLO"})
		return
	}

	c.JSON(http.StatusOK, slo)
}

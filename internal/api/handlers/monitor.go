package handlers

import (
    "net/http"
    "strconv"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "github.com/leozw/uptime-guardian/internal/db"
    "go.uber.org/zap"
)

type CreateMonitorRequest struct {
    Name             string                    `json:"name" binding:"required,min=1,max=255"`
    Type             string                    `json:"type" binding:"required,oneof=http ssl dns domain"`
    Target           string                    `json:"target" binding:"required"`
    Enabled          *bool                     `json:"enabled" binding:"required"`
    Interval         int                       `json:"interval" binding:"required,min=30,max=86400"`
    Timeout          int                       `json:"timeout" binding:"required,min=1,max=60"`
    Regions          []string                  `json:"regions" binding:"required,min=1,dive,oneof=us-east eu-west asia-pac"`
    Config           db.MonitorConfig          `json:"config" binding:"required"`
    NotificationConf *db.NotificationConfig    `json:"notification_config"`
    Tags             map[string]interface{}    `json:"tags"`
}

func (h *Handler) CreateMonitor(c *gin.Context) {
    var req CreateMonitorRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    tenantID := c.GetString("tenant_id")
    userEmail := c.GetString("user_email")
    
    // Validate quota
    count, err := h.repo.CountMonitorsByTenant(tenantID)
    if err != nil {
        h.logger.Error("Failed to count monitors", zap.Error(err))
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
        return
    }
    
    // TODO: Get limit from customer plan
    if count >= 100 {
        c.JSON(http.StatusPaymentRequired, gin.H{"error": "Monitor limit exceeded for your plan"})
        return
    }
    
    // Validate monitor config based on type
    if err := h.validateMonitorConfig(req.Type, req.Config); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    monitor := &db.Monitor{
        ID:               uuid.New().String(),
        TenantID:         tenantID,
        Name:             req.Name,
        Type:             db.MonitorType(req.Type),
        Target:           req.Target,
        Enabled:          *req.Enabled,
        Interval:         req.Interval,
        Timeout:          req.Timeout,
        Regions:          req.Regions,
        Config:           req.Config,
        Tags:             db.JSONB(req.Tags),
        CreatedBy:        userEmail,
    }
    
    if req.NotificationConf != nil {
        monitor.NotificationConf = *req.NotificationConf
    }
    
    if err := h.repo.CreateMonitor(monitor); err != nil {
        h.logger.Error("Failed to create monitor", zap.Error(err))
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create monitor"})
        return
    }
    
    h.logger.Info("Monitor created", 
        zap.String("monitor_id", monitor.ID),
        zap.String("tenant_id", tenantID),
        zap.String("user", userEmail),
    )
    
    c.JSON(http.StatusCreated, monitor)
}

func (h *Handler) GetMonitor(c *gin.Context) {
    monitorID := c.Param("id")
    tenantID := c.GetString("tenant_id")
    
    monitor, err := h.repo.GetMonitor(monitorID, tenantID)
    if err != nil {
        if err.Error() == "monitor not found" {
            c.JSON(http.StatusNotFound, gin.H{"error": "Monitor not found"})
            return
        }
        h.logger.Error("Failed to get monitor", zap.Error(err))
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
        return
    }
    
    c.JSON(http.StatusOK, monitor)
}

func (h *Handler) ListMonitors(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    
    // Pagination
    page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
    limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
    
    if page < 1 {
        page = 1
    }
    if limit < 1 || limit > 100 {
        limit = 20
    }
    
    offset := (page - 1) * limit
    
    monitors, err := h.repo.GetMonitorsByTenant(tenantID, limit, offset)
    if err != nil {
        h.logger.Error("Failed to list monitors", zap.Error(err))
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
        return
    }
    
    total, _ := h.repo.CountMonitorsByTenant(tenantID)
    
    c.JSON(http.StatusOK, gin.H{
        "monitors": monitors,
        "pagination": gin.H{
            "page":  page,
            "limit": limit,
            "total": total,
        },
    })
}

func (h *Handler) UpdateMonitor(c *gin.Context) {
    monitorID := c.Param("id")
    tenantID := c.GetString("tenant_id")
    
    // Get existing monitor
    monitor, err := h.repo.GetMonitor(monitorID, tenantID)
    if err != nil {
        if err.Error() == "monitor not found" {
            c.JSON(http.StatusNotFound, gin.H{"error": "Monitor not found"})
            return
        }
        h.logger.Error("Failed to get monitor", zap.Error(err))
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
        return
    }
    
    var req CreateMonitorRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    // Update fields
    monitor.Name = req.Name
    monitor.Type = db.MonitorType(req.Type)
    monitor.Target = req.Target
    monitor.Enabled = *req.Enabled
    monitor.Interval = req.Interval
    monitor.Timeout = req.Timeout
    monitor.Regions = req.Regions
    monitor.Config = req.Config
    monitor.Tags = db.JSONB(req.Tags)
    
    if req.NotificationConf != nil {
        monitor.NotificationConf = *req.NotificationConf
    }
    
    if err := h.repo.UpdateMonitor(monitor); err != nil {
        h.logger.Error("Failed to update monitor", zap.Error(err))
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update monitor"})
        return
    }
    
    c.JSON(http.StatusOK, monitor)
}

func (h *Handler) DeleteMonitor(c *gin.Context) {
    monitorID := c.Param("id")
    tenantID := c.GetString("tenant_id")
    
    if err := h.repo.DeleteMonitor(monitorID, tenantID); err != nil {
        h.logger.Error("Failed to delete monitor", zap.Error(err))
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete monitor"})
        return
    }
    
    c.JSON(http.StatusNoContent, nil)
}

func (h *Handler) EnableMonitor(c *gin.Context) {
    h.toggleMonitor(c, true)
}

func (h *Handler) DisableMonitor(c *gin.Context) {
    h.toggleMonitor(c, false)
}

func (h *Handler) toggleMonitor(c *gin.Context, enabled bool) {
    monitorID := c.Param("id")
    tenantID := c.GetString("tenant_id")
    
    monitor, err := h.repo.GetMonitor(monitorID, tenantID)
    if err != nil {
        if err.Error() == "monitor not found" {
            c.JSON(http.StatusNotFound, gin.H{"error": "Monitor not found"})
            return
        }
        h.logger.Error("Failed to get monitor", zap.Error(err))
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
        return
    }
    
    monitor.Enabled = enabled
    
    if err := h.repo.UpdateMonitor(monitor); err != nil {
        h.logger.Error("Failed to update monitor", zap.Error(err))
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update monitor"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"enabled": enabled})
}

func (h *Handler) GetMonitorStatus(c *gin.Context) {
    monitorID := c.Param("id")
    tenantID := c.GetString("tenant_id")
    
    status, err := h.repo.GetMonitorStatus(monitorID, tenantID)
    if err != nil {
        if err.Error() == "status not found" {
            c.JSON(http.StatusNotFound, gin.H{"error": "No status available yet"})
            return
        }
        h.logger.Error("Failed to get monitor status", zap.Error(err))
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
        return
    }
    
    c.JSON(http.StatusOK, status)
}

func (h *Handler) GetMonitorHistory(c *gin.Context) {
    monitorID := c.Param("id")
    tenantID := c.GetString("tenant_id")
    
    limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
    if limit < 1 || limit > 1000 {
        limit = 100
    }
    
    history, err := h.repo.GetCheckHistory(monitorID, tenantID, limit)
    if err != nil {
        h.logger.Error("Failed to get monitor history", zap.Error(err))
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"history": history})
}

func (h *Handler) validateMonitorConfig(monitorType string, config db.MonitorConfig) error {
    // TODO: Implement validation based on monitor type
    return nil
}
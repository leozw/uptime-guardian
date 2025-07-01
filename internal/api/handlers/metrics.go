package handlers

import (
    "net/http"

    "github.com/gin-gonic/gin"
    "github.com/leozw/uptime-guardian/internal/storage/postgres"
)

type MetricsHandler struct {
    db *postgres.DB
}

func NewMetricsHandler(db *postgres.DB) *MetricsHandler {
    return &MetricsHandler{db: db}
}

func (h *MetricsHandler) GetOverview(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    
    // Get tenant stats
    stats, err := h.db.GetTenantStats(tenantID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get statistics"})
        return
    }
    
    c.JSON(http.StatusOK, stats)
}

func (h *MetricsHandler) GetDomainMetrics(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    domainID := c.Param("id")
    
    // Verify domain belongs to tenant
    _, err := h.db.GetDomain(domainID, tenantID)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Domain not found"})
        return
    }
    
    // Get check history
    history, err := h.db.GetCheckHistory(domainID, 24) // Last 24 hours
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get metrics"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "domain_id": domainID,
        "history":   history,
    })
}
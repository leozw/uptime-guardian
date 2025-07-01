package handlers

import (
    "net/http"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    
    "github.com/leozw/uptime-guardian/internal/core"
    "github.com/leozw/uptime-guardian/internal/queue"
    "github.com/leozw/uptime-guardian/internal/storage/postgres"
)

type DomainHandler struct {
    db    *postgres.DB
    queue *queue.RedisQueue
}

func NewDomainHandler(db *postgres.DB, queue *queue.RedisQueue) *DomainHandler {
    return &DomainHandler{db: db, queue: queue}
}

type CreateDomainRequest struct {
    Domain string            `json:"domain" binding:"required,hostname"`
    Labels map[string]string `json:"labels,omitempty"`
}

func (h *DomainHandler) ListDomains(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    
    domains, err := h.db.ListDomains(tenantID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list domains"})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "domains": domains,
        "count":   len(domains),
    })
}

func (h *DomainHandler) CreateDomain(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    
    var req CreateDomainRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // Check domain limit
    count, err := h.db.CountDomains(tenantID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check domain limit"})
        return
    }

    tenant, err := h.db.GetTenant(tenantID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get tenant"})
        return
    }

    if count >= tenant.MaxDomains {
        c.JSON(http.StatusForbidden, gin.H{"error": "Domain limit reached"})
        return
    }

    // Check if domain already exists for this tenant
    exists, err := h.db.DomainExists(tenantID, req.Domain)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check domain"})
        return
    }
    if exists {
        c.JSON(http.StatusConflict, gin.H{"error": "Domain already exists"})
        return
    }

    // Create domain
    domain := &core.Domain{
        ID:            uuid.New(),
        TenantID:      uuid.MustParse(tenantID),
        Name:          req.Domain,
        CheckInterval: time.Duration(tenant.CheckIntervalMin) * time.Minute,
        Enabled:       true,
        Labels:        req.Labels,
        CreatedAt:     time.Now(),
        UpdatedAt:     time.Now(),
    }

    if err := h.db.CreateDomain(domain); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create domain"})
        return
    }

    // Queue immediate check
    job := &queue.Job{
        ID:        uuid.New().String(),
        Type:      "domain_check",
        DomainID:  domain.ID.String(),
        TenantID:  tenantID,
        CreatedAt: time.Now(),
    }

    if err := h.queue.Push(c.Request.Context(), job); err != nil {
        // Log error but don't fail the request
        c.Set("queue_error", err.Error())
    }

    c.JSON(http.StatusCreated, domain)
}

func (h *DomainHandler) GetDomain(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    domainID := c.Param("id")

    domain, err := h.db.GetDomain(domainID, tenantID)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Domain not found"})
        return
    }

    c.JSON(http.StatusOK, domain)
}

func (h *DomainHandler) UpdateDomain(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    domainID := c.Param("id")

    var req struct {
        Enabled bool              `json:"enabled"`
        Labels  map[string]string `json:"labels,omitempty"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    domain, err := h.db.GetDomain(domainID, tenantID)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Domain not found"})
        return
    }

    domain.Enabled = req.Enabled
    if req.Labels != nil {
        domain.Labels = req.Labels
    }
    domain.UpdatedAt = time.Now()

    if err := h.db.UpdateDomain(domain); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update domain"})
        return
    }

    c.JSON(http.StatusOK, domain)
}

func (h *DomainHandler) DeleteDomain(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    domainID := c.Param("id")

    if err := h.db.DeleteDomain(domainID, tenantID); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete domain"})
        return
    }

    c.JSON(http.StatusNoContent, nil)
}

func (h *DomainHandler) GetDomainHealth(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    domainID := c.Param("id")

    domain, err := h.db.GetDomain(domainID, tenantID)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Domain not found"})
        return
    }

    // Get latest check results
    results, err := h.db.GetLatestCheckResults(domainID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get check results"})
        return
    }

    // Calculate health
    health := calculateDomainHealth(domain, results)

    c.JSON(http.StatusOK, health)
}

func (h *DomainHandler) TriggerCheck(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    domainID := c.Param("id")

    _, err := h.db.GetDomain(domainID, tenantID)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Domain not found"})
        return
    }

    // Queue check
    job := &queue.Job{
        ID:        uuid.New().String(),
        Type:      "domain_check",
        DomainID:  domainID,
        TenantID:  tenantID,
        Priority:  1, // High priority for manual triggers
        CreatedAt: time.Now(),
    }

    if err := h.queue.Push(c.Request.Context(), job); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to queue check"})
        return
    }

    c.JSON(http.StatusAccepted, gin.H{
        "message": "Check queued successfully",
        "job_id":  job.ID,
    })
}

func calculateDomainHealth(domain *core.Domain, results map[string]*core.CheckResult) *core.DomainHealth {
    health := &core.DomainHealth{
        OverallScore: 100,
        Breakdown:    make(map[string]int),
        Issues:       []core.HealthIssue{},
        LastCheck:    time.Now(),
    }

    // SSL Score
    if ssl, ok := results["ssl"]; ok && ssl != nil {
        if ssl.Success {
            health.Breakdown["ssl"] = 100
        } else {
            health.Breakdown["ssl"] = 0
            health.OverallScore -= 30
            health.Issues = append(health.Issues, core.HealthIssue{
                Severity:    "critical",
                Category:    "ssl",
                Description: "SSL certificate check failed",
                Impact:      "Site may be inaccessible over HTTPS",
            })
        }
    }

    // DNS Score
    if dns, ok := results["dns"]; ok && dns != nil {
        if dns.Success {
            health.Breakdown["dns"] = 100
        } else {
            health.Breakdown["dns"] = 0
            health.OverallScore -= 40
            health.Issues = append(health.Issues, core.HealthIssue{
                Severity:    "critical",
                Category:    "dns",
                Description: "DNS resolution failed",
                Impact:      "Domain is not resolving properly",
            })
        }
    }

    // HTTP Score
    if http, ok := results["http"]; ok && http != nil {
        if http.Success {
            health.Breakdown["http"] = 100
        } else {
            health.Breakdown["http"] = 50
            health.OverallScore -= 20
            health.Issues = append(health.Issues, core.HealthIssue{
                Severity:    "warning",
                Category:    "http",
                Description: "HTTP check failed",
                Impact:      "Site may be experiencing issues",
            })
        }
    }

    if health.OverallScore < 0 {
        health.OverallScore = 0
    }

    return health
}
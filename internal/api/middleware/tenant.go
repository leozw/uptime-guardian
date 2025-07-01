package middleware

import (
    "net/http"

    "github.com/gin-gonic/gin"
    "github.com/leozw/uptime-guardian/internal/storage/postgres"
)

func TenantContext(db *postgres.DB) gin.HandlerFunc {
    return func(c *gin.Context) {
        tenantID := c.GetString("tenant_id")
        if tenantID == "" {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Tenant ID not found"})
            c.Abort()
            return
        }

        tenant, err := db.GetTenant(tenantID)
        if err != nil {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid tenant"})
            c.Abort()
            return
        }

        if !tenant.IsActive {
            c.JSON(http.StatusForbidden, gin.H{"error": "Account is disabled"})
            c.Abort()
            return
        }

        c.Set("tenant", tenant)
        c.Next()
    }
}
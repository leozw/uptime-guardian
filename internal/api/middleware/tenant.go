package middleware

import (
    "net/http"

    "github.com/gin-gonic/gin"
    "github.com/golang-jwt/jwt/v5"
)

func Tenant() gin.HandlerFunc {
    return func(c *gin.Context) {
        claims, exists := c.Get("claims")
        if !exists {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Claims not found"})
            c.Abort()
            return
        }
        
        jwtClaims := claims.(jwt.MapClaims)
        
        // Extract organization as tenant_id
        organization, ok := jwtClaims["organization"].(string)
        if !ok || organization == "" {
            c.JSON(http.StatusBadRequest, gin.H{"error": "Organization not found in token"})
            c.Abort()
            return
        }
        
        c.Set("tenant_id", organization)
        c.Set("X-Scope-OrgID", organization)
        
        c.Next()
    }
}
package handlers

import (
	"github.com/gin-gonic/gin"
)

// Webhook handlers - placeholder for now
func (h *Handler) WebhookHandler(c *gin.Context) {
	// TODO: Implement webhook handling
	c.JSON(200, gin.H{"status": "ok"})
}

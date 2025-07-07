package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/leozw/uptime-guardian/internal/db"
	"go.uber.org/zap"
)

type CreateMonitorGroupRequest struct {
	Name             string                 `json:"name" binding:"required,min=1,max=255"`
	Description      string                 `json:"description"`
	Enabled          *bool                  `json:"enabled" binding:"required"`
	Tags             map[string]interface{} `json:"tags"`
	NotificationConf *db.NotificationConfig `json:"notification_config"`
	Members          []GroupMemberRequest   `json:"members" binding:"required,min=1"`
}

type GroupMemberRequest struct {
	MonitorID  string  `json:"monitor_id" binding:"required"`
	Weight     float64 `json:"weight" binding:"required,min=0,max=1"`
	IsCritical bool    `json:"is_critical"`
}

type GroupAlertRuleRequest struct {
	Name                 string                  `json:"name" binding:"required"`
	Enabled              bool                    `json:"enabled"`
	TriggerCondition     string                  `json:"trigger_condition" binding:"required,oneof=health_score_below any_critical_down percentage_down all_down"`
	ThresholdValue       *float64                `json:"threshold_value"`
	NotificationChannels db.NotificationChannels `json:"notification_channels"`
	CooldownMinutes      int                     `json:"cooldown_minutes" binding:"min=0"`
}

func (h *Handler) CreateMonitorGroup(c *gin.Context) {
	var req CreateMonitorGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Invalid request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenantID := c.GetString("tenant_id")
	userEmail := c.GetString("user_email")

	h.logger.Info("Creating monitor group",
		zap.String("tenant_id", tenantID),
		zap.String("user_email", userEmail),
		zap.String("group_name", req.Name),
		zap.Int("members_count", len(req.Members)),
	)

	// Validate quota
	count, err := h.repo.CountMonitorGroupsByTenant(tenantID)
	if err != nil {
		h.logger.Error("Failed to count monitor groups", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	// TODO: Get limit from customer plan
	if count >= 50 {
		c.JSON(http.StatusPaymentRequired, gin.H{"error": "Monitor group limit exceeded for your plan"})
		return
	}

	// Validate members ANTES de iniciar a transação
	totalWeight := 0.0
	validatedMembers := make([]GroupMemberRequest, 0, len(req.Members))

	for _, member := range req.Members {
		totalWeight += member.Weight

		// Verify monitor exists and belongs to tenant
		monitor, err := h.repo.GetMonitor(member.MonitorID, tenantID)
		if err != nil {
			h.logger.Error("Monitor not found for group member",
				zap.String("monitor_id", member.MonitorID),
				zap.String("tenant_id", tenantID),
				zap.Error(err),
			)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Monitor not found: " + member.MonitorID})
			return
		}
		if monitor == nil {
			h.logger.Error("Monitor is nil",
				zap.String("monitor_id", member.MonitorID),
				zap.String("tenant_id", tenantID),
			)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid monitor: " + member.MonitorID})
			return
		}

		h.logger.Debug("Validated monitor for group",
			zap.String("monitor_id", member.MonitorID),
			zap.String("monitor_name", monitor.Name),
			zap.Float64("weight", member.Weight),
			zap.Bool("is_critical", member.IsCritical),
		)

		validatedMembers = append(validatedMembers, member)
	}

	// CORREÇÃO: Weights should sum to approximately 1.0 (tolerância maior)
	if len(validatedMembers) > 0 && (totalWeight < 0.99 || totalWeight > 1.01) {
		h.logger.Error("Invalid total weight",
			zap.Float64("total_weight", totalWeight),
			zap.Int("members_count", len(validatedMembers)),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Member weights must sum to 1.0 (current: %.3f)", totalWeight)})
		return
	}

	groupID := uuid.New().String()
	group := &db.MonitorGroup{
		ID:          groupID,
		TenantID:    tenantID,
		Name:        req.Name,
		Description: req.Description,
		Enabled:     *req.Enabled,
		Tags:        db.JSONB(req.Tags),
		CreatedBy:   userEmail,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if req.NotificationConf != nil {
		group.NotificationConf = *req.NotificationConf
	}

	// CORREÇÃO: Start transaction - Usar sqlx.Tx
	tx, err := h.repo.BeginTx()
	if err != nil {
		h.logger.Error("Failed to start transaction", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create monitor group"})
		return
	}

	// Usar defer para garantir rollback em caso de erro
	committed := false
	defer func() {
		if !committed {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				h.logger.Error("Failed to rollback transaction", zap.Error(rollbackErr))
			}
		}
	}()

	// Create group
	if err := h.repo.CreateMonitorGroup(group); err != nil {
		h.logger.Error("Failed to create monitor group", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create monitor group"})
		return
	}

	h.logger.Info("Created monitor group",
		zap.String("group_id", group.ID),
		zap.String("group_name", group.Name),
	)

	// Add members
	for i, member := range validatedMembers {
		h.logger.Debug("Adding member to group",
			zap.Int("member_index", i),
			zap.String("monitor_id", member.MonitorID),
			zap.String("group_id", group.ID),
			zap.Float64("weight", member.Weight),
			zap.Bool("is_critical", member.IsCritical),
		)

		if err := h.repo.AddMonitorToGroup(group.ID, member.MonitorID, member.Weight, member.IsCritical); err != nil {
			h.logger.Error("Failed to add monitor to group",
				zap.Error(err),
				zap.String("monitor_id", member.MonitorID),
				zap.String("group_id", group.ID),
			)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add monitors to group"})
			return
		}

		h.logger.Debug("Successfully added member to group",
			zap.String("monitor_id", member.MonitorID),
			zap.String("group_id", group.ID),
		)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		h.logger.Error("Failed to commit transaction", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create monitor group"})
		return
	}
	committed = true

	h.logger.Info("Monitor group created successfully",
		zap.String("group_id", group.ID),
		zap.String("tenant_id", tenantID),
		zap.String("user", userEmail),
		zap.Int("members_added", len(validatedMembers)),
	)

	// Load members from database for accurate response
	members, err := h.repo.GetGroupMembers(group.ID)
	if err != nil {
		h.logger.Error("Failed to load group members for response", zap.Error(err))
		// Fallback to basic member info
		group.Members = make([]db.MonitorGroupMember, len(validatedMembers))
		for i, member := range validatedMembers {
			group.Members[i] = db.MonitorGroupMember{
				GroupID:    group.ID,
				MonitorID:  member.MonitorID,
				Weight:     member.Weight,
				IsCritical: member.IsCritical,
			}
		}
	} else {
		// Use real data from database
		group.Members = make([]db.MonitorGroupMember, len(members))
		for i, member := range members {
			group.Members[i] = *member
		}
		h.logger.Info("Loaded group members from database",
			zap.String("group_id", group.ID),
			zap.Int("members_loaded", len(members)),
		)
	}

	c.JSON(http.StatusCreated, group)
}

func (h *Handler) GetMonitorGroup(c *gin.Context) {
	groupID := c.Param("id")
	tenantID := c.GetString("tenant_id")

	group, err := h.repo.GetMonitorGroup(groupID, tenantID)
	if err != nil {
		if err.Error() == "monitor group not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Monitor group not found"})
			return
		}
		h.logger.Error("Failed to get monitor group", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	// Load members COM os dados dos monitors
	members, err := h.repo.GetGroupMembers(groupID)
	if err != nil {
		h.logger.Error("Failed to get group members", zap.Error(err))
		// Não falha a requisição, apenas retorna grupo sem members
		group.Members = []db.MonitorGroupMember{}
	} else {
		group.Members = make([]db.MonitorGroupMember, len(members))
		for i, member := range members {
			group.Members[i] = *member
		}
	}

	// Load status
	status, err := h.repo.GetGroupStatus(groupID)
	if err == nil {
		group.Status = status
	} else {
		// Calculate status on the fly se não existir
		group.Status = h.calculateGroupStatus(group)
	}

	c.JSON(http.StatusOK, group)
}

func (h *Handler) ListMonitorGroups(c *gin.Context) {
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

	groups, err := h.repo.GetMonitorGroupsByTenant(tenantID, limit, offset)
	if err != nil {
		h.logger.Error("Failed to list monitor groups", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	// Load status e members para cada grupo
	for _, group := range groups {
		// Load status
		status, err := h.repo.GetGroupStatus(group.ID)
		if err == nil {
			group.Status = status
		}

		// CORREÇÃO: Carregar members básicos (sem dados completos dos monitors para performance)
		members, err := h.repo.GetGroupMembersBasic(group.ID)
		if err == nil {
			group.Members = make([]db.MonitorGroupMember, len(members))
			for i, member := range members {
				group.Members[i] = *member
			}
		} else {
			// Fallback: pelo menos definir como array vazio
			group.Members = []db.MonitorGroupMember{}
		}
	}

	total, _ := h.repo.CountMonitorGroupsByTenant(tenantID)

	c.JSON(http.StatusOK, gin.H{
		"groups": groups,
		"pagination": gin.H{
			"page":  page,
			"limit": limit,
			"total": total,
		},
	})
}

func (h *Handler) UpdateMonitorGroup(c *gin.Context) {
	groupID := c.Param("id")
	tenantID := c.GetString("tenant_id")

	// Get existing group
	group, err := h.repo.GetMonitorGroup(groupID, tenantID)
	if err != nil {
		if err.Error() == "monitor group not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Monitor group not found"})
			return
		}
		h.logger.Error("Failed to get monitor group", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	var req CreateMonitorGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update fields
	group.Name = req.Name
	group.Description = req.Description
	group.Enabled = *req.Enabled
	group.Tags = db.JSONB(req.Tags)
	group.UpdatedAt = time.Now()

	if req.NotificationConf != nil {
		group.NotificationConf = *req.NotificationConf
	}

	if err := h.repo.UpdateMonitorGroup(group); err != nil {
		h.logger.Error("Failed to update monitor group", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update monitor group"})
		return
	}

	c.JSON(http.StatusOK, group)
}

func (h *Handler) DeleteMonitorGroup(c *gin.Context) {
	groupID := c.Param("id")
	tenantID := c.GetString("tenant_id")

	if err := h.repo.DeleteMonitorGroup(groupID, tenantID); err != nil {
		h.logger.Error("Failed to delete monitor group", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete monitor group"})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

func (h *Handler) AddMonitorToGroup(c *gin.Context) {
	groupID := c.Param("id")
	tenantID := c.GetString("tenant_id")

	// Verify group exists and belongs to tenant
	_, err := h.repo.GetMonitorGroup(groupID, tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Monitor group not found"})
		return
	}

	var req GroupMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify monitor exists and belongs to tenant
	_, err = h.repo.GetMonitor(req.MonitorID, tenantID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Monitor not found"})
		return
	}

	if err := h.repo.AddMonitorToGroup(groupID, req.MonitorID, req.Weight, req.IsCritical); err != nil {
		h.logger.Error("Failed to add monitor to group", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add monitor to group"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Monitor added to group"})
}

func (h *Handler) RemoveMonitorFromGroup(c *gin.Context) {
	groupID := c.Param("id")
	monitorID := c.Param("monitor_id")
	tenantID := c.GetString("tenant_id")

	// Verify group exists and belongs to tenant
	_, err := h.repo.GetMonitorGroup(groupID, tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Monitor group not found"})
		return
	}

	if err := h.repo.RemoveMonitorFromGroup(groupID, monitorID); err != nil {
		h.logger.Error("Failed to remove monitor from group", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove monitor from group"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Monitor removed from group"})
}

func (h *Handler) GetMonitorGroupStatus(c *gin.Context) {
	groupID := c.Param("id")
	tenantID := c.GetString("tenant_id")

	// Verify group belongs to tenant
	group, err := h.repo.GetMonitorGroup(groupID, tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Monitor group not found"})
		return
	}

	status, err := h.repo.GetGroupStatus(groupID)
	if err != nil {
		if err.Error() == "group status not found" {
			// Calculate status on the fly
			status = h.calculateGroupStatus(group)
			c.JSON(http.StatusOK, status)
			return
		}
		h.logger.Error("Failed to get group status", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, status)
}

func (h *Handler) GetMonitorGroupIncidents(c *gin.Context) {
	groupID := c.Param("id")
	tenantID := c.GetString("tenant_id")

	// Verify group belongs to tenant
	_, err := h.repo.GetMonitorGroup(groupID, tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Monitor group not found"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit < 1 || limit > 100 {
		limit = 50
	}

	incidents, err := h.repo.GetGroupIncidents(groupID, tenantID, limit)
	if err != nil {
		h.logger.Error("Failed to get group incidents", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"incidents": incidents})
}

func (h *Handler) SetMonitorGroupSLO(c *gin.Context) {
	groupID := c.Param("id")
	tenantID := c.GetString("tenant_id")

	// Verify group belongs to tenant
	_, err := h.repo.GetMonitorGroup(groupID, tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Monitor group not found"})
		return
	}

	var req struct {
		TargetUptimePercentage float64 `json:"target_uptime_percentage" binding:"required,min=0,max=100"`
		MeasurementPeriodDays  int     `json:"measurement_period_days" binding:"required,min=1"`
		CalculationMethod      string  `json:"calculation_method" binding:"required,oneof=weighted_average worst_case critical_only"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	slo := &db.MonitorGroupSLO{
		ID:                     uuid.New().String(),
		GroupID:                groupID,
		TenantID:               tenantID,
		TargetUptimePercentage: req.TargetUptimePercentage,
		MeasurementPeriodDays:  req.MeasurementPeriodDays,
		CalculationMethod:      req.CalculationMethod,
		CreatedAt:              time.Now(),
		UpdatedAt:              time.Now(),
	}

	if err := h.repo.CreateOrUpdateGroupSLO(slo); err != nil {
		h.logger.Error("Failed to set group SLO", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set SLO"})
		return
	}

	c.JSON(http.StatusOK, slo)
}

func (h *Handler) CreateGroupAlertRule(c *gin.Context) {
	groupID := c.Param("id")
	tenantID := c.GetString("tenant_id")

	// Verify group belongs to tenant
	_, err := h.repo.GetMonitorGroup(groupID, tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Monitor group not found"})
		return
	}

	var req GroupAlertRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate threshold value for certain conditions
	if req.TriggerCondition == db.TriggerHealthScoreBelow || req.TriggerCondition == db.TriggerPercentageDown {
		if req.ThresholdValue == nil || *req.ThresholdValue < 0 || *req.ThresholdValue > 100 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Threshold value must be between 0 and 100"})
			return
		}
	}

	rule := &db.MonitorGroupAlertRule{
		ID:                   uuid.New().String(),
		GroupID:              groupID,
		Name:                 req.Name,
		Enabled:              req.Enabled,
		TriggerCondition:     req.TriggerCondition,
		ThresholdValue:       req.ThresholdValue,
		NotificationChannels: req.NotificationChannels,
		CooldownMinutes:      req.CooldownMinutes,
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	}

	if rule.CooldownMinutes == 0 {
		rule.CooldownMinutes = 5
	}

	if err := h.repo.CreateGroupAlertRule(rule); err != nil {
		h.logger.Error("Failed to create group alert rule", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create alert rule"})
		return
	}

	c.JSON(http.StatusCreated, rule)
}

// Helper method to calculate group status on the fly
func (h *Handler) calculateGroupStatus(group *db.MonitorGroup) *db.MonitorGroupStatus {
	status := &db.MonitorGroupStatus{
		GroupID:   group.ID,
		LastCheck: time.Now(),
	}

	// Get members
	members, err := h.repo.GetGroupMembers(group.ID)
	if err != nil {
		status.OverallStatus = db.StatusDegraded
		status.Message = "Failed to get member statuses"
		return status
	}

	// Get status for each member
	memberStatuses := make(map[string]db.CheckStatus)
	for _, member := range members {
		monitorStatus, err := h.repo.GetMonitorStatus(member.MonitorID, group.TenantID)
		if err == nil {
			memberStatuses[member.MonitorID] = monitorStatus.Status

			switch monitorStatus.Status {
			case db.StatusUp:
				status.MonitorsUp++
			case db.StatusDown:
				status.MonitorsDown++
				if member.IsCritical {
					status.CriticalMonitorsDown++
				}
			case db.StatusDegraded:
				status.MonitorsDegraded++
			}
		}
	}

	// Convert members slice for calculation
	group.Members = make([]db.MonitorGroupMember, len(members))
	for i, m := range members {
		group.Members[i] = *m
	}

	// Calculate health score and overall status
	status.HealthScore = group.CalculateHealthScore(memberStatuses)
	status.OverallStatus = group.DetermineOverallStatus(memberStatuses)

	// Generate message
	if status.OverallStatus == db.StatusUp {
		status.Message = "All monitors operational"
	} else if status.CriticalMonitorsDown > 0 {
		status.Message = "Critical monitors are down"
	} else if status.MonitorsDown > 0 {
		status.Message = "Some monitors are down"
	} else if status.MonitorsDegraded > 0 {
		status.Message = "Some monitors are degraded"
	}

	return status
}

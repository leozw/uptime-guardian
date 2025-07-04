package db

import (
	"database/sql"
	"fmt"
	"time"
)

// Monitor Group operations

func (r *Repository) CreateMonitorGroup(g *MonitorGroup) error {
	query := `
		INSERT INTO monitor_groups (
			id, tenant_id, name, description, enabled,
			tags, notification_config, created_at, updated_at, created_by
		) VALUES (
			:id, :tenant_id, :name, :description, :enabled,
			:tags, :notification_config, :created_at, :updated_at, :created_by
		)`

	_, err := r.db.NamedExec(query, g)
	return err
}

func (r *Repository) GetMonitorGroup(id, tenantID string) (*MonitorGroup, error) {
	var g MonitorGroup
	query := `SELECT * FROM monitor_groups WHERE id = $1 AND tenant_id = $2`
	err := r.db.Get(&g, query, id, tenantID)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("monitor group not found")
	}
	return &g, err
}

func (r *Repository) GetMonitorGroupsByTenant(tenantID string, limit, offset int) ([]*MonitorGroup, error) {
	groups := []*MonitorGroup{}
	query := `
		SELECT * FROM monitor_groups 
		WHERE tenant_id = $1 
		ORDER BY created_at DESC 
		LIMIT $2 OFFSET $3`

	err := r.db.Select(&groups, query, tenantID, limit, offset)
	return groups, err
}

func (r *Repository) UpdateMonitorGroup(g *MonitorGroup) error {
	query := `
		UPDATE monitor_groups SET
			name = :name,
			description = :description,
			enabled = :enabled,
			tags = :tags,
			notification_config = :notification_config,
			updated_at = :updated_at
		WHERE id = :id AND tenant_id = :tenant_id`

	_, err := r.db.NamedExec(query, g)
	return err
}

func (r *Repository) DeleteMonitorGroup(id, tenantID string) error {
	query := `DELETE FROM monitor_groups WHERE id = $1 AND tenant_id = $2`
	_, err := r.db.Exec(query, id, tenantID)
	return err
}

func (r *Repository) CountMonitorGroupsByTenant(tenantID string) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM monitor_groups WHERE tenant_id = $1`
	err := r.db.Get(&count, query, tenantID)
	return count, err
}

// Monitor Group Member operations

func (r *Repository) AddMonitorToGroup(groupID, monitorID string, weight float64, isCritical bool) error {
	query := `
		INSERT INTO monitor_group_members (
			group_id, monitor_id, weight, is_critical, added_at
		) VALUES (
			$1, $2, $3, $4, $5
		) ON CONFLICT (group_id, monitor_id) DO UPDATE SET
			weight = $3,
			is_critical = $4`

	_, err := r.db.Exec(query, groupID, monitorID, weight, isCritical, time.Now())
	return err
}

func (r *Repository) RemoveMonitorFromGroup(groupID, monitorID string) error {
	query := `DELETE FROM monitor_group_members WHERE group_id = $1 AND monitor_id = $2`
	_, err := r.db.Exec(query, groupID, monitorID)
	return err
}

func (r *Repository) GetGroupMembers(groupID string) ([]*MonitorGroupMember, error) {
	members := []*MonitorGroupMember{}
	query := `
		SELECT mgm.*, m.* 
		FROM monitor_group_members mgm
		JOIN monitors m ON mgm.monitor_id = m.id
		WHERE mgm.group_id = $1
		ORDER BY mgm.is_critical DESC, mgm.weight DESC`

	rows, err := r.db.Query(query, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var member MonitorGroupMember
		var monitor Monitor

		err := rows.Scan(
			&member.ID, &member.GroupID, &member.MonitorID,
			&member.Weight, &member.IsCritical, &member.AddedAt,
			&monitor.ID, &monitor.TenantID, &monitor.Name,
			&monitor.Type, &monitor.Target, &monitor.Enabled,
			&monitor.Interval, &monitor.Timeout, &monitor.Regions,
			&monitor.Config, &monitor.NotificationConf, &monitor.Tags,
			&monitor.CreatedAt, &monitor.UpdatedAt, &monitor.CreatedBy,
		)
		if err != nil {
			return nil, err
		}

		member.Monitor = &monitor
		members = append(members, &member)
	}

	return members, nil
}

func (r *Repository) GetMonitorGroups(monitorID string) ([]*MonitorGroup, error) {
	groups := []*MonitorGroup{}
	query := `
		SELECT g.* FROM monitor_groups g
		JOIN monitor_group_members mgm ON g.id = mgm.group_id
		WHERE mgm.monitor_id = $1
		ORDER BY g.name`

	err := r.db.Select(&groups, query, monitorID)
	return groups, err
}

// Monitor Group Status operations

func (r *Repository) SaveGroupStatus(status *MonitorGroupStatus) error {
	query := `
		INSERT INTO monitor_group_status (
			group_id, overall_status, health_score,
			monitors_up, monitors_down, monitors_degraded,
			critical_monitors_down, last_check, message
		) VALUES (
			:group_id, :overall_status, :health_score,
			:monitors_up, :monitors_down, :monitors_degraded,
			:critical_monitors_down, :last_check, :message
		) ON CONFLICT (group_id) DO UPDATE SET
			overall_status = :overall_status,
			health_score = :health_score,
			monitors_up = :monitors_up,
			monitors_down = :monitors_down,
			monitors_degraded = :monitors_degraded,
			critical_monitors_down = :critical_monitors_down,
			last_check = :last_check,
			message = :message`

	_, err := r.db.NamedExec(query, status)
	return err
}

func (r *Repository) GetGroupStatus(groupID string) (*MonitorGroupStatus, error) {
	var status MonitorGroupStatus
	query := `SELECT * FROM monitor_group_status WHERE group_id = $1`
	err := r.db.Get(&status, query, groupID)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("group status not found")
	}
	return &status, err
}

// Monitor Group SLO operations

func (r *Repository) CreateOrUpdateGroupSLO(slo *MonitorGroupSLO) error {
	query := `
		INSERT INTO monitor_group_slos (
			id, group_id, tenant_id, target_uptime_percentage,
			measurement_period_days, calculation_method,
			created_at, updated_at
		) VALUES (
			:id, :group_id, :tenant_id, :target_uptime_percentage,
			:measurement_period_days, :calculation_method,
			:created_at, :updated_at
		) ON CONFLICT (group_id) DO UPDATE SET
			target_uptime_percentage = :target_uptime_percentage,
			measurement_period_days = :measurement_period_days,
			calculation_method = :calculation_method,
			updated_at = :updated_at`

	_, err := r.db.NamedExec(query, slo)
	return err
}

func (r *Repository) GetGroupSLO(groupID string) (*MonitorGroupSLO, error) {
	var slo MonitorGroupSLO
	query := `SELECT * FROM monitor_group_slos WHERE group_id = $1`
	err := r.db.Get(&slo, query, groupID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &slo, err
}

// Monitor Group Alert Rules operations

func (r *Repository) CreateGroupAlertRule(rule *MonitorGroupAlertRule) error {
	query := `
		INSERT INTO monitor_group_alert_rules (
			id, group_id, name, enabled, trigger_condition,
			threshold_value, notification_channels, cooldown_minutes,
			created_at, updated_at
		) VALUES (
			:id, :group_id, :name, :enabled, :trigger_condition,
			:threshold_value, :notification_channels, :cooldown_minutes,
			:created_at, :updated_at
		)`

	_, err := r.db.NamedExec(query, rule)
	return err
}

func (r *Repository) GetGroupAlertRules(groupID string) ([]*MonitorGroupAlertRule, error) {
	rules := []*MonitorGroupAlertRule{}
	query := `
		SELECT * FROM monitor_group_alert_rules 
		WHERE group_id = $1 AND enabled = true
		ORDER BY created_at`

	err := r.db.Select(&rules, query, groupID)
	return rules, err
}

func (r *Repository) UpdateGroupAlertRule(rule *MonitorGroupAlertRule) error {
	query := `
		UPDATE monitor_group_alert_rules SET
			name = :name,
			enabled = :enabled,
			trigger_condition = :trigger_condition,
			threshold_value = :threshold_value,
			notification_channels = :notification_channels,
			cooldown_minutes = :cooldown_minutes,
			updated_at = :updated_at
		WHERE id = :id`

	_, err := r.db.NamedExec(query, rule)
	return err
}

func (r *Repository) DeleteGroupAlertRule(id string) error {
	query := `DELETE FROM monitor_group_alert_rules WHERE id = $1`
	_, err := r.db.Exec(query, id)
	return err
}

// Monitor Group Incident operations

func (r *Repository) CreateGroupIncident(incident *MonitorGroupIncident) error {
	query := `
		INSERT INTO monitor_group_incidents (
			id, group_id, tenant_id, started_at, severity,
			affected_monitors, root_cause_monitor_id,
			notifications_sent, health_score_at_start
		) VALUES (
			:id, :group_id, :tenant_id, :started_at, :severity,
			:affected_monitors, :root_cause_monitor_id,
			:notifications_sent, :health_score_at_start
		)`

	_, err := r.db.NamedExec(query, incident)
	return err
}

func (r *Repository) GetActiveGroupIncident(groupID string) (*MonitorGroupIncident, error) {
	var incident MonitorGroupIncident
	query := `
		SELECT * FROM monitor_group_incidents 
		WHERE group_id = $1 AND resolved_at IS NULL
		ORDER BY started_at DESC
		LIMIT 1`

	err := r.db.Get(&incident, query, groupID)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no active group incident")
	}
	return &incident, err
}

func (r *Repository) UpdateGroupIncident(incident *MonitorGroupIncident) error {
	query := `
		UPDATE monitor_group_incidents SET
			resolved_at = :resolved_at,
			affected_monitors = :affected_monitors,
			notifications_sent = :notifications_sent,
			acknowledged_at = :acknowledged_at,
			acknowledged_by = :acknowledged_by
		WHERE id = :id`

	_, err := r.db.NamedExec(query, incident)
	return err
}

func (r *Repository) GetGroupIncidents(groupID, tenantID string, limit int) ([]*MonitorGroupIncident, error) {
	incidents := []*MonitorGroupIncident{}
	query := `
		SELECT * FROM monitor_group_incidents
		WHERE group_id = $1 AND tenant_id = $2
		ORDER BY started_at DESC
		LIMIT $3`

	err := r.db.Select(&incidents, query, groupID, tenantID, limit)
	return incidents, err
}

// Monitor Group SLA Report operations

func (r *Repository) CreateGroupSLAReport(report *MonitorGroupSLAReport) error {
	query := `
		INSERT INTO monitor_group_sla_reports (
			id, group_id, tenant_id, period_start, period_end,
			health_score_average, uptime_percentage, downtime_minutes,
			incidents_count, slo_met, created_at
		) VALUES (
			:id, :group_id, :tenant_id, :period_start, :period_end,
			:health_score_average, :uptime_percentage, :downtime_minutes,
			:incidents_count, :slo_met, :created_at
		) ON CONFLICT (group_id, period_start, period_end) DO UPDATE SET
			health_score_average = :health_score_average,
			uptime_percentage = :uptime_percentage,
			downtime_minutes = :downtime_minutes,
			incidents_count = :incidents_count,
			slo_met = :slo_met`

	_, err := r.db.NamedExec(query, report)
	return err
}

func (r *Repository) GetGroupSLAReports(groupID string, limit int) ([]*MonitorGroupSLAReport, error) {
	reports := []*MonitorGroupSLAReport{}
	query := `
		SELECT * FROM monitor_group_sla_reports 
		WHERE group_id = $1 
		ORDER BY period_start DESC 
		LIMIT $2`

	err := r.db.Select(&reports, query, groupID, limit)
	return reports, err
}

// Helper function to get all group statuses for a tenant
func (r *Repository) GetAllGroupStatuses(tenantID string) (map[string]*MonitorGroupStatus, error) {
	statuses := make(map[string]*MonitorGroupStatus)
	query := `
		SELECT gs.* FROM monitor_group_status gs
		JOIN monitor_groups g ON gs.group_id = g.id
		WHERE g.tenant_id = $1`

	rows, err := r.db.Query(query, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var status MonitorGroupStatus
		err := rows.Scan(
			&status.GroupID, &status.OverallStatus, &status.HealthScore,
			&status.MonitorsUp, &status.MonitorsDown, &status.MonitorsDegraded,
			&status.CriticalMonitorsDown, &status.LastCheck, &status.Message,
		)
		if err != nil {
			return nil, err
		}
		statuses[status.GroupID] = &status
	}

	return statuses, nil
}

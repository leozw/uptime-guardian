-- Monitor Groups table
CREATE TABLE monitor_groups (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    enabled BOOLEAN NOT NULL DEFAULT true,
    tags JSONB NOT NULL DEFAULT '{}' :: jsonb,
    notification_config JSONB NOT NULL DEFAULT '{}' :: jsonb,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    created_by VARCHAR(255) NOT NULL,
    UNIQUE(tenant_id, name)
);

-- Create indexes for monitor_groups
CREATE INDEX idx_monitor_groups_tenant ON monitor_groups(tenant_id);
CREATE INDEX idx_monitor_groups_enabled ON monitor_groups(enabled);

-- Monitor Group Members table (many-to-many relationship)
CREATE TABLE monitor_group_members (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    group_id UUID NOT NULL REFERENCES monitor_groups(id) ON DELETE CASCADE,
    monitor_id UUID NOT NULL REFERENCES monitors(id) ON DELETE CASCADE,
    weight DECIMAL(3, 2) NOT NULL DEFAULT 1.0 CHECK (weight >= 0 AND weight <= 1),
    is_critical BOOLEAN NOT NULL DEFAULT false,
    added_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(group_id, monitor_id)
);

-- Create indexes for monitor_group_members
CREATE INDEX idx_monitor_group_members_group ON monitor_group_members(group_id);
CREATE INDEX idx_monitor_group_members_monitor ON monitor_group_members(monitor_id);

-- Monitor Group SLOs
CREATE TABLE monitor_group_slos (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    group_id UUID NOT NULL REFERENCES monitor_groups(id) ON DELETE CASCADE,
    tenant_id VARCHAR(255) NOT NULL,
    target_uptime_percentage DECIMAL(5, 2) NOT NULL DEFAULT 99.9,
    measurement_period_days INTEGER NOT NULL DEFAULT 30,
    calculation_method VARCHAR(50) NOT NULL DEFAULT 'weighted_average',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(group_id),
    CHECK (
        target_uptime_percentage >= 0
        AND target_uptime_percentage <= 100
    ),
    CHECK (measurement_period_days > 0),
    CHECK (calculation_method IN ('weighted_average', 'worst_case', 'critical_only'))
);

-- Monitor Group Status (for quick lookups)
CREATE TABLE monitor_group_status (
    group_id UUID PRIMARY KEY REFERENCES monitor_groups(id) ON DELETE CASCADE,
    overall_status VARCHAR(50) NOT NULL,
    health_score DECIMAL(5, 2) NOT NULL CHECK (health_score >= 0 AND health_score <= 100),
    monitors_up INTEGER NOT NULL DEFAULT 0,
    monitors_down INTEGER NOT NULL DEFAULT 0,
    monitors_degraded INTEGER NOT NULL DEFAULT 0,
    critical_monitors_down INTEGER NOT NULL DEFAULT 0,
    last_check TIMESTAMP NOT NULL,
    message TEXT
);

-- Monitor Group Alert Rules
CREATE TABLE monitor_group_alert_rules (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    group_id UUID NOT NULL REFERENCES monitor_groups(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    trigger_condition VARCHAR(50) NOT NULL,
    threshold_value DECIMAL(5, 2),
    notification_channels JSONB NOT NULL DEFAULT '[]' :: jsonb,
    cooldown_minutes INTEGER NOT NULL DEFAULT 5,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    CHECK (trigger_condition IN ('health_score_below', 'any_critical_down', 'percentage_down', 'all_down'))
);

-- Monitor Group Incidents
CREATE TABLE monitor_group_incidents (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    group_id UUID NOT NULL REFERENCES monitor_groups(id) ON DELETE CASCADE,
    tenant_id VARCHAR(255) NOT NULL,
    started_at TIMESTAMP NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMP,
    severity VARCHAR(50) NOT NULL DEFAULT 'critical',
    affected_monitors JSONB NOT NULL DEFAULT '[]' :: jsonb,
    root_cause_monitor_id UUID REFERENCES monitors(id),
    notifications_sent INTEGER NOT NULL DEFAULT 0,
    health_score_at_start DECIMAL(5, 2),
    acknowledged_at TIMESTAMP,
    acknowledged_by VARCHAR(255)
);

-- Create indexes for monitor_group_incidents
CREATE INDEX idx_monitor_group_incidents_group ON monitor_group_incidents(group_id);
CREATE INDEX idx_monitor_group_incidents_tenant ON monitor_group_incidents(tenant_id);
CREATE INDEX idx_monitor_group_incidents_active ON monitor_group_incidents(resolved_at)
WHERE
    resolved_at IS NULL;

-- SLA reports for groups
CREATE TABLE monitor_group_sla_reports (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    group_id UUID NOT NULL REFERENCES monitor_groups(id) ON DELETE CASCADE,
    tenant_id VARCHAR(255) NOT NULL,
    period_start TIMESTAMP NOT NULL,
    period_end TIMESTAMP NOT NULL,
    health_score_average DECIMAL(5, 2) NOT NULL,
    uptime_percentage DECIMAL(5, 2) NOT NULL,
    downtime_minutes INTEGER NOT NULL DEFAULT 0,
    incidents_count INTEGER NOT NULL DEFAULT 0,
    slo_met BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(group_id, period_start, period_end)
);

-- Create indexes for monitor_group_sla_reports
CREATE INDEX idx_monitor_group_sla_reports_group ON monitor_group_sla_reports(group_id);
CREATE INDEX idx_monitor_group_sla_reports_period ON monitor_group_sla_reports(period_start, period_end);
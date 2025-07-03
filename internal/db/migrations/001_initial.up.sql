-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Monitors table
CREATE TABLE monitors (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL CHECK (type IN ('http', 'ssl', 'dns', 'domain')),
    target TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    interval INTEGER NOT NULL CHECK (interval >= 30),
    timeout INTEGER NOT NULL DEFAULT 30 CHECK (
        timeout >= 1
        AND timeout <= 60
    ),
    regions JSONB NOT NULL DEFAULT '[]' :: jsonb,
    config JSONB NOT NULL DEFAULT '{}' :: jsonb,
    notification_config JSONB NOT NULL DEFAULT '{}' :: jsonb,
    tags JSONB NOT NULL DEFAULT '{}' :: jsonb,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    created_by VARCHAR(255) NOT NULL
);

-- Create indexes for monitors
CREATE INDEX idx_monitors_tenant ON monitors(tenant_id);

CREATE INDEX idx_monitors_enabled ON monitors(enabled);

CREATE INDEX idx_monitors_type ON monitors(type);

-- Check results table
CREATE TABLE check_results (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    monitor_id UUID NOT NULL REFERENCES monitors(id) ON DELETE CASCADE,
    tenant_id VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL CHECK (status IN ('up', 'down', 'degraded')),
    response_time_ms INTEGER,
    status_code INTEGER,
    error TEXT,
    details JSONB,
    region VARCHAR(50) NOT NULL,
    checked_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Create indexes for check_results
CREATE INDEX idx_check_results_monitor ON check_results(monitor_id);

CREATE INDEX idx_check_results_tenant ON check_results(tenant_id);

CREATE INDEX idx_check_results_checked_at ON check_results(checked_at DESC);

-- Monitor last status table (for quick lookups)
CREATE TABLE monitor_last_status (
    monitor_id UUID PRIMARY KEY REFERENCES monitors(id) ON DELETE CASCADE,
    status VARCHAR(50) NOT NULL,
    message TEXT,
    last_check TIMESTAMP NOT NULL,
    response_time_ms INTEGER,
    ssl_expiry_days INTEGER
);

-- Notification channels table
CREATE TABLE notification_channels (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL CHECK (type IN ('webhook', 'email', 'slack')),
    config JSONB NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Create index for notification_channels
CREATE INDEX idx_notification_channels_tenant ON notification_channels(tenant_id);

-- Incidents table
CREATE TABLE incidents (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    monitor_id UUID NOT NULL REFERENCES monitors(id) ON DELETE CASCADE,
    tenant_id VARCHAR(255) NOT NULL,
    started_at TIMESTAMP NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMP,
    severity VARCHAR(50) NOT NULL DEFAULT 'critical',
    notifications_sent INTEGER NOT NULL DEFAULT 0
);

-- Create indexes for incidents
CREATE INDEX idx_incidents_monitor ON incidents(monitor_id);

CREATE INDEX idx_incidents_tenant ON incidents(tenant_id);

CREATE INDEX idx_incidents_active ON incidents(resolved_at)
WHERE
    resolved_at IS NULL;

-- Scheduled checks table (for distributed processing)
CREATE TABLE scheduled_checks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    monitor_id UUID NOT NULL REFERENCES monitors(id) ON DELETE CASCADE,
    scheduled_for TIMESTAMP NOT NULL,
    picked_at TIMESTAMP,
    completed_at TIMESTAMP,
    worker_id VARCHAR(255)
);

-- Create index for scheduled_checks
CREATE INDEX idx_scheduled_checks_pending ON scheduled_checks(scheduled_for)
WHERE
    picked_at IS NULL;
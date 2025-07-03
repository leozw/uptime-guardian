-- SLO configurations per monitor
CREATE TABLE monitor_slos (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  monitor_id UUID NOT NULL REFERENCES monitors(id) ON DELETE CASCADE,
  tenant_id VARCHAR(255) NOT NULL,
  target_uptime_percentage DECIMAL(5, 2) NOT NULL DEFAULT 99.9,
  measurement_period_days INTEGER NOT NULL DEFAULT 30,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
  UNIQUE(monitor_id),
  CHECK (
    target_uptime_percentage >= 0
    AND target_uptime_percentage <= 100
  ),
  CHECK (measurement_period_days > 0)
);

-- SLA reports (monthly/period aggregations)
CREATE TABLE sla_reports (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  monitor_id UUID NOT NULL REFERENCES monitors(id) ON DELETE CASCADE,
  tenant_id VARCHAR(255) NOT NULL,
  period_start TIMESTAMP NOT NULL,
  period_end TIMESTAMP NOT NULL,
  total_checks INTEGER NOT NULL DEFAULT 0,
  successful_checks INTEGER NOT NULL DEFAULT 0,
  failed_checks INTEGER NOT NULL DEFAULT 0,
  uptime_percentage DECIMAL(5, 2) NOT NULL,
  downtime_minutes INTEGER NOT NULL DEFAULT 0,
  average_response_time_ms INTEGER,
  slo_met BOOLEAN NOT NULL DEFAULT false,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  UNIQUE(monitor_id, period_start, period_end)
);

-- Create indexes for sla_reports
CREATE INDEX idx_sla_reports_monitor ON sla_reports(monitor_id);

CREATE INDEX idx_sla_reports_period ON sla_reports(period_start, period_end);

-- Incident details enhancement
ALTER TABLE
  incidents
ADD
  COLUMN downtime_minutes INTEGER NOT NULL DEFAULT 0,
ADD
  COLUMN affected_checks INTEGER NOT NULL DEFAULT 0,
ADD
  COLUMN root_cause TEXT,
ADD
  COLUMN impact_description TEXT,
ADD
  COLUMN resolution_notes TEXT,
ADD
  COLUMN acknowledged_at TIMESTAMP,
ADD
  COLUMN acknowledged_by VARCHAR(255);

-- Incident timeline events
CREATE TABLE incident_events (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  incident_id UUID NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
  event_type VARCHAR(50) NOT NULL,
  event_time TIMESTAMP NOT NULL DEFAULT NOW(),
  description TEXT,
  created_by VARCHAR(255),
  metadata JSONB
);

-- Create indexes for incident_events
CREATE INDEX idx_incident_events_incident ON incident_events(incident_id);

CREATE INDEX idx_incident_events_time ON incident_events(event_time);
-- Domains table
CREATE TABLE domains (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    
    -- Auto-discovered fields
    ips TEXT[] DEFAULT '{}',
    has_ssl BOOLEAN DEFAULT false,
    mx_records TEXT[] DEFAULT '{}',
    nameservers TEXT[] DEFAULT '{}',
    
    -- Configuration
    check_interval INTERVAL DEFAULT '5 minutes',
    enabled BOOLEAN DEFAULT true,
    
    -- Health
    health_score INTEGER DEFAULT 0,
    last_check_at TIMESTAMP,
    
    -- Metadata
    labels JSONB DEFAULT '{}',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    UNIQUE(tenant_id, name)
);

-- Indexes
CREATE INDEX idx_domains_tenant_id ON domains(tenant_id);
CREATE INDEX idx_domains_enabled ON domains(enabled);
CREATE INDEX idx_domains_last_check ON domains(last_check_at);
CREATE INDEX idx_domains_health_score ON domains(health_score);
-- Enable extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Tenants table
CREATE TABLE tenants (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    api_key VARCHAR(255) UNIQUE NOT NULL,
    mimir_tenant_id VARCHAR(255) NOT NULL,
    max_domains INTEGER DEFAULT 10,
    check_interval_min INTEGER DEFAULT 5,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Tenant passwords (separate for security)
CREATE TABLE tenant_passwords (
    tenant_id UUID PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
    password_hash VARCHAR(255) NOT NULL
);

-- Indexes
CREATE INDEX idx_tenants_email ON tenants(email);
CREATE INDEX idx_tenants_api_key ON tenants(api_key);
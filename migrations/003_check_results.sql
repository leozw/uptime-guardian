-- Check results table
CREATE TABLE check_results (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    domain_id UUID NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
    check_type VARCHAR(50) NOT NULL,
    success BOOLEAN NOT NULL,
    response_time_ms FLOAT,
    details JSONB,
    error_message TEXT,
    checked_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Indexes
CREATE INDEX idx_check_results_domain_id ON check_results(domain_id);
CREATE INDEX idx_check_results_type ON check_results(check_type);
CREATE INDEX idx_check_results_checked_at ON check_results(checked_at DESC);

-- Composite index for latest results query
CREATE INDEX idx_check_results_latest ON check_results(domain_id, check_type, checked_at DESC);
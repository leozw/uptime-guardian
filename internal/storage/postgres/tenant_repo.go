package postgres

import (
	"github.com/leozw/uptime-guardian/internal/core"
)

func (db *DB) CreateTenant(tenant *core.Tenant, hashedPassword string) error {
	query := `
        INSERT INTO tenants (
            id, name, email, api_key, mimir_tenant_id,
            max_domains, check_interval_min, is_active,
            created_at, updated_at
        ) VALUES (
            $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
        )`

	_, err := db.Exec(query,
		tenant.ID, tenant.Name, tenant.Email, tenant.APIKey,
		tenant.MimirTenantID, tenant.MaxDomains, tenant.CheckIntervalMin,
		tenant.IsActive, tenant.CreatedAt, tenant.UpdatedAt,
	)

	if err != nil {
		return err
	}

	// Store password
	passwordQuery := `
        INSERT INTO tenant_passwords (tenant_id, password_hash)
        VALUES ($1, $2)
    `
	_, err = db.Exec(passwordQuery, tenant.ID, hashedPassword)

	return err
}

func (db *DB) GetTenant(id string) (*core.Tenant, error) {
	var tenant core.Tenant
	query := `
        SELECT id, name, email, api_key, mimir_tenant_id,
               max_domains, check_interval_min, is_active,
               created_at, updated_at
        FROM tenants
        WHERE id = $1
    `

	err := db.Get(&tenant, query, id)
	if err != nil {
		return nil, err
	}

	return &tenant, nil
}

func (db *DB) GetTenantByEmail(email string) (*core.Tenant, string, error) {
	var tenant core.Tenant
	var hashedPassword string

	query := `
        SELECT t.id, t.name, t.email, t.api_key, t.mimir_tenant_id,
               t.max_domains, t.check_interval_min, t.is_active,
               t.created_at, t.updated_at, tp.password_hash
        FROM tenants t
        JOIN tenant_passwords tp ON t.id = tp.tenant_id
        WHERE t.email = $1
    `

	row := db.QueryRow(query, email)
	err := row.Scan(
		&tenant.ID, &tenant.Name, &tenant.Email, &tenant.APIKey,
		&tenant.MimirTenantID, &tenant.MaxDomains, &tenant.CheckIntervalMin,
		&tenant.IsActive, &tenant.CreatedAt, &tenant.UpdatedAt,
		&hashedPassword,
	)

	if err != nil {
		return nil, "", err
	}

	return &tenant, hashedPassword, nil
}

func (db *DB) EmailExists(email string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM tenants WHERE email = $1)`
	err := db.Get(&exists, query, email)
	return exists, err
}

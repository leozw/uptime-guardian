package postgres

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"github.com/leozw/uptime-guardian/internal/checker"
	"github.com/leozw/uptime-guardian/internal/core"
)

func (db *DB) CreateDomain(domain *core.Domain) error {
	labelsJSON, _ := json.Marshal(domain.Labels)

	query := `
        INSERT INTO domains (
            id, tenant_id, name, check_interval, enabled,
            labels, created_at, updated_at
        ) VALUES (
            $1, $2, $3, $4, $5, $6, $7, $8
        )`

	_, err := db.Exec(query,
		domain.ID, domain.TenantID, domain.Name,
		domain.CheckInterval, domain.Enabled,
		labelsJSON, domain.CreatedAt, domain.UpdatedAt,
	)

	return err
}

func (db *DB) GetDomain(id, tenantID string) (*core.Domain, error) {
	var domain core.Domain
	var labelsJSON []byte
	var ips, mxRecords, nameservers pq.StringArray

	query := `
        SELECT id, tenant_id, name, ips, has_ssl, mx_records,
               nameservers, check_interval, enabled, health_score,
               last_check_at, labels, created_at, updated_at
        FROM domains
        WHERE id = $1 AND tenant_id = $2
    `

	err := db.QueryRow(query, id, tenantID).Scan(
		&domain.ID, &domain.TenantID, &domain.Name,
		&ips, &domain.HasSSL, &mxRecords,
		&nameservers, &domain.CheckInterval, &domain.Enabled,
		&domain.HealthScore, &domain.LastCheckAt,
		&labelsJSON, &domain.CreatedAt, &domain.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	domain.IPs = []string(ips)
	domain.MXRecords = []string(mxRecords)
	domain.Nameservers = []string(nameservers)

	if labelsJSON != nil {
		json.Unmarshal(labelsJSON, &domain.Labels)
	}

	return &domain, nil
}

func (db *DB) ListDomains(tenantID string) ([]*core.Domain, error) {
	query := `
        SELECT id, tenant_id, name, has_ssl, health_score,
               enabled, last_check_at, created_at, updated_at
        FROM domains
        WHERE tenant_id = $1
        ORDER BY name
    `

	rows, err := db.Query(query, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var domains []*core.Domain
	for rows.Next() {
		var domain core.Domain
		err := rows.Scan(
			&domain.ID, &domain.TenantID, &domain.Name,
			&domain.HasSSL, &domain.HealthScore,
			&domain.Enabled, &domain.LastCheckAt,
			&domain.CreatedAt, &domain.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		domains = append(domains, &domain)
	}

	return domains, nil
}

func (db *DB) UpdateDomain(domain *core.Domain) error {
	labelsJSON, _ := json.Marshal(domain.Labels)

	query := `
        UPDATE domains SET
            enabled = $1,
            labels = $2,
            updated_at = $3
        WHERE id = $4 AND tenant_id = $5
    `

	_, err := db.Exec(query,
		domain.Enabled, labelsJSON, domain.UpdatedAt,
		domain.ID, domain.TenantID,
	)

	return err
}

func (db *DB) DeleteDomain(id, tenantID string) error {
	query := `DELETE FROM domains WHERE id = $1 AND tenant_id = $2`
	_, err := db.Exec(query, id, tenantID)
	return err
}

func (db *DB) CountDomains(tenantID string) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM domains WHERE tenant_id = $1`
	err := db.Get(&count, query, tenantID)
	return count, err
}

func (db *DB) DomainExists(tenantID, domainName string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM domains WHERE tenant_id = $1 AND name = $2)`
	err := db.Get(&exists, query, tenantID, domainName)
	return exists, err
}

func (db *DB) GetDomainsToCheck() ([]*core.Domain, error) {
	query := `
        SELECT id, tenant_id, name, check_interval
        FROM domains
        WHERE enabled = true
        AND (last_check_at IS NULL OR last_check_at + check_interval < NOW())
        LIMIT 100
    `

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var domains []*core.Domain
	for rows.Next() {
		var domain core.Domain
		err := rows.Scan(
			&domain.ID, &domain.TenantID,
			&domain.Name, &domain.CheckInterval,
		)
		if err != nil {
			return nil, err
		}
		domains = append(domains, &domain)
	}

	return domains, nil
}

func (db *DB) SaveCheckResults(domainID uuid.UUID, analysis *checker.DomainAnalysis) error {
	tx, err := db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Save each check result
	for checkType, result := range analysis.Checks {
		query := `
            INSERT INTO check_results (
                id, domain_id, check_type, success,
                response_time_ms, details, error_message, checked_at
            ) VALUES (
                $1, $2, $3, $4, $5, $6, $7, $8
            )`

		_, err := tx.Exec(query,
			uuid.New(), domainID, checkType, result.Success,
			result.ResponseTime, result.Details, result.ErrorMessage,
			result.CheckedAt,
		)
		if err != nil {
			return err
		}
	}

	// Update domain info
	updateQuery := `
        UPDATE domains SET
            health_score = $1,
            last_check_at = $2,
            updated_at = $3
        WHERE id = $4
    `

	_, err = tx.Exec(updateQuery,
		analysis.HealthScore, time.Now(), time.Now(), domainID,
	)
	if err != nil {
		return err
	}

	// Auto-discover and update domain info if DNS check succeeded
	if dnsCheck, ok := analysis.Checks["dns"]; ok && dnsCheck.Success {
		var dnsDetails core.DNSCheckDetails
		if err := json.Unmarshal(dnsCheck.Details, &dnsDetails); err == nil {
			updateDNSQuery := `
                UPDATE domains SET
                    ips = $1,
                    mx_records = $2,
                    nameservers = $3
                WHERE id = $4
            `
			_, err = tx.Exec(updateDNSQuery,
				pq.Array(dnsDetails.ARecords),
				pq.Array(extractMXHosts(dnsDetails.MXRecords)),
				pq.Array(dnsDetails.NSRecords),
				domainID,
			)
		}
	}

	// Update SSL status
	if sslCheck, ok := analysis.Checks["ssl"]; ok {
		updateSSLQuery := `UPDATE domains SET has_ssl = $1 WHERE id = $2`
		_, err = tx.Exec(updateSSLQuery, sslCheck.Success, domainID)
	}

	return tx.Commit()
}

func (db *DB) GetLatestCheckResults(domainID string) (map[string]*core.CheckResult, error) {
	query := `
        SELECT DISTINCT ON (check_type)
            id, domain_id, check_type, success,
            response_time_ms, details, error_message, checked_at
        FROM check_results
        WHERE domain_id = $1
        ORDER BY check_type, checked_at DESC
    `

	rows, err := db.Query(query, domainID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make(map[string]*core.CheckResult)
	for rows.Next() {
		var result core.CheckResult
		err := rows.Scan(
			&result.ID, &result.DomainID, &result.CheckType,
			&result.Success, &result.ResponseTime, &result.Details,
			&result.ErrorMessage, &result.CheckedAt,
		)
		if err != nil {
			return nil, err
		}
		results[result.CheckType] = &result
	}

	return results, nil
}

func extractMXHosts(mxRecords []core.MXRecord) []string {
	hosts := make([]string, len(mxRecords))
	for i, mx := range mxRecords {
		hosts[i] = mx.Host
	}
	return hosts
}

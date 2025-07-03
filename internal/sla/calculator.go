package sla

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/leozw/uptime-guardian/internal/db"
	"go.uber.org/zap"
)

type Calculator struct {
	repo   *db.Repository
	logger *zap.Logger
}

func NewCalculator(repo *db.Repository, logger *zap.Logger) *Calculator {
	return &Calculator{
		repo:   repo,
		logger: logger,
	}
}

// CalculateSLA calcula o SLA para um monitor em um período
func (c *Calculator) CalculateSLA(monitorID string, periodStart, periodEnd time.Time) (*db.SLAReport, error) {
	monitor, err := c.repo.GetMonitorByID(monitorID)
	if err != nil {
		return nil, fmt.Errorf("failed to get monitor: %w", err)
	}

	// Buscar todos os checks no período
	checks, err := c.repo.GetCheckResultsInPeriod(monitorID, periodStart, periodEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to get check results: %w", err)
	}

	if len(checks) == 0 {
		return nil, fmt.Errorf("no checks found in period")
	}

	// Calcular estatísticas
	totalChecks := len(checks)
	successfulChecks := 0
	failedChecks := 0
	totalResponseTime := int64(0)

	for _, check := range checks {
		if check.Status == db.StatusUp {
			successfulChecks++
		} else {
			failedChecks++
		}
		totalResponseTime += int64(check.ResponseTimeMs)
	}

	// Calcular uptime percentage
	uptimePercentage := (float64(successfulChecks) / float64(totalChecks)) * 100

	// Calcular downtime em minutos
	downtimeMinutes := c.calculateDowntimeMinutes(checks, monitor.Interval)

	// Calcular tempo médio de resposta
	avgResponseTime := int(totalResponseTime / int64(totalChecks))

	// Verificar se o SLO foi cumprido
	slo, _ := c.repo.GetMonitorSLO(monitorID)
	sloMet := true
	if slo != nil {
		sloMet = uptimePercentage >= slo.TargetUptimePercentage
	}

	report := &db.SLAReport{
		ID:                    uuid.New().String(),
		MonitorID:             monitorID,
		TenantID:              monitor.TenantID,
		PeriodStart:           periodStart,
		PeriodEnd:             periodEnd,
		TotalChecks:           totalChecks,
		SuccessfulChecks:      successfulChecks,
		FailedChecks:          failedChecks,
		UptimePercentage:      uptimePercentage,
		DowntimeMinutes:       downtimeMinutes,
		AverageResponseTimeMs: &avgResponseTime,
		SLOMet:                sloMet,
		CreatedAt:             time.Now(),
	}

	return report, nil
}

// calculateDowntimeMinutes calcula o tempo total de downtime baseado nos checks
func (c *Calculator) calculateDowntimeMinutes(checks []*db.CheckResult, intervalSeconds int) int {
	if len(checks) == 0 {
		return 0
	}

	downtimeMinutes := 0
	inDowntime := false
	var downtimeStart time.Time

	for _, check := range checks { // Removido o 'i' não utilizado
		if check.Status != db.StatusUp && !inDowntime {
			inDowntime = true
			downtimeStart = check.CheckedAt
		} else if check.Status == db.StatusUp && inDowntime {
			inDowntime = false
			downtimeMinutes += int(check.CheckedAt.Sub(downtimeStart).Minutes())
		}
	}

	// Se ainda está em downtime no final do período
	if inDowntime && len(checks) > 0 {
		lastCheck := checks[len(checks)-1]
		downtimeMinutes += int(time.Since(lastCheck.CheckedAt).Minutes())
	}

	return downtimeMinutes
}

// GenerateMonthlyReport gera o relatório mensal de SLA
func (c *Calculator) GenerateMonthlyReport(monitorID string, year int, month time.Month) (*db.SLAReport, error) {
	startOfMonth := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	endOfMonth := startOfMonth.AddDate(0, 1, 0).Add(-time.Second)

	report, err := c.CalculateSLA(monitorID, startOfMonth, endOfMonth)
	if err != nil {
		return nil, err
	}

	// Salvar o relatório
	if err := c.repo.CreateSLAReport(report); err != nil {
		return nil, fmt.Errorf("failed to save SLA report: %w", err)
	}

	return report, nil
}

// GetCurrentMonthSLA calcula o SLA do mês atual até o momento
func (c *Calculator) GetCurrentMonthSLA(monitorID string) (*db.SLAReport, error) {
	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	return c.CalculateSLA(monitorID, startOfMonth, now)
}

// GetSLAHistory retorna o histórico de SLA reports
func (c *Calculator) GetSLAHistory(monitorID string, limit int) ([]*db.SLAReport, error) {
	return c.repo.GetSLAReports(monitorID, limit)
}

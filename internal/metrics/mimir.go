package metrics

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/prompb"

	"github.com/leozw/uptime-guardian/internal/checker"
	"github.com/leozw/uptime-guardian/internal/core"
)

type MimirClient struct {
	url    string
	client *http.Client
}

func NewMimirClient(url string) *MimirClient {
	return &MimirClient{
		url: url,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (m *MimirClient) SendDomainMetrics(tenantID, domain string, analysis *checker.DomainAnalysis) error {
	var timeseries []prompb.TimeSeries
	now := time.Now().UnixNano() / int64(time.Millisecond)

	// Health score
	timeseries = append(timeseries, prompb.TimeSeries{
		Labels: []prompb.Label{
			{Name: "__name__", Value: "domain_health_score"},
			{Name: "domain", Value: domain},
			{Name: "tenant_id", Value: tenantID},
		},
		Samples: []prompb.Sample{
			{Value: float64(analysis.HealthScore), Timestamp: now},
		},
	})

	// Process check results
	for checkType, result := range analysis.Checks {
		// Check success
		successValue := 0.0
		if result.Success {
			successValue = 1.0
		}

		timeseries = append(timeseries, prompb.TimeSeries{
			Labels: []prompb.Label{
				{Name: "__name__", Value: "domain_check_success"},
				{Name: "domain", Value: domain},
				{Name: "tenant_id", Value: tenantID},
				{Name: "check_type", Value: checkType},
			},
			Samples: []prompb.Sample{
				{Value: successValue, Timestamp: now},
			},
		})

		// Response time
		timeseries = append(timeseries, prompb.TimeSeries{
			Labels: []prompb.Label{
				{Name: "__name__", Value: "domain_check_duration_seconds"},
				{Name: "domain", Value: domain},
				{Name: "tenant_id", Value: tenantID},
				{Name: "check_type", Value: checkType},
			},
			Samples: []prompb.Sample{
				{Value: result.ResponseTime / 1000, Timestamp: now},
			},
		})

		// Type-specific metrics
		switch checkType {
		case "ssl":
			if result.Success && result.Details != nil {
				var details core.SSLCheckDetails
				if err := json.Unmarshal(result.Details, &details); err == nil {
					timeseries = append(timeseries, prompb.TimeSeries{
						Labels: []prompb.Label{
							{Name: "__name__", Value: "domain_ssl_days_remaining"},
							{Name: "domain", Value: domain},
							{Name: "tenant_id", Value: tenantID},
						},
						Samples: []prompb.Sample{
							{Value: float64(details.DaysToExpiry), Timestamp: now},
						},
					})
				}
			}

		case "whois":
			if result.Success && result.Details != nil {
				var details core.WHOISCheckDetails
				if err := json.Unmarshal(result.Details, &details); err == nil {
					timeseries = append(timeseries, prompb.TimeSeries{
						Labels: []prompb.Label{
							{Name: "__name__", Value: "domain_expiration_days"},
							{Name: "domain", Value: domain},
							{Name: "tenant_id", Value: tenantID},
						},
						Samples: []prompb.Sample{
							{Value: float64(details.DaysToExpiry), Timestamp: now},
						},
					})
				}
			}

		case "dns":
			if result.Success && result.Details != nil {
				var details core.DNSCheckDetails
				if err := json.Unmarshal(result.Details, &details); err == nil {
					// A records count
					timeseries = append(timeseries, prompb.TimeSeries{
						Labels: []prompb.Label{
							{Name: "__name__", Value: "domain_dns_record_count"},
							{Name: "domain", Value: domain},
							{Name: "tenant_id", Value: tenantID},
							{Name: "record_type", Value: "A"},
						},
						Samples: []prompb.Sample{
							{Value: float64(len(details.ARecords)), Timestamp: now},
						},
					})
				}
			}
		}
	}

	return m.remoteWrite(tenantID, timeseries)
}

func (m *MimirClient) remoteWrite(tenantID string, timeseries []prompb.TimeSeries) error {
	req := &prompb.WriteRequest{
		Timeseries: timeseries,
	}

	data, err := req.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	compressed := snappy.Encode(nil, data)

	httpReq, err := http.NewRequest("POST", m.url+"/api/v1/push", bytes.NewReader(compressed))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/x-protobuf")
	httpReq.Header.Set("Content-Encoding", "snappy")
	httpReq.Header.Set("X-Prometheus-Remote-Write-Version", "0.1.0")
	httpReq.Header.Set("X-Scope-OrgID", tenantID)

	resp, err := m.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("remote write failed with status %d", resp.StatusCode)
	}

	return nil
}

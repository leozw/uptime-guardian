package metrics

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/golang/snappy"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/prometheus/prompb"
)

func (c *Collector) StartRemoteWrite(ctx context.Context) {
	ticker := time.NewTicker(c.config.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.writeToMimir()
		}
	}
}

func (c *Collector) writeToMimir() error {
	// Gather metrics
	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		return fmt.Errorf("failed to gather metrics: %w", err)
	}

	// Convert to remote write format
	samples := c.metricsToSamples(mfs)
	if len(samples) == 0 {
		return nil
	}

	// Batch samples
	for i := 0; i < len(samples); i += c.config.BatchSize {
		end := i + c.config.BatchSize
		if end > len(samples) {
			end = len(samples)
		}

		batch := samples[i:end]
		if err := c.sendBatch(batch); err != nil {
			return fmt.Errorf("failed to send batch: %w", err)
		}
	}

	return nil
}

func (c *Collector) metricsToSamples(mfs []*dto.MetricFamily) []prompb.TimeSeries {
	var samples []prompb.TimeSeries

	for _, mf := range mfs {
		for _, m := range mf.Metric {
			// Extract tenant_id from labels
			var tenantID string
			labels := make([]prompb.Label, 0, len(m.Label)+1)

			for _, l := range m.Label {
				if l.GetName() == "tenant_id" {
					tenantID = l.GetValue()
				}
				labels = append(labels, prompb.Label{
					Name:  l.GetName(),
					Value: l.GetValue(),
				})
			}

			if tenantID == "" {
				continue // Skip metrics without tenant_id
			}

			// Add metric name
			labels = append(labels, prompb.Label{
				Name:  "__name__",
				Value: mf.GetName(),
			})

			// Get value
			var value float64
			switch mf.GetType() {
			case dto.MetricType_COUNTER:
				value = m.Counter.GetValue()
			case dto.MetricType_GAUGE:
				value = m.Gauge.GetValue()
			case dto.MetricType_HISTOGRAM:
				// For histograms, we need to handle buckets
				hist := m.Histogram
				for i, bucket := range hist.Bucket {
					bucketLabels := append([]prompb.Label{}, labels...)
					bucketLabels = append(bucketLabels, prompb.Label{
						Name:  "le",
						Value: fmt.Sprintf("%g", bucket.GetUpperBound()),
					})

					samples = append(samples, prompb.TimeSeries{
						Labels: bucketLabels,
						Samples: []prompb.Sample{{
							Value:     float64(bucket.GetCumulativeCount()),
							Timestamp: time.Now().UnixNano() / 1e6,
						}},
					})
				}
				continue
			default:
				continue
			}

			samples = append(samples, prompb.TimeSeries{
				Labels: labels,
				Samples: []prompb.Sample{{
					Value:     value,
					Timestamp: time.Now().UnixNano() / 1e6,
				}},
			})
		}
	}

	return samples
}

func (c *Collector) sendBatch(samples []prompb.TimeSeries) error {
	// Group by tenant
	byTenant := make(map[string][]prompb.TimeSeries)
	for _, ts := range samples {
		var tenantID string
		for _, label := range ts.Labels {
			if label.Name == "tenant_id" {
				tenantID = label.Value
				break
			}
		}
		if tenantID != "" {
			byTenant[tenantID] = append(byTenant[tenantID], ts)
		}
	}

	// Send per tenant
	for tenantID, tenantSamples := range byTenant {
		req := &prompb.WriteRequest{
			Timeseries: tenantSamples,
		}

		data, err := req.Marshal()
		if err != nil {
			return err
		}

		compressed := snappy.Encode(nil, data)

		httpReq, err := http.NewRequest("POST", c.config.URL+"/api/v1/push", bytes.NewReader(compressed))
		if err != nil {
			return err
		}

		httpReq.Header.Set("Content-Type", "application/x-protobuf")
		httpReq.Header.Set("Content-Encoding", "snappy")
		httpReq.Header.Set(c.config.TenantHeader, tenantID)

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(httpReq)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("remote write failed: %s", resp.Status)
		}
	}

	return nil
}

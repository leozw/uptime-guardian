package metrics

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/golang/snappy"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/prometheus/prompb"
)

func (c *Collector) StartRemoteWrite(ctx context.Context) {
	log.Printf("Starting remote write to Mimir: %s", c.config.URL)
	ticker := time.NewTicker(c.config.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := c.writeToMimir(); err != nil {
				log.Printf("Error writing to Mimir: %v", err)
			}
		}
	}
}

func (c *Collector) writeToMimir() error {
	log.Println("Gathering metrics for Mimir...")

	// Gather metrics
	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		return fmt.Errorf("failed to gather metrics: %w", err)
	}

	// Convert to remote write format
	samples := c.metricsToSamples(mfs)
	log.Printf("Found %d samples to send to Mimir", len(samples))

	if len(samples) == 0 {
		log.Println("No samples to send")
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

	log.Println("Successfully sent metrics to Mimir")
	return nil
}

func (c *Collector) metricsToSamples(mfs []*dto.MetricFamily) []prompb.TimeSeries {
	var samples []prompb.TimeSeries

	for _, mf := range mfs {
		// Skip non-uptime metrics - updated to include domain_ prefix
		if !strings.HasPrefix(mf.GetName(), "uptime_") &&
			!strings.HasPrefix(mf.GetName(), "ssl_") &&
			!strings.HasPrefix(mf.GetName(), "dns_") &&
			!strings.HasPrefix(mf.GetName(), "domain_") {
			continue
		}

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
				for _, bucket := range hist.Bucket {
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

				// Add +Inf bucket
				bucketLabels := append([]prompb.Label{}, labels...)
				bucketLabels = append(bucketLabels, prompb.Label{
					Name:  "le",
					Value: "+Inf",
				})
				samples = append(samples, prompb.TimeSeries{
					Labels: bucketLabels,
					Samples: []prompb.Sample{{
						Value:     float64(hist.GetSampleCount()),
						Timestamp: time.Now().UnixNano() / 1e6,
					}},
				})

				// Add sum
				sumLabels := append([]prompb.Label{}, labels...)
				sumLabels[len(sumLabels)-1] = prompb.Label{
					Name:  "__name__",
					Value: mf.GetName() + "_sum",
				}
				samples = append(samples, prompb.TimeSeries{
					Labels: sumLabels,
					Samples: []prompb.Sample{{
						Value:     hist.GetSampleSum(),
						Timestamp: time.Now().UnixNano() / 1e6,
					}},
				})

				// Add count
				countLabels := append([]prompb.Label{}, labels...)
				countLabels[len(countLabels)-1] = prompb.Label{
					Name:  "__name__",
					Value: mf.GetName() + "_count",
				}
				samples = append(samples, prompb.TimeSeries{
					Labels: countLabels,
					Samples: []prompb.Sample{{
						Value:     float64(hist.GetSampleCount()),
						Timestamp: time.Now().UnixNano() / 1e6,
					}},
				})

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

	log.Printf("Sending metrics for %d tenants", len(byTenant))

	// Send per tenant
	for tenantID, tenantSamples := range byTenant {
		log.Printf("Sending %d samples for tenant: %s", len(tenantSamples), tenantID)

		req := &prompb.WriteRequest{
			Timeseries: tenantSamples,
		}

		data, err := req.Marshal()
		if err != nil {
			return err
		}

		compressed := snappy.Encode(nil, data)

		url := c.config.URL + "/api/v1/push"
		log.Printf("Sending to URL: %s with X-Scope-OrgID: %s", url, tenantID)

		httpReq, err := http.NewRequest("POST", url, bytes.NewReader(compressed))
		if err != nil {
			return err
		}

		httpReq.Header.Set("Content-Type", "application/x-protobuf")
		httpReq.Header.Set("Content-Encoding", "snappy")
		httpReq.Header.Set(c.config.TenantHeader, tenantID)

		// Adicione o token de autorização do Kong
		if c.config.AuthToken != "" {
			httpReq.Header.Set("Authorization", "Bearer "+c.config.AuthToken)
			log.Printf("Added Authorization header")
		}

		// Log all headers (mas esconda o token)
		headers := make(map[string]string)
		for k, v := range httpReq.Header {
			if k == "Authorization" {
				headers[k] = "Bearer ***"
			} else {
				headers[k] = strings.Join(v, ", ")
			}
		}
		log.Printf("Request headers: %v", headers)

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(httpReq)
		if err != nil {
			log.Printf("Failed to send request: %v", err)
			return err
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		log.Printf("Response status: %d, body: %s", resp.StatusCode, string(body))

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
			return fmt.Errorf("remote write failed: %s - %s", resp.Status, string(body))
		}
	}

	return nil
}

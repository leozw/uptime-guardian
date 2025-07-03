package checks

import (
	"fmt"
	"strings"
	"time"

	"github.com/leozw/uptime-guardian/internal/db"
	"github.com/miekg/dns"
)

type DNSChecker struct{}

func NewDNSChecker() *DNSChecker {
	return &DNSChecker{}
}

func (d *DNSChecker) Check(monitor *db.Monitor, region string) *db.CheckResult {
	result := &db.CheckResult{
		MonitorID: monitor.ID,
		TenantID:  monitor.TenantID,
		Region:    region,
		Details:   make(db.JSONB),
	}

	recordType := monitor.Config.RecordType
	if recordType == "" {
		recordType = "A"
	}

	// Create DNS client
	c := new(dns.Client)
	c.Timeout = time.Duration(monitor.Timeout) * time.Second

	// Create query
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(monitor.Target), dnsStringToType(recordType))

	// Query DNS
	start := time.Now()
	r, _, err := c.Exchange(m, "8.8.8.8:53") // Using Google DNS
	duration := time.Since(start)

	result.ResponseTimeMs = int(duration.Milliseconds())

	if err != nil {
		result.Status = db.StatusDown
		result.Error = fmt.Sprintf("DNS query failed: %v", err)
		return result
	}

	if r.Rcode != dns.RcodeSuccess {
		result.Status = db.StatusDown
		result.Error = fmt.Sprintf("DNS query failed with code: %s", dns.RcodeToString[r.Rcode])
		return result
	}

	// Extract answers
	var answers []string
	for _, ans := range r.Answer {
		switch recordType {
		case "A":
			if a, ok := ans.(*dns.A); ok {
				answers = append(answers, a.A.String())
			}
		case "AAAA":
			if aaaa, ok := ans.(*dns.AAAA); ok {
				answers = append(answers, aaaa.AAAA.String())
			}
		case "CNAME":
			if cname, ok := ans.(*dns.CNAME); ok {
				answers = append(answers, cname.Target)
			}
		case "MX":
			if mx, ok := ans.(*dns.MX); ok {
				answers = append(answers, fmt.Sprintf("%d %s", mx.Preference, mx.Mx))
			}
		case "TXT":
			if txt, ok := ans.(*dns.TXT); ok {
				answers = append(answers, strings.Join(txt.Txt, " "))
			}
		}
	}

	result.Details["answers"] = answers
	result.Details["record_count"] = len(answers)

	if len(answers) == 0 {
		result.Status = db.StatusDown
		result.Error = fmt.Sprintf("No %s records found", recordType)
		return result
	}

	// Check expected values if configured
	if len(monitor.Config.ExpectedValues) > 0 {
		found := false
		for _, expected := range monitor.Config.ExpectedValues {
			for _, answer := range answers {
				if strings.Contains(answer, expected) {
					found = true
					break
				}
			}
			if found {
				break
			}
		}

		if !found {
			result.Status = db.StatusDown
			result.Error = "Expected DNS values not found"
			return result
		}
	}

	result.Status = db.StatusUp
	return result
}

func dnsStringToType(recordType string) uint16 {
	switch recordType {
	case "A":
		return dns.TypeA
	case "AAAA":
		return dns.TypeAAAA
	case "CNAME":
		return dns.TypeCNAME
	case "MX":
		return dns.TypeMX
	case "TXT":
		return dns.TypeTXT
	case "NS":
		return dns.TypeNS
	default:
		return dns.TypeA
	}
}

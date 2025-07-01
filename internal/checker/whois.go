package checker

import (
	"fmt"
	"strings"
	"time"

	"github.com/likexian/whois"
	whoisparser "github.com/likexian/whois-parser"

	"github.com/leozw/uptime-guardian/internal/core"
)

type WHOISChecker struct{}

func NewWHOISChecker() *WHOISChecker {
	return &WHOISChecker{}
}

func (w *WHOISChecker) Check(domain string) (*core.WHOISCheckDetails, error) {
	// Get WHOIS data
	raw, err := whois.Whois(domain)
	if err != nil {
		return nil, fmt.Errorf("whois lookup failed: %w", err)
	}

	// Parse WHOIS data
	result, err := whoisparser.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("whois parse failed: %w", err)
	}

	details := &core.WHOISCheckDetails{
		Registrar: result.Registrar.Name,
		Status:    strings.Split(result.Domain.Status, ","),
	}

	// Parse dates
	if result.Domain.CreatedDate != "" {
		if t, err := parseWhoisDate(result.Domain.CreatedDate); err == nil {
			details.CreatedDate = &t
		}
	}

	if result.Domain.UpdatedDate != "" {
		if t, err := parseWhoisDate(result.Domain.UpdatedDate); err == nil {
			details.UpdatedDate = &t
		}
	}

	if result.Domain.ExpirationDate != "" {
		if t, err := parseWhoisDate(result.Domain.ExpirationDate); err == nil {
			details.DomainExpiry = &t
			details.DaysToExpiry = int(time.Until(t).Hours() / 24)
		}
	}

	// Check if domain is expiring soon
	if details.DaysToExpiry > 0 && details.DaysToExpiry < 60 {
		return details, fmt.Errorf("domain expiring in %d days", details.DaysToExpiry)
	}

	return details, nil
}

func parseWhoisDate(dateStr string) (time.Time, error) {
	// Try common WHOIS date formats
	formats := []string{
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
		"02-Jan-2006",
		"2006.01.02 15:04:05",
		"2006/01/02",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse date: %s", dateStr)
}

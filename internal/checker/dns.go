package checker

import (
    "context"
    "fmt"
    "net"
    "strings"
    "time"

    "github.com/miekg/dns"
    "github.com/leozw/uptime-guardian/internal/core"
)

type DNSChecker struct {
    resolver *net.Resolver
    client   *dns.Client
}

func NewDNSChecker() *DNSChecker {
    return &DNSChecker{
        resolver: &net.Resolver{
            PreferGo: true,
            Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
                d := net.Dialer{Timeout: 5 * time.Second}
                return d.DialContext(ctx, network, address)
            },
        },
        client: &dns.Client{Timeout: 5 * time.Second},
    }
}

func (d *DNSChecker) Check(domain string) (*core.DNSCheckDetails, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    details := &core.DNSCheckDetails{
        ARecords:    []string{},
        AAAARecords: []string{},
        MXRecords:   []core.MXRecord{},
        TXTRecords:  []string{},
        NSRecords:   []string{},
    }

    startTime := time.Now()

    // A Records
    ips, err := d.resolver.LookupHost(ctx, domain)
    if err == nil {
        for _, ip := range ips {
            if strings.Contains(ip, ":") {
                details.AAAARecords = append(details.AAAARecords, ip)
            } else {
                details.ARecords = append(details.ARecords, ip)
            }
        }
    }

    // MX Records
    mxRecords, err := d.resolver.LookupMX(ctx, domain)
    if err == nil {
        for _, mx := range mxRecords {
            details.MXRecords = append(details.MXRecords, core.MXRecord{
                Priority: int(mx.Pref),
                Host:     strings.TrimSuffix(mx.Host, "."),
            })
        }
    }

    // TXT Records
    txtRecords, err := d.resolver.LookupTXT(ctx, domain)
    if err == nil {
        details.TXTRecords = txtRecords
    }

    // NS Records
    nsRecords, err := d.resolver.LookupNS(ctx, domain)
    if err == nil {
        for _, ns := range nsRecords {
            details.NSRecords = append(details.NSRecords, strings.TrimSuffix(ns.Host, "."))
        }
    }

    // CNAME Record
    cname, err := d.resolver.LookupCNAME(ctx, domain)
    if err == nil && cname != domain+"." {
        cnameStr := strings.TrimSuffix(cname, ".")
        details.CNAMERecord = &cnameStr
    }

    // Check DNSSEC
    details.HasDNSSEC = d.checkDNSSEC(domain)

    details.ResponseTime = float64(time.Since(startTime).Milliseconds())

    // Validate we got at least some records
    if len(details.ARecords) == 0 && len(details.AAAARecords) == 0 && details.CNAMERecord == nil {
        return details, fmt.Errorf("no A, AAAA or CNAME records found")
    }

    return details, nil
}

func (d *DNSChecker) checkDNSSEC(domain string) bool {
    m := new(dns.Msg)
    m.SetQuestion(dns.Fqdn(domain), dns.TypeDNSKEY)
    m.SetEdns0(4096, true)

    r, _, err := d.client.Exchange(m, "8.8.8.8:53")
    if err != nil || r == nil {
        return false
    }

    // Check if AD (Authenticated Data) flag is set
    return r.AuthenticatedData
}
package deploy

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"regexp"
	"sort"
	"strings"
	"time"
)

var domainLabelRE = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)

func validateHostname(hostname string) error {
	h := strings.ToLower(strings.TrimSpace(strings.TrimSuffix(hostname, ".")))
	if h == "" || len(h) > 253 || strings.ContainsAny(h, "/:@") {
		return fmt.Errorf("%w: invalid hostname", ErrInvalidState)
	}
	labels := strings.Split(h, ".")
	if len(labels) < 2 {
		return fmt.Errorf("%w: hostname must include a public suffix", ErrInvalidState)
	}
	for _, label := range labels {
		if !domainLabelRE.MatchString(label) {
			return fmt.Errorf("%w: invalid hostname label %q", ErrInvalidState, label)
		}
	}
	return nil
}

func normalizeHostname(hostname string) string {
	return strings.ToLower(strings.TrimSpace(strings.TrimSuffix(hostname, ".")))
}

func normalizeSubdomain(input, fallback string) string {
	base := strings.ToLower(strings.TrimSpace(input))
	if base == "" {
		base = fallback
	}
	base = strings.Trim(base, ".")
	base = strings.ReplaceAll(base, "_", "-")
	var b strings.Builder
	lastDash := false
	for _, r := range base {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if ok {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 48 {
		out = strings.Trim(out[:48], "-")
	}
	if out == "" || !domainLabelRE.MatchString(out) {
		sum := sha256.Sum256([]byte(fallback + "|" + time.Now().UTC().Format(time.RFC3339Nano)))
		out = "app-" + hex.EncodeToString(sum[:4])
	}
	return out
}

func managedHostname(subdomain, baseDomain string) string {
	base := normalizeHostname(baseDomain)
	if base == "" {
		base = "ironflyer.app"
	}
	return normalizeHostname(subdomain + "." + base)
}

func sameApex(a, b string) bool {
	a = normalizeHostname(a)
	b = normalizeHostname(b)
	return a == b || strings.HasSuffix(a, "."+b) || strings.HasSuffix(b, "."+a)
}

func mergeDomainMetadata(a, b map[string]any) map[string]any {
	out := copyMapAny(a)
	if out == nil {
		out = map[string]any{}
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}

func sortedDNSRecords(in []DNSRecord) []DNSRecord {
	out := append([]DNSRecord(nil), in...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Type != out[j].Type {
			return out[i].Type < out[j].Type
		}
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].Value < out[j].Value
	})
	return out
}

func domainIsApex(hostname string) bool {
	hostname = normalizeHostname(hostname)
	return strings.Count(hostname, ".") == 1
}

func dnsTargetFor(hostname, providerTarget string) []DNSRecord {
	target := normalizeHostname(providerTarget)
	if target == "" {
		target = "edge.ironflyer.app"
	}
	if ip := net.ParseIP(target); ip != nil {
		typ := "A"
		if ip.To4() == nil {
			typ = "AAAA"
		}
		return []DNSRecord{{Type: typ, Name: hostname, Value: target, TTL: 300}}
	}
	if domainIsApex(hostname) {
		return []DNSRecord{
			{Type: "ALIAS", Name: "@", Value: target, TTL: 300},
			{Type: "TXT", Name: "_ironflyer." + hostname, Value: verificationTXT(hostname), TTL: 300},
		}
	}
	return []DNSRecord{
		{Type: "CNAME", Name: hostname, Value: target, TTL: 300},
		{Type: "TXT", Name: "_ironflyer." + hostname, Value: verificationTXT(hostname), TTL: 300},
	}
}

func verificationTXT(hostname string) string {
	sum := sha256.Sum256([]byte("ironflyer-domain|" + normalizeHostname(hostname)))
	return "ironflyer-verify=" + hex.EncodeToString(sum[:12])
}

func domainEventPayload(d Domain) map[string]any {
	return map[string]any{
		"domain_id":           d.ID,
		"hostname":            d.Hostname,
		"kind":                string(d.Kind),
		"status":              string(d.Status),
		"verification_status": d.VerificationStatus,
		"certificate_status":  string(d.CertificateStatus),
		"primary":             d.Primary,
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

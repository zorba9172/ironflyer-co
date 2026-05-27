// Package finisher — CompliancePCIGate.
//
// Enforces PCI-DSS v4 cardholder-data controls against the project
// workspace. Only fires when Project.Spec.Compliance contains "pci".
//
// Controls covered:
//
//   - PCI-DSS 3.4  No storage of raw PAN. We Luhn-validate any
//     13-16 digit run in source / migrations / logs to suppress
//     false positives (timestamps, IDs).
//   - PCI-DSS 4.2  TLS required on payment routes.
//   - PCI-DSS 6.5  Webhook signature verification at every
//     Stripe/Paddle payment handler.
//   - PCI-DSS 10.5 No card data in logs (no log line includes
//     `card`, `pan`, `cvv`, `cvc`).
//
// Severity ladder:
//   - SeverityCritical on raw PAN found (audit ship-stopper).
//   - SeverityError on missing TLS / plaintext card columns.
//   - SeverityWarning on missing webhook signature check.
//
// Repair agent is the Security agent.
package finisher

import (
	"context"
	"regexp"
	"strings"

	"ironflyer/core/orchestrator/internal/ai/agents"
	"ironflyer/core/orchestrator/internal/ai/domain"
)

type CompliancePCIGate struct{}

func (CompliancePCIGate) Name() domain.GateName    { return domain.GateCompliancePCI }
func (CompliancePCIGate) RepairAgent() agents.Role { return agents.RoleSecurity }

// pciDigitRunRe finds runs of 13-16 digits. We then Luhn-check each
// hit so timestamp-like sequences and execution IDs do not trip the
// gate.
var pciDigitRunRe = regexp.MustCompile(`(?:\d[ -]?){13,19}`)

// pciCardColumnRe matches `card`/`pan`/`cvv`/`cvc` columns declared as
// TEXT/VARCHAR in SQL migrations — plaintext storage of cardholder data
// is a PCI ship-stopper.
var pciCardColumnRe = regexp.MustCompile(`(?i)\b(pan|card_number|cardnumber|cvv|cvc|card_pan)\s+(?:varchar|text|char)\b`)

// pciLogCardRe matches log lines that mention raw card terms. Anything
// of the form log/println/print/fmt.* containing "card" / "pan" / "cvv"
// is flagged so the operator can scrub it.
var pciLogCardRe = regexp.MustCompile(`(?i)(?:log\.|logger\.|fmt\.Print|println|console\.log)[^\n]{0,160}\b(card_number|cardnumber|\bcvv\b|\bcvc\b|\bpan\b)\b`)

// pciWebhookHandlerRe matches the canonical Stripe/Paddle webhook
// handler signatures in Go and TS. We need at least one of these in
// the project to know whether to enforce signature verification.
var pciWebhookHandlerRe = regexp.MustCompile(`(?i)(/budget/webhook|/wallet/webhook|stripe[_-]?webhook|paddle[_-]?webhook|webhooks?/(stripe|paddle))`)

// pciSignatureCheckRe matches a Stripe / Paddle signature-verification
// call. Either of `webhook.ConstructEvent` (stripe-go), `Stripe-Signature`
// header verification, or Paddle's `Paddle-Signature` HMAC check counts.
var pciSignatureCheckRe = regexp.MustCompile(`(?i)(webhook\.ConstructEvent|Stripe-Signature|paddle-signature|verifywebhook|verify_webhook|hmac\.New\(sha256|crypto/hmac)`)

func (CompliancePCIGate) Check(ctx context.Context, env *GateEnv) []domain.Issue {
	if env == nil || env.Project == nil {
		return nil
	}
	p := env.Project
	if !p.Spec.HasCompliance("pci") {
		return nil
	}
	var issues []domain.Issue

	// PCI-DSS 3.4 — no raw PAN in source.
	for _, f := range p.Files {
		low := strings.ToLower(f.Path)
		// Skip docs that legitimately quote test card numbers.
		if strings.HasSuffix(low, ".md") || strings.Contains(low, "/docs/") {
			continue
		}
		for _, m := range pciDigitRunRe.FindAllString(f.Content, -1) {
			digits := stripNonDigits(m)
			if len(digits) < 13 || len(digits) > 19 {
				continue
			}
			if !luhnValid(digits) {
				continue
			}
			issues = append(issues, domain.Issue{
				Gate: domain.GateCompliancePCI, Severity: domain.SeverityCritical,
				Message: "PCI 3.4: candidate PAN found (Luhn-valid 13-19 digit sequence)",
				Path:    f.Path,
				Hint:    "do not commit raw card numbers. Use Stripe / Paddle tokens; replace any test fixture with the documented Stripe test PANs in a .env that is gitignored.",
			})
			break
		}
	}

	// PCI-DSS 3.4 — plaintext card columns in SQL.
	for _, f := range p.Files {
		low := strings.ToLower(f.Path)
		if !strings.HasSuffix(low, ".sql") && !strings.Contains(low, "/migrations/") {
			continue
		}
		if pciCardColumnRe.MatchString(f.Content) {
			issues = append(issues, domain.Issue{
				Gate: domain.GateCompliancePCI, Severity: domain.SeverityCritical,
				Message: "PCI 3.4: plaintext cardholder column declared in migration",
				Path:    f.Path,
				Hint:    "PANs must never land in your database. Persist only a Stripe / Paddle token (e.g. payment_method_id), not the card itself.",
			})
		}
	}

	// PCI-DSS 4.2 — TLS on payment routes (reuse the shared SOC2 marker).
	if !hasTLSMarker(p) {
		issues = append(issues, domain.Issue{
			Gate: domain.GateCompliancePCI, Severity: domain.SeverityError,
			Message: "PCI 4.2: no TLS marker found — payment traffic MUST be HTTPS-only",
			Hint:    "terminate TLS at the ingress (Caddy / nginx / cloud LB). Card data on plaintext channels is a PCI ship-stopper.",
		})
	}

	// PCI-DSS 6.5 — webhook signature verification.
	hasHandler := false
	for _, f := range p.Files {
		if pciWebhookHandlerRe.MatchString(f.Path) || pciWebhookHandlerRe.MatchString(f.Content) {
			hasHandler = true
			break
		}
	}
	if hasHandler && !projectMentions(p,
		"webhook.ConstructEvent", "Stripe-Signature", "paddle-signature",
		"verifyWebhook", "verify_webhook",
	) && !anyContentMatches(p, pciSignatureCheckRe) {
		issues = append(issues, domain.Issue{
			Gate: domain.GateCompliancePCI, Severity: domain.SeverityWarning,
			Message: "PCI 6.5: payment webhook handler detected without a signature-verification call",
			Hint:    "verify `Stripe-Signature` via stripe-go `webhook.ConstructEvent`, or HMAC-SHA256 the Paddle body against your endpoint secret. Unsigned webhooks let attackers forge top-ups.",
		})
	}

	// PCI-DSS 10.5 — no card terms in log lines.
	for _, f := range p.Files {
		low := strings.ToLower(f.Path)
		if strings.HasSuffix(low, ".md") || strings.HasSuffix(low, ".example") {
			continue
		}
		if pciLogCardRe.MatchString(f.Content) {
			issues = append(issues, domain.Issue{
				Gate: domain.GateCompliancePCI, Severity: domain.SeverityError,
				Message: "PCI 10.5: log statement mentions card / PAN / CVV",
				Path:    f.Path,
				Hint:    "scrub the payload. Log the Stripe / Paddle token id or the last four digits at most.",
			})
			break
		}
	}

	return issues
}

// luhnValid runs the standard mod-10 checksum. digits must already
// contain only ASCII '0'..'9'.
func luhnValid(digits string) bool {
	sum := 0
	alt := false
	for i := len(digits) - 1; i >= 0; i-- {
		c := int(digits[i] - '0')
		if c < 0 || c > 9 {
			return false
		}
		if alt {
			c *= 2
			if c > 9 {
				c -= 9
			}
		}
		sum += c
		alt = !alt
	}
	return sum%10 == 0
}

// stripNonDigits filters everything but ASCII digits. Cheap O(N).
func stripNonDigits(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			out = append(out, s[i])
		}
	}
	return string(out)
}

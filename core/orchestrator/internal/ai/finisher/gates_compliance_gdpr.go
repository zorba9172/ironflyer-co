// Package finisher — ComplianceGDPRGate.
//
// Enforces the GDPR data-subject-rights surface on EU-serving apps.
// Only fires when Project.Spec.Compliance contains "gdpr".
//
// Controls covered:
//
//   - Art. 7  Cookie consent banner present on a client landing route.
//   - Art. 12 Public privacy policy artefact.
//   - Art. 20 User data export endpoint (`/data-export`,
//     `/account/export`, `/me/export`, or similar).
//   - Art. 17 User account deletion endpoint (`/account/delete`,
//     `/me/delete`, or DELETE on a user route).
//   - Art. 5  No PII in client-side analytics calls
//     (no email/phone/name field name appearing inside `analytics.track`
//     / `gtag` / `mixpanel.track` / `posthog.capture` calls).
//
// Severity ladder: SeverityError on missing export / delete (those are
// statutory rights); SeverityWarning on missing consent banner / PII
// in analytics; SeverityInfo on missing privacy policy artefact (the
// most easily papered-over after the fact).
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

type ComplianceGDPRGate struct{}

func (ComplianceGDPRGate) Name() domain.GateName    { return domain.GateComplianceGDPR }
func (ComplianceGDPRGate) RepairAgent() agents.Role { return agents.RoleSecurity }

// gdprConsentBannerRe matches any of the common cookie / consent banner
// libraries or component markers. Heuristic — at least one match is
// enough to satisfy the gate.
var gdprConsentBannerRe = regexp.MustCompile(`(?i)(cookie-?consent|cookiebot|onetrust|trustarc|klaro|osano|tarteaucitron|<cookiebanner|<consentbanner|gdpr-consent|cookie_banner)`)

// gdprExportRouteRe matches a user-data-export route declared in any
// router (Go, Next.js page, Express, Flask). Variants accepted: data-export,
// account/export, me/export, user/export, profile/export.
var gdprExportRouteRe = regexp.MustCompile(`(?i)["'/]((?:data|account|me|user|profile)[/_-]?export|export[/_-]?(?:my-?data|account|profile))["'/]`)

// gdprDeleteRouteRe matches an account-deletion route. Accepts
// /account/delete, /me/delete, /user/delete, /profile/delete, or a
// declared DELETE handler on /users/:id.
var gdprDeleteRouteRe = regexp.MustCompile(`(?i)["'/]((?:account|me|user|profile)[/_-]?delete|delete[/_-]?(?:account|profile|my-?data))["'/]`)

// gdprDeleteVerbRe is the secondary signal — a router/method binding
// that wires DELETE on a user-scoped resource. Less strict but useful
// when the project picked REST verbs over named routes.
var gdprDeleteVerbRe = regexp.MustCompile(`(?i)(\.Delete\(|router\.delete\(|app\.delete\(|@DeleteMapping|methods=\["DELETE"\])\s*[("'][^"')]*\b(users?|accounts?|me|profile)\b`)

// gdprAnalyticsPIIRe matches an analytics call (gtag/posthog/mixpanel/
// segment/amplitude/analytics.track) whose payload literal contains a
// PII field name (email/phone/name/firstname/lastname/ssn/dob).
// Conservative: the match is on the same line as the call so we do not
// flag multi-line property bag literals.
var gdprAnalyticsPIIRe = regexp.MustCompile(`(?i)(gtag\(|posthog\.capture\(|mixpanel\.track\(|analytics\.track\(|segment\.track\(|amplitude\.track\()[^;\n]{0,200}\b(email|phone|first_?name|last_?name|full_?name|ssn|dob|date_of_birth)\b`)

// gdprPrivacyPolicyPrefixes matches the canonical paths a privacy
// policy can live at. Case-insensitive.
var gdprPrivacyPolicyPrefixes = []string{
	"privacy.md", "privacy.html", "privacy.tsx", "privacy.jsx", "privacy.vue",
	"docs/privacy", "legal/privacy",
	"public/privacy", "pages/privacy", "app/privacy",
	"clients/web/src/app/privacy",
}

func (ComplianceGDPRGate) Check(ctx context.Context, env *GateEnv) []domain.Issue {
	if env == nil || env.Project == nil {
		return nil
	}
	p := env.Project
	if !p.Spec.HasCompliance("gdpr") {
		return nil
	}
	var issues []domain.Issue

	// Art. 7 — cookie consent banner.
	if !anyContentMatches(p, gdprConsentBannerRe) &&
		!projectMentions(p, "cookieConsent", "cookie_consent", "cookie-consent") {
		issues = append(issues, domain.Issue{
			Gate: domain.GateComplianceGDPR, Severity: domain.SeverityWarning,
			Message: "Art. 7: no cookie consent banner detected across the client surface",
			Hint:    "ship a consent gate (Cookiebot / OneTrust / Klaro / your own banner) before any analytics / marketing tag fires.",
		})
	}

	// Art. 12 — public privacy policy artefact.
	if !hasGDPRPrivacyArtefact(p) {
		issues = append(issues, domain.Issue{
			Gate: domain.GateComplianceGDPR, Severity: domain.SeverityInfo,
			Message: "Art. 12: no privacy policy artefact (privacy.md / pages/privacy / legal/privacy)",
			Hint:    "publish a privacy.md (or a /privacy route in the client) naming controller, lawful basis, retention, and the DPO contact.",
		})
	}

	// Art. 20 — data export endpoint.
	if !anyContentMatches(p, gdprExportRouteRe) &&
		!projectMentions(p, "dataExport", "data_export", "exportMyData", "export_my_data") {
		issues = append(issues, domain.Issue{
			Gate: domain.GateComplianceGDPR, Severity: domain.SeverityError,
			Message: "Art. 20: no user data export endpoint detected (e.g. /data-export, /account/export)",
			Hint:    "expose a route that returns the user's personal data on demand — Article 20 is a statutory right.",
		})
	}

	// Art. 17 — account deletion endpoint.
	if !anyContentMatches(p, gdprDeleteRouteRe) && !anyContentMatches(p, gdprDeleteVerbRe) &&
		!projectMentions(p, "deleteAccount", "delete_account", "rightToBeForgotten", "right_to_be_forgotten") {
		issues = append(issues, domain.Issue{
			Gate: domain.GateComplianceGDPR, Severity: domain.SeverityError,
			Message: "Art. 17: no account deletion endpoint detected (e.g. /account/delete, DELETE /users/:id)",
			Hint:    "wire a deletion handler that scrubs PII and downstream traces. Right-to-be-forgotten is statutory.",
		})
	}

	// Art. 5 — no PII in client-side analytics calls.
	for _, f := range p.Files {
		low := strings.ToLower(f.Path)
		// Only inspect client/web surfaces (TS/JS/TSX/JSX/Vue/Svelte).
		if !strings.HasSuffix(low, ".ts") && !strings.HasSuffix(low, ".tsx") &&
			!strings.HasSuffix(low, ".js") && !strings.HasSuffix(low, ".jsx") &&
			!strings.HasSuffix(low, ".vue") && !strings.HasSuffix(low, ".svelte") {
			continue
		}
		if gdprAnalyticsPIIRe.MatchString(f.Content) {
			issues = append(issues, domain.Issue{
				Gate: domain.GateComplianceGDPR, Severity: domain.SeverityWarning,
				Message: "Art. 5: client analytics call appears to ship raw PII (email/phone/name)",
				Path:    f.Path,
				Hint:    "hash or omit PII before forwarding to analytics. Use a server-side proxy when you need user-keyed cohorts.",
			})
			break
		}
	}

	return issues
}

// hasGDPRPrivacyArtefact returns true when any project file matches the
// known privacy-policy prefixes. Case-insensitive over the full path.
func hasGDPRPrivacyArtefact(p *domain.Project) bool {
	for _, f := range p.Files {
		low := strings.ToLower(strings.TrimPrefix(f.Path, "/"))
		base := filepathBase(low)
		if base == "privacy.md" || base == "privacy.html" ||
			base == "privacy.tsx" || base == "privacy.jsx" || base == "privacy.vue" ||
			strings.HasPrefix(base, "privacy.") {
			return true
		}
		for _, pref := range gdprPrivacyPolicyPrefixes {
			pl := strings.ToLower(pref)
			if low == pl || strings.HasPrefix(low, pl) || strings.Contains(low, "/"+pl) {
				return true
			}
		}
	}
	return false
}

// Package finisher — LighthouseGate.
//
// The LighthouseGate is Ironflyer's brand-quality enforcer: every web
// project that reaches Deploy gets audited against Google's PageSpeed
// Insights API (Lighthouse on Google infra) for Performance,
// Accessibility, SEO, and Best Practices. The competitive frame is
// "production discipline" — competitors ship code without ever
// measuring the live-runtime quality of what they generated. This
// gate makes "Lighthouse passed" a hard precondition for closing an
// execution.
//
// The gate is intentionally network-bound (calls Google's public PSI
// endpoint) rather than runtime-bound (would require Chromium inside
// the user's sandbox). That choice keeps the runtime image lean and
// the gate stack-agnostic: any publicly-reachable URL works the same
// way for Next.js, Vite, Astro, static HTML, or a deployed Go
// frontend.
//
// Operator switches:
//
//   - IRONFLYER_LIGHTHOUSE_GATE_DISABLE=true        — gate emits a warn-only stub.
//   - IRONFLYER_LIGHTHOUSE_TARGET_URL=https://...  — fallback URL when
//     env.DeployURL is empty (useful for self-hosted setups without a
//     wired deploy store).
//   - IRONFLYER_LIGHTHOUSE_PSI_API_KEY=...         — PageSpeed Insights API
//     key; raises the quota from the unauthenticated tier.
//   - IRONFLYER_LIGHTHOUSE_STRATEGY=mobile|desktop  — audit strategy.
//     Defaults to "mobile" (matches Google's own default and Core Web
//     Vitals reporting).
//   - IRONFLYER_LIGHTHOUSE_THRESHOLD_PERFORMANCE=80
//     IRONFLYER_LIGHTHOUSE_THRESHOLD_ACCESSIBILITY=90
//     IRONFLYER_LIGHTHOUSE_THRESHOLD_SEO=80
//     IRONFLYER_LIGHTHOUSE_THRESHOLD_BEST_PRACTICES=80
//                                                  — per-category score thresholds
//     (0-100). Defaults above match the brand-quality bar baked into
//     CLAUDE.md (a11y is treated more strictly than the other axes).
//   - IRONFLYER_LIGHTHOUSE_TIMEOUT_SECONDS=60      — HTTP timeout for the
//     PSI call. Default 60s.
//
// The gate skips silently (returns nil issues = passes) when:
//   - The project is mobile-only or pure backend (no web frontend).
//   - No DeployURL is available and no override env var is set.
//   - The operator explicitly disabled the gate (warn-only stub).
//
// It emits SeverityError (blocking) for Performance + Accessibility
// failures, SeverityWarning for SEO + Best Practices, and uses the
// generic Coder as the repair agent — markup, CSS, image, and meta
// tag fixes are exactly the Coder's wheelhouse.

package finisher

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"ironflyer/apps/orchestrator/internal/agents"
	"ironflyer/apps/orchestrator/internal/domain"
)

// LighthouseGate audits the deployed web artifact via PageSpeed Insights.
type LighthouseGate struct {
	// HTTP is the client used for the PSI call. Nil falls back to a
	// per-call http.Client with the configured timeout. Exposed so the
	// engine can inject a shared transport (connection pooling) when
	// many projects audit in parallel.
	HTTP *http.Client
}

func (LighthouseGate) Name() domain.GateName    { return domain.GateLighthouse }
func (LighthouseGate) RepairAgent() agents.Role { return agents.RoleCoder }

func (g LighthouseGate) Check(ctx context.Context, env *GateEnv) []domain.Issue {
	if env == nil || env.Project == nil {
		return nil
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("IRONFLYER_LIGHTHOUSE_GATE_DISABLE")), "true") {
		return []domain.Issue{{
			Gate: domain.GateLighthouse, Severity: domain.SeverityWarning,
			Message: "lighthouse gate disabled by operator (IRONFLYER_LIGHTHOUSE_GATE_DISABLE=true)",
			Hint:    "remove the env var to re-enable Performance / Accessibility / SEO / Best Practices auditing",
		}}
	}
	p := env.Project
	if !projectHasWebFrontend(p) {
		// Pure mobile or pure backend — Lighthouse has nothing to audit.
		return nil
	}
	target := resolveLighthouseTarget(env)
	if target == "" {
		// No URL to point Google at. Stay silent rather than block — the
		// gate is most useful after a real Deploy; before that, the project
		// hasn't graduated to "publicly auditable" yet.
		return nil
	}

	cfg := loadLighthouseConfig()
	report, err := runLighthouseAudit(ctx, g.HTTP, target, cfg)
	if err != nil {
		// A failed PSI call is a soft signal, not a brand-blocker. Emit a
		// warning so the dashboard surfaces the dark gate without
		// halting the loop on every transient Google outage.
		return []domain.Issue{{
			Gate: domain.GateLighthouse, Severity: domain.SeverityWarning,
			Message: "lighthouse audit could not run: " + err.Error(),
			Hint:    "verify the deploy URL is publicly reachable and PSI API is up",
		}}
	}
	return lighthouseIssues(report, cfg)
}

// projectHasWebFrontend filters out mobile-only / backend-only projects so
// the gate stays dark where it has no signal. A project is "web" when the
// stack declares a recognised frontend OR a workspace file points at a web
// build (index.html / package.json / Next/Vite config). Mobile is allowed
// to co-exist — many projects ship a marketing site alongside the app.
func projectHasWebFrontend(p *domain.Project) bool {
	frontend := strings.ToLower(strings.TrimSpace(p.Spec.Stack.Frontend))
	switch {
	case strings.Contains(frontend, "next"),
		strings.Contains(frontend, "vite"),
		strings.Contains(frontend, "remix"),
		strings.Contains(frontend, "astro"),
		strings.Contains(frontend, "react"),
		strings.Contains(frontend, "svelte"),
		strings.Contains(frontend, "vue"),
		strings.Contains(frontend, "solid"),
		strings.Contains(frontend, "static"),
		strings.Contains(frontend, "html"):
		return true
	}
	for _, f := range p.Files {
		path := strings.ToLower(f.Path)
		switch {
		case path == "index.html",
			strings.HasSuffix(path, "/index.html"),
			path == "next.config.js",
			path == "next.config.mjs",
			path == "next.config.ts",
			path == "vite.config.ts",
			path == "vite.config.js",
			path == "astro.config.mjs",
			path == "package.json":
			return true
		}
	}
	return false
}

// resolveLighthouseTarget picks the URL the PSI call will audit. Engine-
// supplied DeployURL wins; env-var override is the self-hosted fallback.
func resolveLighthouseTarget(env *GateEnv) string {
	if env == nil {
		return ""
	}
	if u := strings.TrimSpace(env.DeployURL); u != "" {
		return u
	}
	if u := strings.TrimSpace(os.Getenv("IRONFLYER_LIGHTHOUSE_TARGET_URL")); u != "" {
		return u
	}
	return ""
}

// lighthouseCategoryKey is one PSI category we score against a threshold.
type lighthouseCategoryKey struct {
	// id matches Lighthouse's category id in the PSI response.
	id string
	// label is the human-facing name surfaced on Issue messages.
	label string
	// severity is the severity emitted when the category score falls
	// below the configured threshold.
	severity domain.Severity
}

var lighthouseCategories = []lighthouseCategoryKey{
	{id: "performance", label: "Performance", severity: domain.SeverityError},
	{id: "accessibility", label: "Accessibility", severity: domain.SeverityError},
	{id: "seo", label: "SEO", severity: domain.SeverityWarning},
	{id: "best-practices", label: "Best Practices", severity: domain.SeverityWarning},
}

// lighthouseConfig captures the operator-tunable bits in one place so the
// audit code stays declarative.
type lighthouseConfig struct {
	thresholds  map[string]int
	strategy    string
	apiKey      string
	timeoutSecs int
}

func loadLighthouseConfig() lighthouseConfig {
	c := lighthouseConfig{
		thresholds: map[string]int{
			"performance":    envIntDefault("IRONFLYER_LIGHTHOUSE_THRESHOLD_PERFORMANCE", 80),
			"accessibility":  envIntDefault("IRONFLYER_LIGHTHOUSE_THRESHOLD_ACCESSIBILITY", 90),
			"seo":            envIntDefault("IRONFLYER_LIGHTHOUSE_THRESHOLD_SEO", 80),
			"best-practices": envIntDefault("IRONFLYER_LIGHTHOUSE_THRESHOLD_BEST_PRACTICES", 80),
		},
		strategy:    strings.ToLower(strings.TrimSpace(os.Getenv("IRONFLYER_LIGHTHOUSE_STRATEGY"))),
		apiKey:      strings.TrimSpace(os.Getenv("IRONFLYER_LIGHTHOUSE_PSI_API_KEY")),
		timeoutSecs: envIntDefault("IRONFLYER_LIGHTHOUSE_TIMEOUT_SECONDS", 60),
	}
	if c.strategy != "desktop" {
		c.strategy = "mobile"
	}
	for k, v := range c.thresholds {
		if v < 0 {
			c.thresholds[k] = 0
		}
		if v > 100 {
			c.thresholds[k] = 100
		}
	}
	if c.timeoutSecs <= 0 {
		c.timeoutSecs = 60
	}
	return c
}

func envIntDefault(name string, def int) int {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

// psiResponse is the minimal subset of the PageSpeed Insights v5
// payload the gate parses. PSI returns a full Lighthouse JSON under
// lighthouseResult; we only need category scores + a handful of
// Core Web Vitals audits.
type psiResponse struct {
	LighthouseResult struct {
		Categories map[string]struct {
			Score float64 `json:"score"`
			Title string  `json:"title"`
		} `json:"categories"`
		Audits map[string]struct {
			Title            string  `json:"title"`
			DisplayValue     string  `json:"displayValue"`
			Score            float64 `json:"score"`
			NumericValue     float64 `json:"numericValue"`
			NumericUnit      string  `json:"numericUnit"`
			ScoreDisplayMode string  `json:"scoreDisplayMode"`
		} `json:"audits"`
	} `json:"lighthouseResult"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// runLighthouseAudit issues the PSI call and parses the response. Network
// errors and 4xx/5xx HTTP statuses surface as errors so the gate can
// degrade to a warning rather than a false-pass.
func runLighthouseAudit(ctx context.Context, client *http.Client, target string, cfg lighthouseConfig) (*psiResponse, error) {
	if client == nil {
		client = &http.Client{Timeout: time.Duration(cfg.timeoutSecs) * time.Second}
	}
	q := url.Values{}
	q.Set("url", target)
	q.Set("strategy", cfg.strategy)
	for _, c := range lighthouseCategories {
		q.Add("category", c.id)
	}
	if cfg.apiKey != "" {
		q.Set("key", cfg.apiKey)
	}
	endpoint := "https://www.googleapis.com/pagespeedonline/v5/runPagespeed?" + q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Ironflyer-LighthouseGate/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20)) // 8 MiB ceiling — PSI responses are typically ~1-2 MiB.
	if err != nil {
		return nil, err
	}
	if resp.StatusCode/100 != 2 {
		return nil, errors.New("PSI HTTP " + strconv.Itoa(resp.StatusCode) + ": " + tail(string(body), 240))
	}
	var out psiResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, errors.New("PSI JSON decode: " + err.Error())
	}
	if out.Error != nil {
		return nil, errors.New("PSI error " + strconv.Itoa(out.Error.Code) + ": " + out.Error.Message)
	}
	if len(out.LighthouseResult.Categories) == 0 {
		return nil, errors.New("PSI response missing lighthouseResult.categories")
	}
	return &out, nil
}

// lighthouseIssues converts a parsed PSI response into the Issue slice the
// engine consumes. Empty slice = gate passes; any element fails the gate.
func lighthouseIssues(report *psiResponse, cfg lighthouseConfig) []domain.Issue {
	if report == nil {
		return nil
	}
	var issues []domain.Issue

	for _, cat := range lighthouseCategories {
		entry, ok := report.LighthouseResult.Categories[cat.id]
		if !ok {
			continue
		}
		score := int(entry.Score*100 + 0.5)
		threshold := cfg.thresholds[cat.id]
		if score >= threshold {
			continue
		}
		issues = append(issues, domain.Issue{
			Gate:     domain.GateLighthouse,
			Severity: cat.severity,
			Message:  cat.label + " score " + strconv.Itoa(score) + " is below threshold " + strconv.Itoa(threshold),
			Hint:     lighthouseHintFor(cat.id, report),
		})
	}

	// Core Web Vitals: surface the specific numeric audits even when the
	// rolled-up Performance score happened to scrape past threshold —
	// CWV breaches still hurt rankings and conversion.
	issues = append(issues, lighthouseCoreWebVitalIssues(report)...)

	return issues
}

// lighthouseHintFor builds a per-category Hint pointing the Coder at the
// top failing audits for that category. PSI returns the full audit map;
// we surface up to 3 failing audit titles so the repair prompt is
// grounded in concrete defects rather than a bare score.
func lighthouseHintFor(category string, report *psiResponse) string {
	if report == nil {
		return ""
	}
	auditIDsForCategory := lighthouseAuditIDs(category)
	type failing struct {
		title   string
		display string
	}
	var found []failing
	for _, id := range auditIDsForCategory {
		a, ok := report.LighthouseResult.Audits[id]
		if !ok {
			continue
		}
		if a.ScoreDisplayMode == "informative" || a.ScoreDisplayMode == "notApplicable" || a.ScoreDisplayMode == "manual" {
			continue
		}
		if a.Score >= 0.9 {
			continue
		}
		title := strings.TrimSpace(a.Title)
		if title == "" {
			title = id
		}
		found = append(found, failing{title: title, display: strings.TrimSpace(a.DisplayValue)})
		if len(found) >= 3 {
			break
		}
	}
	if len(found) == 0 {
		return "raise " + category + " score via the failing audits in the live Lighthouse report"
	}
	var b strings.Builder
	b.WriteString("top failing audits: ")
	for i, f := range found {
		if i > 0 {
			b.WriteString("; ")
		}
		b.WriteString(f.title)
		if f.display != "" {
			b.WriteString(" (")
			b.WriteString(f.display)
			b.WriteString(")")
		}
	}
	return b.String()
}

// lighthouseAuditIDs returns the audit IDs Lighthouse groups under a given
// category. The list is conservative — only the high-signal audits the
// Coder can fix without invoking a CDN or build pipeline rewrite.
func lighthouseAuditIDs(category string) []string {
	switch category {
	case "performance":
		return []string{
			"largest-contentful-paint",
			"cumulative-layout-shift",
			"total-blocking-time",
			"first-contentful-paint",
			"speed-index",
			"interactive",
			"render-blocking-resources",
			"unused-css-rules",
			"unused-javascript",
			"uses-text-compression",
			"uses-optimized-images",
			"uses-responsive-images",
			"modern-image-formats",
			"efficient-animated-content",
			"unminified-css",
			"unminified-javascript",
			"server-response-time",
		}
	case "accessibility":
		return []string{
			"color-contrast",
			"image-alt",
			"label",
			"link-name",
			"button-name",
			"document-title",
			"html-has-lang",
			"html-lang-valid",
			"meta-viewport",
			"tabindex",
			"aria-required-attr",
			"aria-valid-attr",
			"aria-valid-attr-value",
			"heading-order",
			"list",
			"listitem",
		}
	case "seo":
		return []string{
			"document-title",
			"meta-description",
			"http-status-code",
			"link-text",
			"crawlable-anchors",
			"is-crawlable",
			"robots-txt",
			"hreflang",
			"canonical",
			"font-size",
			"viewport",
			"tap-targets",
		}
	case "best-practices":
		return []string{
			"is-on-https",
			"uses-http2",
			"no-vulnerable-libraries",
			"errors-in-console",
			"image-aspect-ratio",
			"image-size-responsive",
			"deprecations",
			"csp-xss",
			"inspector-issues",
			"password-inputs-can-be-pasted-into",
		}
	}
	return nil
}

// lighthouseCoreWebVitalIssues surfaces individual CWV breaches even when
// the rolled-up Performance category passed. Google ranks pages on the
// raw CWV thresholds (LCP <= 2.5s, CLS <= 0.1, INP/TBT <= 200ms), so a
// "good enough" composite score can still hurt SEO if one vital is bad.
func lighthouseCoreWebVitalIssues(report *psiResponse) []domain.Issue {
	if report == nil {
		return nil
	}
	type vital struct {
		auditID string
		label   string
		// goodMaxMS is the upper bound (in the audit's native unit — ms
		// for timing audits, dimensionless score for CLS) at which the
		// vital is still considered "good" per Google's published
		// thresholds. A breach here is always a SeverityError because
		// CWV directly affects ranking, not just score.
		goodMaxMS float64
		unitLabel string
	}
	vitals := []vital{
		{auditID: "largest-contentful-paint", label: "Largest Contentful Paint (LCP)", goodMaxMS: 2500, unitLabel: "ms"},
		{auditID: "cumulative-layout-shift", label: "Cumulative Layout Shift (CLS)", goodMaxMS: 0.1, unitLabel: ""},
		{auditID: "total-blocking-time", label: "Total Blocking Time (TBT)", goodMaxMS: 200, unitLabel: "ms"},
		{auditID: "interaction-to-next-paint", label: "Interaction to Next Paint (INP)", goodMaxMS: 200, unitLabel: "ms"},
	}
	var issues []domain.Issue
	for _, v := range vitals {
		a, ok := report.LighthouseResult.Audits[v.auditID]
		if !ok {
			continue
		}
		if a.NumericValue <= v.goodMaxMS {
			continue
		}
		issues = append(issues, domain.Issue{
			Gate:     domain.GateLighthouse,
			Severity: domain.SeverityError,
			Message:  v.label + " breached the Core Web Vitals \"good\" threshold",
			Hint:     formatCWVHint(v.label, a.NumericValue, v.goodMaxMS, v.unitLabel),
		})
	}
	return issues
}

func formatCWVHint(label string, observed, threshold float64, unit string) string {
	return "observed " + formatVitalNumber(observed, unit) +
		" vs. target " + formatVitalNumber(threshold, unit) +
		" — fix the Lighthouse audit Google flagged for " + label
}

func formatVitalNumber(v float64, unit string) string {
	// CLS comes through unitless and small (e.g. 0.27); timing audits
	// come through in ms and are typically 100-10000. We pick precision
	// per range so the Hint reads naturally in both regimes.
	if unit == "" {
		return strconv.FormatFloat(v, 'f', 3, 64)
	}
	return strconv.FormatFloat(v, 'f', 0, 64) + unit
}

package notify

import (
	"fmt"
	"strings"
	"time"
)

// fallbackDashboardURL is the destination for CTA buttons when no
// explicit DashboardURL is wired through. Override via the Engine or
// Dispatcher so staging + production land on the right host.
const fallbackDashboardURL = "https://ironflyer.dev/app"

// EmailContent is the trio every sender consumes — subject + html + text.
type EmailContent struct {
	Subject  string
	HTMLBody string
	TextBody string
}

// renderRunComplete builds the email body for a successful project run.
func renderRunComplete(projectName, projectID, dashboardURL string) EmailContent {
	subject := fmt.Sprintf("%s run completed successfully", projectName)
	heading := "Run complete"
	body := `<p style="margin:0 0 16px 0;">Every gate passed and <strong style="color:#f7f4ff;">` + escapeHTML(projectName) + `</strong> is ready for review.</p>`
	link := projectLink(dashboardURL, projectID)
	return EmailContent{
		Subject:  subject,
		HTMLBody: wrapHTML(heading, body, "View project", link, dashboardURL),
		TextBody: textVersion(subject, projectName+" finished the run and is ready for review.", link),
	}
}

// renderGateFailed builds the email body for a gate failure event.
func renderGateFailed(projectName, projectID, gateName, reason, dashboardURL string) EmailContent {
	subject := fmt.Sprintf("%s gate failed in %s", gateName, projectName)
	heading := "Gate failed"
	body := `<p style="margin:0 0 16px 0;">The <strong style="color:#f7f4ff;">` + escapeHTML(gateName) +
		`</strong> gate failed in <strong style="color:#f7f4ff;">` + escapeHTML(projectName) + `</strong>.</p>` +
		`<p style="margin:0 0 16px 0;">Reason: ` + escapeHTML(reason) + `</p>`
	link := projectLink(dashboardURL, projectID)
	return EmailContent{
		Subject:  subject,
		HTMLBody: wrapHTML(heading, body, "Open project", link, dashboardURL),
		TextBody: textVersion(subject, "The "+gateName+" gate failed in "+projectName+". Reason: "+reason, link),
	}
}

// renderDeployDone builds the email body for a successful deploy event.
func renderDeployDone(projectName, projectID, dashboardURL string) EmailContent {
	subject := fmt.Sprintf("%s is live", projectName)
	heading := "Deployment complete"
	body := `<p style="margin:0 0 16px 0;"><strong style="color:#f7f4ff;">` + escapeHTML(projectName) + `</strong> deployed successfully.</p>`
	link := projectLink(dashboardURL, projectID)
	return EmailContent{
		Subject:  subject,
		HTMLBody: wrapHTML(heading, body, "Open project", link, dashboardURL),
		TextBody: textVersion(subject, projectName+" deployed successfully.", link),
	}
}

// renderBudgetWarning builds the email body when the user is near their cap.
func renderBudgetWarning(projectName, dashboardURL string) EmailContent {
	subject := "Budget warning: approaching plan cap"
	heading := "Budget attention needed"
	body := `<p style="margin:0 0 16px 0;">Usage for <strong style="color:#f7f4ff;">` + escapeHTML(projectName) + `</strong> is approaching the plan cap. Top up the wallet to keep paid executions running.</p>`
	link := strings.TrimRight(orFallback(dashboardURL), "/") + "/billing"
	return EmailContent{
		Subject:  subject,
		HTMLBody: wrapHTML(heading, body, "Open budget", link, dashboardURL),
		TextBody: textVersion(subject, "Usage for "+projectName+" is approaching the plan cap.", link),
	}
}

// renderWelcome is sent once on signup. Tone: confident, no hype.
func renderWelcome(name, _, dashboardURL string) EmailContent {
	subject := "Welcome to Ironflyer — your workspace is ready"
	heading := "Welcome."
	if n := strings.TrimSpace(name); n != "" {
		heading = "Welcome, " + escapeHTML(n) + "."
	}
	body := `<p style="margin:0 0 16px 0;">Your Ironflyer workspace is set up. Sign in to start a build from a Figma file or a prompt.</p>` +
		`<p style="margin:0 0 16px 0;">Gates block bad releases. Wallet runs every paid execution. Patches stay reviewable. You stay in control.</p>`
	link := orFallback(dashboardURL)
	return EmailContent{
		Subject:  subject,
		HTMLBody: wrapHTML(heading, body, "Open Studio", link, dashboardURL),
		TextBody: textVersion(subject, "Your Ironflyer workspace is set up. Gates block bad releases, the wallet runs every paid execution, patches stay reviewable.", link),
	}
}

// renderPasswordReset frames the link as one-shot, expiring, and safe to
// ignore. Includes the raw URL inside the body so clients that strip
// the CTA button still recover. dashboardURL is purely for the footer
// "manage notifications" anchor; the CTA points at resetURL.
func renderPasswordReset(name, resetURL string, ttl time.Duration, dashboardURL string) EmailContent {
	subject := "Reset your Ironflyer password"
	heading := "Reset your password"
	expires := humanizeTTL(ttl)
	greeting := ""
	if n := strings.TrimSpace(name); n != "" {
		greeting = `<p style="margin:0 0 16px 0;">Hi ` + escapeHTML(n) + `,</p>`
	}
	body := greeting +
		`<p style="margin:0 0 16px 0;">You requested a password reset. The link below works once and expires in ` + escapeHTML(expires) + `.</p>` +
		`<p style="margin:0 0 16px 0;">If you didn't request this, ignore this email — your password stays unchanged.</p>` +
		`<p style="margin:24px 0 0 0;font-size:13px;color:#777096;">If the button does not work, paste this URL into your browser:<br>` +
		`<a href="` + escapeAttr(resetURL) + `" style="color:#b56cff;word-break:break-all;">` + escapeHTML(resetURL) + `</a></p>`
	return EmailContent{
		Subject:  subject,
		HTMLBody: wrapHTML(heading, body, "Reset password", resetURL, dashboardURL),
		TextBody: textVersion(subject, "You requested a password reset. The link expires in "+expires+". If you didn't request this, ignore this email.", resetURL),
	}
}

// renderReceipt acknowledges a successful Stripe top-up. Always sent
// (legal-relevant): includes amount, currency, transaction id, date.
func renderReceipt(name, currency string, amountCents int, transactionID, dashboardURL string) EmailContent {
	formatted := formatMoney(currency, amountCents)
	subject := "Receipt: " + formatted + " added to your Ironflyer wallet"
	heading := "Top-up confirmed"
	greeting := ""
	if n := strings.TrimSpace(name); n != "" {
		greeting = `<p style="margin:0 0 16px 0;">Hi ` + escapeHTML(n) + `,</p>`
	}
	date := time.Now().UTC().Format("2006-01-02 15:04 UTC")
	row := func(label, value string) string {
		return `<tr>` +
			`<td style="padding:8px 12px;border-bottom:1px solid rgba(178,133,255,0.16);font-size:13px;color:#777096;width:40%;">` + label + `</td>` +
			`<td style="padding:8px 12px;border-bottom:1px solid rgba(178,133,255,0.16);font-size:14px;color:#f7f4ff;font-family:'SFMono-Regular','Menlo','Consolas',monospace;">` + escapeHTML(value) + `</td>` +
			`</tr>`
	}
	receiptTable := `<table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%" style="margin:16px 0 0 0;background-color:#11132a;border:1px solid rgba(178,133,255,0.16);border-radius:8px;border-collapse:separate;">` +
		row("Amount", formatted) +
		row("Currency", strings.ToUpper(currency)) +
		row("Transaction ID", transactionID) +
		row("Date", date) +
		`</table>`
	body := greeting +
		`<p style="margin:0 0 16px 0;">We received your top-up. Funds are available immediately for paid executions.</p>` +
		receiptTable
	link := strings.TrimRight(orFallback(dashboardURL), "/") + "/wallet"
	return EmailContent{
		Subject:  subject,
		HTMLBody: wrapHTML(heading, body, "Open wallet", link, dashboardURL),
		TextBody: textVersion(subject,
			"Top-up confirmed. Amount: "+formatted+". Currency: "+strings.ToUpper(currency)+
				". Transaction ID: "+transactionID+". Date: "+date+".", link),
	}
}

// projectLink composes the URL the email CTA points at when a project
// is in scope.
func projectLink(dashboardURL, projectID string) string {
	base := strings.TrimRight(orFallback(dashboardURL), "/")
	if projectID == "" {
		return base
	}
	return base + "/projects/" + projectID
}

// securityLink composes the canonical sessions/security URL the CTA
// for MFAEnabled + NewDeviceLogin templates points at.
func securityLink(dashboardURL string) string {
	return strings.TrimRight(orFallback(dashboardURL), "/") + "/settings/security"
}

// walletLink composes the canonical wallet URL the LowBalance CTA
// points at.
func walletLink(dashboardURL string) string {
	return strings.TrimRight(orFallback(dashboardURL), "/") + "/wallet"
}

// truncateUA shortens a User-Agent string for inline display. The
// templates render UA inside a body line; an unbounded UA would blow
// up the rendered email height. Strings shorter than max pass through
// unchanged; longer strings are clipped and an ellipsis is appended.
func truncateUA(ua string, max int) string {
	if max <= 1 {
		return ua
	}
	if len(ua) <= max {
		return ua
	}
	return ua[:max-1] + "…"
}

// renderMFAEnabled is the security-template body for KindMFAEnabled.
// Subject is short by design — inbox previews truncate past ~50 chars
// and the user must see "MFA enabled" before opening.
func renderMFAEnabled(name string, at time.Time, dashboardURL string) EmailContent {
	subject := "Two-factor authentication enabled"
	heading := "MFA enabled"
	greeting := ""
	if n := strings.TrimSpace(name); n != "" {
		greeting = `<p style="margin:0 0 16px 0;">Hi ` + escapeHTML(n) + `,</p>`
	}
	stamp := at.UTC()
	if stamp.IsZero() {
		stamp = time.Now().UTC()
	}
	when := stamp.Format("2006-01-02 15:04 UTC")
	body := greeting +
		`<p style="margin:0 0 16px 0;">MFA was enabled on your Ironflyer account at <strong style="color:#f7f4ff;">` + escapeHTML(when) + `</strong>.</p>` +
		`<p style="margin:0 0 16px 0;">If this wasn't you, reset your password and revoke all sessions immediately.</p>`
	link := securityLink(dashboardURL)
	return EmailContent{
		Subject:  subject,
		HTMLBody: wrapHTML(heading, body, "Review sessions", link, dashboardURL),
		TextBody: textVersion(subject,
			"MFA was enabled on your Ironflyer account at "+when+". If this wasn't you, reset your password and revoke all sessions immediately.",
			link),
	}
}

// renderNewDeviceLogin is the security-template body for
// KindNewDeviceLogin. Body lists IP, User-Agent (truncated), location,
// and time. Footer line is the standard "if this wasn't you" call-out.
func renderNewDeviceLogin(name, ip, ua, location string, at time.Time, dashboardURL string) EmailContent {
	where := strings.TrimSpace(location)
	if where == "" {
		where = strings.TrimSpace(ip)
	}
	if where == "" {
		where = "an unknown location"
	}
	subject := "New sign-in from " + where
	heading := "New sign-in detected"
	greeting := ""
	if n := strings.TrimSpace(name); n != "" {
		greeting = `<p style="margin:0 0 16px 0;">Hi ` + escapeHTML(n) + `,</p>`
	}
	stamp := at.UTC()
	if stamp.IsZero() {
		stamp = time.Now().UTC()
	}
	when := stamp.Format("2006-01-02 15:04 UTC")
	uaDisplay := truncateUA(ua, 80)
	row := func(label, value string) string {
		if strings.TrimSpace(value) == "" {
			value = "—"
		}
		return `<tr>` +
			`<td style="padding:8px 12px;border-bottom:1px solid rgba(178,133,255,0.16);font-size:13px;color:#777096;width:40%;">` + label + `</td>` +
			`<td style="padding:8px 12px;border-bottom:1px solid rgba(178,133,255,0.16);font-size:14px;color:#f7f4ff;font-family:'SFMono-Regular','Menlo','Consolas',monospace;">` + escapeHTML(value) + `</td>` +
			`</tr>`
	}
	table := `<table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%" style="margin:16px 0 0 0;background-color:#11132a;border:1px solid rgba(178,133,255,0.16);border-radius:8px;border-collapse:separate;">` +
		row("IP address", ip) +
		row("Device", uaDisplay) +
		row("Location", location) +
		row("Time", when) +
		`</table>`
	body := greeting +
		`<p style="margin:0 0 16px 0;">A new device just signed in to your Ironflyer account.</p>` +
		table +
		`<p style="margin:24px 0 0 0;">If this wasn't you, change your password and revoke all sessions.</p>`
	link := securityLink(dashboardURL)
	return EmailContent{
		Subject:  subject,
		HTMLBody: wrapHTML(heading, body, "Review sessions", link, dashboardURL),
		TextBody: textVersion(subject,
			"A new device just signed in. IP: "+ip+". Device: "+uaDisplay+". Location: "+location+". Time: "+when+". If this wasn't you, change your password and revoke all sessions.",
			link),
	}
}

// renderLowBalance is the financial-template body for KindLowBalance.
// CTA points at /wallet so the user can top up immediately. Body
// states the current balance, threshold, and the 402-refuse impact.
func renderLowBalance(name, currency string, balanceCents, thresholdCents int, dashboardURL string) EmailContent {
	balance := formatMoney(currency, balanceCents)
	threshold := formatMoney(currency, thresholdCents)
	subject := "Wallet balance below " + threshold
	heading := "Top up your wallet"
	greeting := ""
	if n := strings.TrimSpace(name); n != "" {
		greeting = `<p style="margin:0 0 16px 0;">Hi ` + escapeHTML(n) + `,</p>`
	}
	body := greeting +
		`<p style="margin:0 0 16px 0;">Your Ironflyer wallet balance is <strong style="color:#f7f4ff;">` + escapeHTML(balance) + `</strong>, below your alert threshold of <strong style="color:#f7f4ff;">` + escapeHTML(threshold) + `</strong>.</p>` +
		`<p style="margin:0 0 16px 0;">Paid executions will refuse with 402 once the balance hits zero. Top up the wallet to keep runs and deploys flowing.</p>`
	link := walletLink(dashboardURL)
	return EmailContent{
		Subject:  subject,
		HTMLBody: wrapHTML(heading, body, "Top up wallet", link, dashboardURL),
		TextBody: textVersion(subject,
			"Your Ironflyer wallet balance is "+balance+", below the "+threshold+" alert threshold. Paid executions refuse with 402 once balance hits zero.",
			link),
	}
}

// renderWeeklyDigest is the opt-in summary template for the Sunday
// digest. Body renders a 6-row table using the same receipt-table
// pattern as renderReceipt so the visual is consistent.
func renderWeeklyDigest(name, periodLabel string, counts WeeklyDigestCounts, dashboardURL string) EmailContent {
	subject := fmt.Sprintf("Your Ironflyer week: %d runs, %d deploys", counts.Runs, counts.Deploys)
	heading := "Your Ironflyer week"
	greeting := ""
	if n := strings.TrimSpace(name); n != "" {
		greeting = `<p style="margin:0 0 16px 0;">Hi ` + escapeHTML(n) + `,</p>`
	}
	period := strings.TrimSpace(periodLabel)
	if period == "" {
		period = "the past 7 days"
	}
	row := func(label, value string) string {
		return `<tr>` +
			`<td style="padding:8px 12px;border-bottom:1px solid rgba(178,133,255,0.16);font-size:13px;color:#777096;width:60%;">` + label + `</td>` +
			`<td style="padding:8px 12px;border-bottom:1px solid rgba(178,133,255,0.16);font-size:14px;color:#f7f4ff;font-family:'SFMono-Regular','Menlo','Consolas',monospace;">` + escapeHTML(value) + `</td>` +
			`</tr>`
	}
	currency := counts.Currency
	if currency == "" {
		currency = "USD"
	}
	table := `<table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%" style="margin:16px 0 0 0;background-color:#11132a;border:1px solid rgba(178,133,255,0.16);border-radius:8px;border-collapse:separate;">` +
		row("Period", period) +
		row("Runs", strconvInt(counts.Runs)) +
		row("Gates failed", strconvInt(counts.Gates)) +
		row("Deploys", strconvInt(counts.Deploys)) +
		row("Refusals", strconvInt(counts.Refusals)) +
		row("Spend", formatMoney(currency, counts.SpendCents)) +
		`</table>`
	body := greeting +
		`<p style="margin:0 0 16px 0;">Here's a summary of what shipped through Ironflyer this week.</p>` +
		table
	link := orFallback(dashboardURL)
	return EmailContent{
		Subject:  subject,
		HTMLBody: wrapHTML(heading, body, "Open dashboard", link, dashboardURL),
		TextBody: textVersion(subject,
			fmt.Sprintf("Period: %s. Runs: %d. Gates failed: %d. Deploys: %d. Refusals: %d. Spend: %s.",
				period, counts.Runs, counts.Gates, counts.Deploys, counts.Refusals, formatMoney(currency, counts.SpendCents)),
			link),
	}
}

// strconvInt is the integer-to-string helper the digest table reuses.
// Inlined here rather than importing strconv just for the digest path —
// keeps the file's import set tight.
func strconvInt(n int) string {
	return fmt.Sprintf("%d", n)
}

func orFallback(dashboardURL string) string {
	base := strings.TrimRight(dashboardURL, "/")
	if base == "" {
		return fallbackDashboardURL
	}
	return base
}

// wrapHTML renders the locked dark Ironflyer shell shared by every
// transactional email. Inline styles only — most clients strip <style>.
// Table-based layout (no flex/grid) for Outlook. The CTA carries a
// solid background-color fallback before the gradient so clients that
// drop linear-gradient still render a recognisable button.
func wrapHTML(heading, bodyHTML, ctaLabel, ctaURL, dashboardURL string) string {
	manageBase := strings.TrimRight(orFallback(dashboardURL), "/")
	return `<!doctype html><html dir="ltr" lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Ironflyer</title></head>
<body style="margin:0;padding:0;background-color:#050612;font-family:'Inter','Segoe UI',Arial,sans-serif;color:#b9b2d3;">
  <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%" style="background-color:#050612;padding:32px 16px;">
    <tr><td align="center">
      <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="600" style="max-width:600px;background-color:#11132a;border:1px solid rgba(178,133,255,0.16);border-radius:8px;">
        <tr><td style="padding:32px;">
          <div style="font-family:'SFMono-Regular','Menlo','Consolas',monospace;font-size:11.5px;letter-spacing:1.2px;text-transform:uppercase;color:#b56cff;margin-bottom:20px;">IRONFLYER</div>
          <h1 style="margin:0 0 20px 0;font-size:28px;line-height:1.15;font-weight:800;color:#f7f4ff;">` + heading + `</h1>
          <div style="font-size:15px;line-height:1.55;color:#b9b2d3;">` + bodyHTML + `</div>
          <table role="presentation" cellpadding="0" cellspacing="0" border="0" style="margin:28px 0 0 0;">
            <tr><td align="center" bgcolor="#b56cff" style="background-color:#b56cff;background-image:linear-gradient(100deg,#ff7848 0%,#e149c9 52%,#8f4dff 100%);border-radius:8px;">
              <a href="` + escapeAttr(ctaURL) + `" target="_blank" style="display:inline-block;padding:14px 28px;font-size:15px;font-weight:700;color:#ffffff;text-decoration:none;border-radius:8px;font-family:'Inter','Segoe UI',Arial,sans-serif;">` + ctaLabel + `</a>
            </td></tr>
          </table>
        </td></tr>
        <tr><td style="padding:0 32px 28px 32px;border-top:1px solid rgba(178,133,255,0.16);">
          <p style="margin:24px 0 0 0;font-size:11px;line-height:1.5;color:#777096;">Sent by Ironflyer. Manage notifications: ` + escapeHTML(manageBase) + `/settings/notifications</p>
        </td></tr>
      </table>
    </td></tr>
  </table>
</body></html>`
}

// textVersion renders a plaintext fallback. Ends with the canonical link.
func textVersion(subject, body, link string) string {
	parts := []string{subject, "", body}
	if link != "" {
		parts = append(parts, "", link)
	}
	parts = append(parts, "", "— Ironflyer")
	return strings.Join(parts, "\n")
}

// humanizeTTL formats a duration as the shortest sensible human string
// (e.g. "30 minutes", "1 hour"). Anything under a minute falls back to
// seconds; days only kick in past 48h.
func humanizeTTL(d time.Duration) string {
	if d <= 0 {
		return "a few minutes"
	}
	if d < time.Minute {
		return pluralize(int(d.Seconds()), "second")
	}
	if d < time.Hour {
		return pluralize(int(d.Minutes()), "minute")
	}
	if d < 48*time.Hour {
		return pluralize(int(d.Hours()), "hour")
	}
	return pluralize(int(d.Hours()/24), "day")
}

func pluralize(n int, unit string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, unit)
	}
	return fmt.Sprintf("%d %ss", n, unit)
}

// formatMoney renders cents as a currency-aware string. Falls back to
// "CUR n.nn" for unknown currencies so the receipt still parses.
func formatMoney(currency string, cents int) string {
	cur := strings.ToUpper(strings.TrimSpace(currency))
	whole := cents / 100
	frac := cents % 100
	if frac < 0 {
		frac = -frac
	}
	switch cur {
	case "USD":
		return fmt.Sprintf("$%d.%02d", whole, frac)
	case "GBP":
		return fmt.Sprintf("£%d.%02d", whole, frac)
	case "EUR":
		return fmt.Sprintf("€%d,%02d", whole, frac)
	case "ILS":
		return fmt.Sprintf("₪%d.%02d", whole, frac)
	default:
		if cur == "" {
			cur = "USD"
		}
		return fmt.Sprintf("%s %d.%02d", cur, whole, frac)
	}
}

// escapeHTML is the minimal escaper this package needs. We hand-roll
// rather than pulling html/template because each render path already
// concatenates trusted layout strings — only user-supplied fields need
// escaping, and only against the four entities that break inline body
// text inside <p>/<td>.
func escapeHTML(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
	)
	return r.Replace(s)
}

// escapeAttr escapes a string for use inside an HTML attribute value.
// Same minimal set as escapeHTML — the four entities are sufficient
// because every attribute the templates emit is double-quoted.
func escapeAttr(s string) string {
	return escapeHTML(s)
}

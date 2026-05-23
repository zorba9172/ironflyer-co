package notify

import (
	"fmt"
	"strings"
)

// dashboardURL is the destination for the CTA buttons in email bodies. We
// allow override via the Engine's DashboardURL field so staging + production
// emails link to the right place.
const fallbackDashboardURL = "https://ironflyer.dev/app"

// EmailContent is the trio every sender consumes — subject + html + text.
// Returning all three together keeps the rule engine straightforward.
type EmailContent struct {
	Subject  string
	HTMLBody string
	TextBody string
}

// renderRunComplete builds the email body for a successful project run.
func renderRunComplete(projectName, projectID, dashboardURL string) EmailContent {
	subject := fmt.Sprintf("%s run completed successfully", projectName)
	return EmailContent{
		Subject:  subject,
		HTMLBody: wrapHTML("Run complete", "Every gate passed and "+projectName+" is ready for review.", "View dashboard", projectLink(dashboardURL, projectID)),
		TextBody: textVersion(subject, projectName+" finished the run and is ready for review.", projectLink(dashboardURL, projectID)),
	}
}

// renderGateFailed builds the email body for a gate failure event.
func renderGateFailed(projectName, projectID, gateName, reason, dashboardURL string) EmailContent {
	subject := fmt.Sprintf("%s gate failed in %s", gateName, projectName)
	return EmailContent{
		Subject:  subject,
		HTMLBody: wrapHTML("Gate failed", "The "+gateName+" gate failed in "+projectName+". Reason: "+reason, "Open project", projectLink(dashboardURL, projectID)),
		TextBody: textVersion(subject, "The "+gateName+" gate failed in "+projectName+". Reason: "+reason, projectLink(dashboardURL, projectID)),
	}
}

// renderDeployDone builds the email body for a successful deploy event.
func renderDeployDone(projectName, projectID, dashboardURL string) EmailContent {
	subject := fmt.Sprintf("%s is live", projectName)
	return EmailContent{
		Subject:  subject,
		HTMLBody: wrapHTML("Deployment complete", projectName+" deployed successfully.", "Open project", projectLink(dashboardURL, projectID)),
		TextBody: textVersion(subject, projectName+" deployed successfully.", projectLink(dashboardURL, projectID)),
	}
}

// renderBudgetWarning builds the email body when the user is near their cap.
func renderBudgetWarning(projectName, dashboardURL string) EmailContent {
	subject := "Budget warning: approaching plan cap"
	return EmailContent{
		Subject:  subject,
		HTMLBody: wrapHTML("Budget attention needed", "Usage for "+projectName+" is approaching the plan cap.", "Open budget", strings.TrimRight(dashboardURL, "/")+"/billing"),
		TextBody: textVersion(subject, "Usage for "+projectName+" is approaching the plan cap.", strings.TrimRight(dashboardURL, "/")+"/billing"),
	}
}

// renderWebhookDisabled is sent when a webhook is auto-disabled after
// repeated failures. It is the only non-event-driven email the engine sends.
func renderWebhookDisabled(webhookURL, dashboardURL string, failures int) EmailContent {
	subject := "Webhook disabled automatically"
	return EmailContent{
		Subject:  subject,
		HTMLBody: wrapHTML("Webhook failed too many times", fmt.Sprintf("Webhook %s failed %d times in a row and was disabled automatically. You can re-enable it after fixing the endpoint.", webhookURL, failures), "Manage webhooks", strings.TrimRight(dashboardURL, "/")+"/settings/notifications"),
		TextBody: textVersion(subject, fmt.Sprintf("Webhook %s was disabled after %d failures.", webhookURL, failures), strings.TrimRight(dashboardURL, "/")+"/settings/notifications"),
	}
}

// projectLink composes the URL the email CTA points at. We pick a project
// when one is known so the user lands on the actionable screen, not the
// generic dashboard.
func projectLink(dashboardURL, projectID string) string {
	base := strings.TrimRight(dashboardURL, "/")
	if base == "" {
		base = fallbackDashboardURL
	}
	if projectID == "" {
		return base
	}
	return base + "/projects/" + projectID
}

// wrapHTML renders the alabaster-and-lime branded shell shared by every
// transactional email. Inline styles are required because most clients
// strip <style> tags.
func wrapHTML(heading, body, ctaLabel, ctaURL string) string {
	return `<!doctype html><html dir="ltr" lang="en"><head><meta charset="utf-8"></head>
<body style="margin:0;padding:0;background:#F6F4EE;font-family:'Inter',Arial,sans-serif;color:#111;">
  <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%" style="padding:32px 0;">
    <tr><td align="center">
      <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="560" style="background:#FFFFFF;border-radius:18px;overflow:hidden;border:1px solid rgba(17,17,17,0.08);">
        <tr><td style="background:linear-gradient(135deg,#CFEF38 0%,#F6F4EE 100%);padding:28px 32px;">
          <div style="font-weight:900;font-size:20px;letter-spacing:-0.02em;">Ironflyer</div>
          <div style="font-weight:800;font-size:26px;margin-top:8px;">` + heading + `</div>
        </td></tr>
        <tr><td style="padding:28px 32px 8px 32px;font-size:15px;line-height:1.6;">` + body + `</td></tr>
        <tr><td style="padding:8px 32px 32px 32px;">
          <a href="` + ctaURL + `" style="display:inline-block;padding:12px 22px;background:#111;color:#CFEF38;border-radius:999px;font-weight:800;text-decoration:none;">` + ctaLabel + `</a>
        </td></tr>
        <tr><td style="padding:0 32px 28px 32px;font-size:12px;color:rgba(17,17,17,0.55);">
          You received this email because notifications are enabled for your Ironflyer account. You can turn them off at any time from Settings → Notifications.
        </td></tr>
      </table>
    </td></tr>
  </table>
</body></html>`
}

// textVersion renders a plain-text fallback so spam scores stay sane and
// terminal email clients still get a useful message.
func textVersion(subject, body, ctaURL string) string {
	return subject + "\n\n" + body + "\n\n" + ctaURL + "\n\nIronflyer"
}

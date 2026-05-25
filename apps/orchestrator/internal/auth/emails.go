// Package auth — transactional email templates.
//
// Plain, builder-facing copy. No orbs, no testimonials, no "magic".
// Match the Ironflyer house tone from CLAUDE.md: precise, senior,
// direct. Templates produce parallel HTML + text bodies — every
// receiving inbox renders one or the other.
//
// Each template here is a Go text/template; the resolver materializes
// the right data struct and hands the rendered bodies to the existing
// notify.EmailSender (Resend/SendGrid/Noop). Subject lines stay short
// so they don't truncate in inbox previews.
package auth

import (
	"bytes"
	"context"
	"fmt"
	"text/template"

	"ironflyer/apps/orchestrator/internal/notify"
)

// EmailKind enumerates the templates we ship.
type EmailKind string

const (
	EmailWelcome           EmailKind = "welcome"
	EmailVerification      EmailKind = "verification"
	EmailPasswordReset     EmailKind = "password_reset"
	EmailEmailChange       EmailKind = "email_change"
	EmailMfaEnabled        EmailKind = "mfa_enabled"
	EmailMfaRecoveryUsed   EmailKind = "mfa_recovery_used"
)

// EmailContext is the union of fields every template can reference.
// Empty fields render as empty strings; templates are conditional on
// presence where needed.
type EmailContext struct {
	UserName     string
	UserEmail    string
	VerifyURL    string
	ResetURL     string
	NewEmail     string
	BrandName    string
	SupportEmail string
}

// emailTemplates is the registry every Send call resolves. Each entry
// is {subject, htmlBody, textBody}.
type emailTemplates struct {
	Subject string
	HTML    string
	Text    string
}

var emailRegistry = map[EmailKind]emailTemplates{
	EmailWelcome: {
		Subject: "Welcome to Ironflyer",
		HTML: `<p>Hello {{.UserName}},</p>
<p>Your Ironflyer account is ready. Ironflyer ships software through enforced gates — spec, UX, architecture, code, lint, tests, security, deploy. Each one blocks until it passes.</p>
<p>Next step: verify your email so paid plans and deploys unlock.</p>
<p><a href="{{.VerifyURL}}">{{.VerifyURL}}</a></p>
<p>— The Ironflyer team</p>`,
		Text: `Hello {{.UserName}},

Your Ironflyer account is ready. Ironflyer ships software through enforced gates — spec, UX, architecture, code, lint, tests, security, deploy. Each one blocks until it passes.

Next step: verify your email so paid plans and deploys unlock.
{{.VerifyURL}}

— The Ironflyer team`,
	},
	EmailVerification: {
		Subject: "Verify your Ironflyer email",
		HTML: `<p>Hello {{.UserName}},</p>
<p>Confirm this address by visiting the link below. The link expires in 48 hours.</p>
<p><a href="{{.VerifyURL}}">{{.VerifyURL}}</a></p>
<p>If you did not sign up, ignore this email — nothing changes on your account.</p>`,
		Text: `Hello {{.UserName}},

Confirm this address by visiting the link below. The link expires in 48 hours.
{{.VerifyURL}}

If you did not sign up, ignore this email — nothing changes on your account.`,
	},
	EmailPasswordReset: {
		Subject: "Reset your Ironflyer password",
		HTML: `<p>Hello {{.UserName}},</p>
<p>Someone requested a password reset for {{.UserEmail}}. If that was you, set a new password using the link below. The link expires in 1 hour.</p>
<p><a href="{{.ResetURL}}">{{.ResetURL}}</a></p>
<p>If it was not you, ignore this email — the existing password keeps working.</p>`,
		Text: `Hello {{.UserName}},

Someone requested a password reset for {{.UserEmail}}. If that was you, set a new password using the link below. The link expires in 1 hour.
{{.ResetURL}}

If it was not you, ignore this email — the existing password keeps working.`,
	},
	EmailEmailChange: {
		Subject: "Confirm your new Ironflyer email",
		HTML: `<p>Hello {{.UserName}},</p>
<p>You asked to change the email on your Ironflyer account to <strong>{{.NewEmail}}</strong>. Confirm by visiting the link below. The link expires in 48 hours.</p>
<p><a href="{{.VerifyURL}}">{{.VerifyURL}}</a></p>
<p>If you did not request this change, ignore the email — the account stays on its existing address.</p>`,
		Text: `Hello {{.UserName}},

You asked to change the email on your Ironflyer account to {{.NewEmail}}. Confirm by visiting the link below. The link expires in 48 hours.
{{.VerifyURL}}

If you did not request this change, ignore the email — the account stays on its existing address.`,
	},
	EmailMfaEnabled: {
		Subject: "Two-factor authentication enabled",
		HTML: `<p>Hello {{.UserName}},</p>
<p>Two-factor authentication is now active on your Ironflyer account. Future sign-ins will require a code from your authenticator app.</p>
<p>If this was not you, change your password immediately and contact support.</p>`,
		Text: `Hello {{.UserName}},

Two-factor authentication is now active on your Ironflyer account. Future sign-ins will require a code from your authenticator app.

If this was not you, change your password immediately and contact support.`,
	},
	EmailMfaRecoveryUsed: {
		Subject: "Ironflyer recovery code used",
		HTML: `<p>Hello {{.UserName}},</p>
<p>A recovery code was just used to sign in to your Ironflyer account. If that was not you, change your password and regenerate recovery codes from the security settings.</p>`,
		Text: `Hello {{.UserName}},

A recovery code was just used to sign in to your Ironflyer account. If that was not you, change your password and regenerate recovery codes from the security settings.`,
	},
}

// Render returns (subject, html, text) for the supplied template + context.
func Render(kind EmailKind, ctx EmailContext) (string, string, string, error) {
	tpl, ok := emailRegistry[kind]
	if !ok {
		return "", "", "", fmt.Errorf("unknown email template %q", kind)
	}
	if ctx.BrandName == "" {
		ctx.BrandName = "Ironflyer"
	}
	subj := tpl.Subject
	html, err := renderTemplate(tpl.HTML, ctx)
	if err != nil {
		return "", "", "", err
	}
	text, err := renderTemplate(tpl.Text, ctx)
	if err != nil {
		return "", "", "", err
	}
	return subj, html, text, nil
}

func renderTemplate(src string, ctx EmailContext) (string, error) {
	t, err := template.New("email").Parse(src)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, ctx); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// SendEmail is the convenience wrapper resolvers call. Renders the
// template and dispatches via the supplied notify.EmailSender. When the
// sender is nil (operator hasn't wired one) we return nil so the
// surrounding flow still completes — the resolver decides whether that
// is an error.
func SendEmail(ctx context.Context, sender notify.EmailSender, to string, kind EmailKind, data EmailContext) error {
	if sender == nil {
		return nil
	}
	subj, html, text, err := Render(kind, data)
	if err != nil {
		return err
	}
	return sender.Send(ctx, to, subj, html, text)
}

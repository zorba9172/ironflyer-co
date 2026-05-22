package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/rs/zerolog"
)

// EmailSender is the single interface call sites depend on. Provider-specific
// implementations live in this file (Resend, SendGrid, Noop) so swapping
// vendors is one constructor call in main.go.
type EmailSender interface {
	Send(ctx context.Context, to, subject, htmlBody, textBody string) error
}

// senderConfig captures the shared fields each provider needs.
type senderConfig struct {
	apiKey   string
	from     string
	client   *http.Client
	logger   zerolog.Logger
}

func newSenderConfig(apiKey, from string, logger zerolog.Logger) senderConfig {
	return senderConfig{
		apiKey: apiKey,
		from:   from,
		client: &http.Client{Timeout: 15 * time.Second},
		logger: logger,
	}
}

// NoopSender is the fallback when no provider is configured. It logs the
// intent at info level so dev/test runs can still observe what would have
// been sent.
type NoopSender struct {
	logger zerolog.Logger
}

// NewNoopSender returns a sender that only logs.
func NewNoopSender(logger zerolog.Logger) *NoopSender {
	return &NoopSender{logger: logger}
}

// Send logs the email and returns nil. Never errors.
func (n *NoopSender) Send(_ context.Context, to, subject, _, textBody string) error {
	n.logger.Info().
		Str("to", to).
		Str("subject", subject).
		Int("text_bytes", len(textBody)).
		Msg("notify: would send email (noop sender)")
	return nil
}

// ResendSender posts to https://api.resend.com/emails. Resend takes a single
// JSON document with from / to / subject / html / text fields.
type ResendSender struct{ cfg senderConfig }

// NewResendSender constructs a Resend-backed sender.
func NewResendSender(apiKey, from string, logger zerolog.Logger) *ResendSender {
	return &ResendSender{cfg: newSenderConfig(apiKey, from, logger)}
}

// Send delivers a single email via the Resend API.
func (s *ResendSender) Send(ctx context.Context, to, subject, htmlBody, textBody string) error {
	if s.cfg.apiKey == "" {
		return errors.New("resend: missing API key")
	}
	body := map[string]any{
		"from":    s.cfg.from,
		"to":      []string{to},
		"subject": subject,
		"html":    htmlBody,
		"text":    textBody,
	}
	return postJSON(ctx, s.cfg, "https://api.resend.com/emails", body,
		func(req *http.Request) {
			req.Header.Set("Authorization", "Bearer "+s.cfg.apiKey)
		})
}

// SendgridSender posts to https://api.sendgrid.com/v3/mail/send. Their schema
// is nested (personalizations + content array), so we construct it inline.
type SendgridSender struct{ cfg senderConfig }

// NewSendgridSender constructs a SendGrid-backed sender.
func NewSendgridSender(apiKey, from string, logger zerolog.Logger) *SendgridSender {
	return &SendgridSender{cfg: newSenderConfig(apiKey, from, logger)}
}

// Send delivers a single email via the SendGrid v3 API.
func (s *SendgridSender) Send(ctx context.Context, to, subject, htmlBody, textBody string) error {
	if s.cfg.apiKey == "" {
		return errors.New("sendgrid: missing API key")
	}
	body := map[string]any{
		"personalizations": []map[string]any{
			{"to": []map[string]string{{"email": to}}, "subject": subject},
		},
		"from": map[string]string{"email": s.cfg.from},
		"content": []map[string]string{
			{"type": "text/plain", "value": textBody},
			{"type": "text/html", "value": htmlBody},
		},
	}
	return postJSON(ctx, s.cfg, "https://api.sendgrid.com/v3/mail/send", body,
		func(req *http.Request) {
			req.Header.Set("Authorization", "Bearer "+s.cfg.apiKey)
		})
}

// postJSON marshals body, posts to url, and treats any non-2xx as an error.
// The headerFn hook lets each provider inject its bespoke auth header.
func postJSON(ctx context.Context, cfg senderConfig, url string, body any, headerFn func(*http.Request)) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if headerFn != nil {
		headerFn(req)
	}
	resp, err := cfg.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	buf, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return fmt.Errorf("email provider returned %d: %s", resp.StatusCode, string(buf))
}

// SenderFromEnv inspects EMAIL_PROVIDER / EMAIL_API_KEY / EMAIL_FROM and
// returns the best-fit sender. Unknown / blank provider yields a Noop so
// the orchestrator never refuses to start on a misconfiguration.
func SenderFromEnv(provider, apiKey, from string, logger zerolog.Logger) EmailSender {
	switch provider {
	case "resend":
		if apiKey == "" || from == "" {
			logger.Warn().Msg("notify: EMAIL_PROVIDER=resend but key/from missing — using noop")
			return NewNoopSender(logger)
		}
		return NewResendSender(apiKey, from, logger)
	case "sendgrid":
		if apiKey == "" || from == "" {
			logger.Warn().Msg("notify: EMAIL_PROVIDER=sendgrid but key/from missing — using noop")
			return NewNoopSender(logger)
		}
		return NewSendgridSender(apiKey, from, logger)
	default:
		return NewNoopSender(logger)
	}
}

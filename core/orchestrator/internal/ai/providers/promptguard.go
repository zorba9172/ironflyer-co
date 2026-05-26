package providers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"

	"github.com/rs/zerolog"
)

// ErrPromptRefused is returned by PromptGuard.Inspect / the BillingGuard
// entry points when a request carries user-sourced content that the
// guard refuses to forward. Callers can errors.Is against this sentinel
// to surface a 4xx instead of a generic provider failure.
var ErrPromptRefused = errors.New("prompt refused by promptguard")

type PromptGuardConfig struct {
	MaxUserCharsPerMessage int
	MaxTotalRequestChars   int
	BlockMode              bool
}

func (c PromptGuardConfig) withDefaults() PromptGuardConfig {
	if c.MaxUserCharsPerMessage <= 0 {
		c.MaxUserCharsPerMessage = 100_000
	}
	if c.MaxTotalRequestChars <= 0 {
		c.MaxTotalRequestChars = 400_000
	}
	return c
}

type AuditFn func(ctx context.Context, kind string, details map[string]any)

type Severity string

const (
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

type Message struct {
	Role    string
	Content string
}

type Finding struct {
	Severity     Severity
	Reason       string
	MessageIndex int
	Excerpt      string
}

type Result struct {
	Sanitized []Message
	Findings  []Finding
	Refused   bool
}

type PromptGuard struct {
	cfg    PromptGuardConfig
	logger zerolog.Logger
	audit  AuditFn
}

func NewPromptGuard(cfg PromptGuardConfig, logger zerolog.Logger, auditor AuditFn) *PromptGuard {
	return &PromptGuard{cfg: cfg.withDefaults(), logger: logger, audit: auditor}
}

var (
	reRoleHijack   = regexp.MustCompile(`(?im)^\s*(system|assistant|developer)\s*:`)
	reRevealPrompt = regexp.MustCompile(`(?i)(reveal\s+(your\s+)?(system|hidden|secret)\s+prompt|print\s+your\s+(instructions|system\s+message)|what\s+(are|were)\s+your\s+(original\s+)?instructions)`)
	reExfilEnv     = regexp.MustCompile(`[A-Z_]{4,}(KEY|TOKEN|SECRET|PASSWORD)`)
	reExfilSend    = regexp.MustCompile(`(?i)(send\s+to\s+https?://|post\s+to\s+https?://)`)
	reZeroWidth    = regexp.MustCompile(`[\x{200B}-\x{200F}\x{202A}-\x{202E}\x{2060}-\x{206F}\x{FEFF}]`)
)

var literalInjectionMarkers = []string{
	"</system>", "</assistant>", "</developer>",
	"<|im_start|>system", "<|im_end|>",
	"<<SYS>>", "[INST]", "[/INST]",
}

var ignorePhrases = []string{
	"ignore previous instructions",
	"disregard the above",
	"forget everything",
	"override the system prompt",
}

const (
	untrustedOpen  = "<<<UNTRUSTED_USER_INPUT\n"
	untrustedClose = "\nUNTRUSTED_USER_INPUT>>>"
	truncSeparator = "\n… [truncated] …\n"
	systemMarker   = "### SYSTEM"
)

// Inspect scans a slice of user-sourced messages, returns the sanitized
// slice plus the findings. When BlockMode is true and any HIGH/CRITICAL
// finding fires, Refused is set and Sanitized mirrors the original
// (caller MUST honour Refused before forwarding).
func (g *PromptGuard) Inspect(ctx context.Context, messages []Message) (Result, error) {
	if g == nil {
		return Result{Sanitized: messages}, nil
	}
	res := Result{Sanitized: make([]Message, len(messages))}
	copy(res.Sanitized, messages)

	total := 0
	for _, m := range messages {
		total += len(m.Content)
	}
	if total > g.cfg.MaxTotalRequestChars {
		f := Finding{
			Severity: SeverityMedium,
			Reason:   "total_request_too_long",
			Excerpt:  "",
		}
		res.Findings = append(res.Findings, f)
		res.Refused = true
		g.emit(ctx, f)
		return res, nil
	}

	lowBatch := 0
	for i := range res.Sanitized {
		msg := &res.Sanitized[i]
		if reZeroWidth.MatchString(msg.Content) {
			msg.Content = reZeroWidth.ReplaceAllString(msg.Content, "")
			lowBatch++
		}
		if msg.Role != "user" {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(msg.Content), systemMarker) {
			f := Finding{Severity: SeverityCritical, Reason: "user_message_starts_with_system_marker", MessageIndex: i, Excerpt: head(msg.Content, 160)}
			res.Findings = append(res.Findings, f)
			g.emit(ctx, f)
			if g.cfg.BlockMode {
				res.Refused = true
				continue
			}
			msg.Content = wrap(msg.Content)
			continue
		}

		findings := g.scan(msg.Content, i)
		blocked := false
		for _, f := range findings {
			res.Findings = append(res.Findings, f)
			g.emit(ctx, f)
			if f.Severity == SeverityHigh || f.Severity == SeverityCritical {
				blocked = true
			}
		}
		if blocked {
			if g.cfg.BlockMode {
				res.Refused = true
				continue
			}
			msg.Content = wrap(msg.Content)
		}

		if n := len(msg.Content); n > g.cfg.MaxUserCharsPerMessage {
			f := Finding{Severity: SeverityMedium, Reason: "message_too_long", MessageIndex: i, Excerpt: ""}
			res.Findings = append(res.Findings, f)
			g.emit(ctx, f)
			msg.Content = truncateMiddle(msg.Content, g.cfg.MaxUserCharsPerMessage)
		}
	}

	if lowBatch > 0 && g.audit != nil {
		g.audit(ctx, "promptguard.finding", map[string]any{
			"severity": string(SeverityLow),
			"reason":   "zero_width_unicode_stripped",
			"count":    lowBatch,
		})
	}

	return res, nil
}

// InspectRequest adapts the canonical providers.Request into the
// Message-slice contract Inspect operates on. System content is tagged
// system-role so it bypasses the user-only checks; Prompt content is
// the user-sourced payload. Returns the mutated request with sanitized
// fields and the result.
func (g *PromptGuard) InspectRequest(ctx context.Context, req Request) (Request, Result, error) {
	if g == nil {
		return req, Result{}, nil
	}
	msgs := []Message{{Role: "system", Content: req.System}, {Role: "user", Content: req.Prompt}}
	res, err := g.Inspect(ctx, msgs)
	if err != nil || res.Refused {
		return req, res, err
	}
	req.System = res.Sanitized[0].Content
	req.Prompt = res.Sanitized[1].Content
	return req, res, nil
}

func (g *PromptGuard) scan(content string, idx int) []Finding {
	var out []Finding
	if m := reRoleHijack.FindStringIndex(content); m != nil {
		out = append(out, Finding{Severity: SeverityCritical, Reason: "user_message_claims_non_user_role", MessageIndex: idx, Excerpt: head(content[m[0]:], 160)})
	}
	for _, marker := range literalInjectionMarkers {
		if strings.Contains(content, marker) {
			out = append(out, Finding{Severity: SeverityCritical, Reason: "literal_injection_marker:" + marker, MessageIndex: idx, Excerpt: marker})
			break
		}
	}
	lc := strings.ToLower(content)
	for _, phrase := range ignorePhrases {
		if strings.Contains(lc, phrase) {
			out = append(out, Finding{Severity: SeverityCritical, Reason: "instruction_to_ignore_prior_context", MessageIndex: idx, Excerpt: phrase})
			break
		}
	}
	if reRevealPrompt.MatchString(content) {
		out = append(out, Finding{Severity: SeverityHigh, Reason: "asks_to_reveal_system_prompt", MessageIndex: idx, Excerpt: head(content, 160)})
	}
	if strings.Contains(content, "Project.Secrets") {
		out = append(out, Finding{Severity: SeverityHigh, Reason: "exfiltration_target:project_secrets", MessageIndex: idx, Excerpt: "Project.Secrets"})
	} else if m := reExfilEnv.FindString(content); m != "" {
		out = append(out, Finding{Severity: SeverityHigh, Reason: "exfiltration_target:env_var", MessageIndex: idx, Excerpt: m})
	}
	if reExfilSend.MatchString(content) {
		out = append(out, Finding{Severity: SeverityHigh, Reason: "exfiltration_instruction", MessageIndex: idx, Excerpt: head(content, 160)})
	}
	return out
}

func (g *PromptGuard) emit(ctx context.Context, f Finding) {
	if g == nil {
		return
	}
	g.logger.Warn().
		Str("severity", string(f.Severity)).
		Str("reason", f.Reason).
		Int("message_index", f.MessageIndex).
		Msg("promptguard: finding")
	if g.audit == nil {
		return
	}
	h := sha256.Sum256([]byte(f.Excerpt))
	g.audit(ctx, "promptguard.finding", map[string]any{
		"severity":       string(f.Severity),
		"reason":         f.Reason,
		"message_index":  f.MessageIndex,
		"excerpt_sha256": hex.EncodeToString(h[:]),
	})
}

func head(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func wrap(s string) string {
	return untrustedOpen + s + untrustedClose
}

// truncateMiddle keeps the first 60% and last 40% of the budget,
// joined by a visible marker. Preserves head context (where the user
// usually states the task) and tail (where they often append the
// actual question after a long paste).
func truncateMiddle(s string, budget int) string {
	if len(s) <= budget {
		return s
	}
	sep := truncSeparator
	usable := budget - len(sep)
	if usable < 64 {
		return s[:budget]
	}
	headN := usable * 60 / 100
	tailN := usable - headN
	return s[:headN] + sep + s[len(s)-tailN:]
}

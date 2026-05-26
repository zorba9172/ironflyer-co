package audit

import (
	"regexp"
	"strings"
	"sync/atomic"
)

// redactionEnabled is the process-wide toggle; SetRedactionEnabled is
// called once by main.go after config load. Default ON (1) so deployments
// that never opt in still benefit from the masking pass.
var redactionEnabled int32 = 1

// SetRedactionEnabled toggles audit PII redaction. Pass true to mask
// emails, IP addresses, and provider keys before each entry is hashed
// and stored; pass false (IRONFLYER_AUDIT_REDACT=off) to disable.
func SetRedactionEnabled(on bool) {
	if on {
		atomic.StoreInt32(&redactionEnabled, 1)
	} else {
		atomic.StoreInt32(&redactionEnabled, 0)
	}
}

func redactionActive() bool { return atomic.LoadInt32(&redactionEnabled) == 1 }

var (
	emailRE = regexp.MustCompile(`([A-Za-z0-9._%+\-]+)@([A-Za-z0-9.\-]+\.[A-Za-z]{2,})`)
	ipv4RE  = regexp.MustCompile(`\b(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})\b`)

	// Provider keys — kept aligned with appsec/secrets.go.
	providerKeyREs = []*regexp.Regexp{
		regexp.MustCompile(`\bsk-ant-[A-Za-z0-9_-]{20,}\b`),
		regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{20,}\b`),
		regexp.MustCompile(`\bsk_(live|test)_[0-9a-zA-Z]{20,}\b`),
		regexp.MustCompile(`\brk_(live|test)_[0-9a-zA-Z]{20,}\b`),
		regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`),
		regexp.MustCompile(`\bghp_[0-9A-Za-z]{36}\b`),
		regexp.MustCompile(`\bgho_[0-9A-Za-z]{36}\b`),
		regexp.MustCompile(`\bgithub_pat_[0-9A-Za-z_]{82}\b`),
		regexp.MustCompile(`\bAIza[0-9A-Za-z\-_]{35}\b`),
		regexp.MustCompile(`\bxox[abprs]-[0-9A-Za-z-]{10,}\b`),
	}
)

// redactEntry mutates e so that the canonical JSON hashed by hashEntry
// no longer carries raw PII. Summary + Attrs string values are scanned;
// structural fields (ids, hashes, action) are left intact. Safe to call
// on a zero-redaction config — the function short-circuits.
func redactEntry(e *Entry) {
	if !redactionActive() || e == nil {
		return
	}
	if e.Summary != "" {
		e.Summary = redactString(e.Summary)
	}
	for k, v := range e.Attrs {
		e.Attrs[k] = redactValue(v)
	}
}

func redactValue(v any) any {
	switch t := v.(type) {
	case string:
		return redactString(t)
	case []any:
		for i, item := range t {
			t[i] = redactValue(item)
		}
		return t
	case map[string]any:
		for k, item := range t {
			t[k] = redactValue(item)
		}
		return t
	default:
		return v
	}
}

func redactString(s string) string {
	if s == "" {
		return s
	}
	for _, re := range providerKeyREs {
		s = re.ReplaceAllString(s, "[REDACTED:key]")
	}
	s = emailRE.ReplaceAllStringFunc(s, func(m string) string {
		at := strings.LastIndex(m, "@")
		if at < 0 {
			return "[REDACTED:email]"
		}
		return "[REDACTED]@" + m[at+1:]
	})
	s = ipv4RE.ReplaceAllString(s, "$1.$2.$3.x")
	return s
}

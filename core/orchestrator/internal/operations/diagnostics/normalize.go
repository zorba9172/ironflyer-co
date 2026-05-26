package diagnostics

import (
	"regexp"
	"strings"
)

const classMaxLen = 80

var (
	// uuidPattern collapses any 8-4-4-4-12 hex token (case-insensitive).
	uuidPattern = regexp.MustCompile(`(?i)\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b`)
	// hex32Pattern matches any 16+ char run of hex (digest / hash style).
	hex32Pattern = regexp.MustCompile(`(?i)\b[0-9a-f]{16,}\b`)
	// numberPattern collapses any 3+ digit integer / decimal run so
	// "ledger entry 4827 failed" aggregates with "ledger entry 9914
	// failed". 3 digits is the threshold so we keep small counts (e.g.
	// HTTP status "404") readable while still collapsing IDs and ports.
	numberPattern = regexp.MustCompile(`\b\d{3,}(?:\.\d+)?\b`)
	// whitespacePattern collapses any run of whitespace to a single
	// space so a multi-line panic doesn't blow the class out.
	whitespacePattern = regexp.MustCompile(`\s+`)
)

// NormalizeMessage turns a raw log message into an aggregation class.
// The rule, in order:
//
//  1. Lowercase the string.
//  2. Replace every UUID with `<uuid>`.
//  3. Replace every 16+ char hex run with `<hex>`.
//  4. Replace every 3+ digit number with `<num>`.
//  5. Collapse whitespace runs to a single space and trim ends.
//  6. Truncate to classMaxLen (80) characters.
//
// The truncation keeps the aggregator stable when downstream errors
// concatenate a wall of provider detail at the tail.
func NormalizeMessage(msg string) string {
	if msg == "" {
		return "(empty)"
	}
	out := strings.ToLower(msg)
	out = uuidPattern.ReplaceAllString(out, "<uuid>")
	out = hex32Pattern.ReplaceAllString(out, "<hex>")
	out = numberPattern.ReplaceAllString(out, "<num>")
	out = whitespacePattern.ReplaceAllString(out, " ")
	out = strings.TrimSpace(out)
	if len(out) > classMaxLen {
		out = out[:classMaxLen]
	}
	if out == "" {
		return "(empty)"
	}
	return out
}

// Package repair implements the V22 repair genome and patch memory.
//
// Repair genome: failure_signature → known fix recipe. The genome
// learns which fix shape recovered the build/gate so the next
// occurrence of the same failure short-circuits the expensive
// reasoning loop. Per the V22 proof pack, this is one of the
// "reuse_repair" levers ProfitGuard pulls to keep margin positive
// under repeated-class failures.
//
// Patch memory: past patches keyed by an intent signature (prompt +
// gate context). Lets the finisher rank or re-apply a known patch
// when the same intent recurs.
//
// Signatures are deterministic SHA-256 hex strings of normalised
// inputs so the same failure / intent always maps to the same key
// across processes.
package repair

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"unicode"
)

// FailureSignature normalises a raw failure string (compiler output,
// gate verdict, stack trace, etc.) into a stable SHA-256 hex key.
// Normalisation: lowercase, collapse whitespace, trim. We deliberately
// keep punctuation and tokens intact — the signature is a fingerprint
// of the failure class, not of its phrasing variants.
func FailureSignature(raw string) string {
	return sha256Hex(normalise(raw))
}

// IntentSignature derives a stable SHA-256 hex key for a (prompt,
// gates) intent. Used as the lookup key in patch_memory.
func IntentSignature(prompt, gates string) string {
	return sha256Hex(normalise(prompt) + "\x00" + normalise(gates))
}

// normalise lowercases the input and collapses every run of whitespace
// into a single space, then trims surrounding whitespace.
func normalise(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	b.Grow(len(s))
	inWS := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !inWS {
				b.WriteByte(' ')
				inWS = true
			}
			continue
		}
		b.WriteRune(r)
		inWS = false
	}
	return strings.TrimSpace(b.String())
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

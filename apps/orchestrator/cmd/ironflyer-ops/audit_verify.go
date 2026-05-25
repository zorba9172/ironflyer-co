package main

import (
	"context"
	"flag"
	"fmt"
	"time"
)

// runAuditVerify is the operator hand-pull on the audit chain.
//
// The orchestrator's audit store today is either an in-process ring
// buffer (audit.MemoryStore — process-local, cannot be walked by an
// out-of-band CLI) or a SurrealDB-backed store
// (audit.SurrealStore — requires the same Surreal connection the
// orchestrator pod is using). Neither shape is reachable from the
// pg-only CLI surface.
//
// The honest behaviour here is to surface that limitation explicitly
// and tell the operator which endpoint to hit instead — the GraphQL
// `verifyAudit` query (mounted at `/graphql`) re-uses the live audit
// store inside the orchestrator process and is the canonical way to
// produce a chain proof. We still echo the supplied --since / --until
// so the operator can paste them straight into the GraphQL payload.
func runAuditVerify(parent context.Context, args []string) error {
	fs := flag.NewFlagSet("audit verify", flag.ContinueOnError)
	var (
		sinceStr string
		untilStr string
	)
	fs.StringVar(&sinceStr, "since", "", "ISO8601 lower bound (inclusive)")
	fs.StringVar(&untilStr, "until", "", "ISO8601 upper bound (inclusive)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if sinceStr == "" {
		return &usageError{msg: "audit verify: --since is required (ISO8601, e.g. 2026-05-24T00:00:00Z)"}
	}
	since, err := time.Parse(time.RFC3339, sinceStr)
	if err != nil {
		return fmt.Errorf("parse --since: %w", err)
	}
	var until time.Time
	if untilStr != "" {
		until, err = time.Parse(time.RFC3339, untilStr)
		if err != nil {
			return fmt.Errorf("parse --until: %w", err)
		}
	}
	_ = parent

	// We deliberately do not silently no-op: surface a friendly note
	// so the operator knows the CLI cannot walk the audit chain
	// directly and points them at the live GraphQL surface.
	return printJSON(map[string]any{
		"status":  "delegated",
		"since":   since,
		"until":   until,
		"message": "audit verification requires the orchestrator's live audit.Store (Memory ring buffer or Surreal). Run the GraphQL verifyAudit query against /graphql with operator credentials to walk the hash chain in-process.",
	})
}

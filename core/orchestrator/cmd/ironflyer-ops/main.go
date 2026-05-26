// ironflyer-ops is the operator-only CLI for investigating live
// orchestrator state when the GraphQL surface is misbehaving (or when
// an on-call operator wants a faster path than typing GraphQL by
// hand). Every subcommand opens a fresh pgxpool against POSTGRES_URL
// (or --dsn), calls the matching V22 service, and prints JSON.
//
// Subcommands:
//
//	ironflyer-ops approvals pending [--tenant <id>]
//	ironflyer-ops abuse score --tenant <id> [--user <id>]
//	ironflyer-ops scale snapshot
//	ironflyer-ops wallet show --tenant <id>
//	ironflyer-ops audit verify --since <iso> --until <iso>
//
// The binary itself is operator-only — it talks to the database with
// elevated credentials, so distribute it only to operators. There is
// no in-CLI auth check; that responsibility lives at the binary
// distribution layer.
package main

import (
	"context"
	"fmt"
	"os"
)

const usage = `ironflyer-ops — operator investigation CLI

Usage:
  ironflyer-ops <command> <subcommand> [flags]

Commands:
  approvals pending           List pending deploy approvals
  abuse     score             Print abuse score + tier for a tenant/user
  scale     snapshot          Print active/queued executions + sandbox capacity
  wallet    show              Print wallet balance, hold, lifetime counters
  audit     verify            Verify audit chain integrity over a time range

Common flags:
  --dsn <postgres-dsn>        Defaults to POSTGRES_URL env

Run 'ironflyer-ops <command> <subcommand> -h' for subcommand flags.
`

// dispatch is the top-level command router. We keep it plain — cobra
// is not in go.mod and the operator surface is small enough that a
// switch on os.Args[1:3] reads better than a framework here.
func main() {
	ctx := context.Background()
	if len(os.Args) < 3 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	cmd := os.Args[1]
	sub := os.Args[2]
	args := os.Args[3:]

	switch cmd {
	case "approvals":
		switch sub {
		case "pending":
			if err := runApprovalsPending(ctx, args); err != nil {
				fail(err)
			}
			return
		}
	case "abuse":
		switch sub {
		case "score":
			if err := runAbuseScore(ctx, args); err != nil {
				fail(err)
			}
			return
		}
	case "scale":
		switch sub {
		case "snapshot":
			if err := runScaleSnapshot(ctx, args); err != nil {
				fail(err)
			}
			return
		}
	case "wallet":
		switch sub {
		case "show":
			if err := runWalletShow(ctx, args); err != nil {
				fail(err)
			}
			return
		}
	case "audit":
		switch sub {
		case "verify":
			if err := runAuditVerify(ctx, args); err != nil {
				fail(err)
			}
			return
		}
	case "-h", "--help", "help":
		fmt.Print(usage)
		return
	}
	fmt.Fprintf(os.Stderr, "ironflyer-ops: unknown command: %s %s\n\n%s", cmd, sub, usage)
	os.Exit(2)
}

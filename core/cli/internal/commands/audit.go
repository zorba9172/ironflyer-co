package commands

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"strconv"

	"ironflyer/core/cli/internal/ui"
)

// auditCmd is the umbrella for the immutable audit log. The orchestrator
// exposes a hash-chained log at /audit; this command lets operators read
// + verify the chain from the terminal, which is what compliance teams
// will actually do during an attestation.
func auditCmd() *Command {
	return &Command{
		Name:  "audit",
		Short: "Query or verify the immutable audit log",
		Long: "The audit log is append-only and hash-chained. `list` filters\n" +
			"recent entries; `verify` walks the chain and reports whether it\n" +
			"has been tampered with.",
		Usage: "ironflyer audit [list|verify] [flags]",
		Subs: []*Command{
			auditListCmd(),
			auditVerifyCmd(),
		},
		Run: auditListCmd().Run,
	}
}

func auditListCmd() *Command {
	var project, action, outcome, since, until string
	var limit int
	return &Command{
		Name:  "list",
		Short: "List audit entries matching the filters",
		Usage: "ironflyer audit list [--project=ID] [--action=A] [--outcome=O] [--since=T] [--until=T] [--limit=N]",
		Examples: []string{
			"ironflyer audit list --project=my-app --outcome=blocked",
			"ironflyer audit list --action=patch.applied --limit=20",
		},
		RegFlags: func(fs *flag.FlagSet) {
			fs.StringVar(&project, "project", "", "project id scope")
			fs.StringVar(&action, "action", "", "audit action (e.g. patch.applied, gate.verdict)")
			fs.StringVar(&outcome, "outcome", "", "success | failure | blocked")
			fs.StringVar(&since, "since", "", "lower bound (RFC3339)")
			fs.StringVar(&until, "until", "", "upper bound (RFC3339)")
			fs.IntVar(&limit, "limit", 0, "max rows (server default 100, cap 1000)")
		},
		Run: func(ctx context.Context, env *Env, fs *flag.FlagSet, args []string) error {
			if err := requireAuth(env); err != nil {
				return err
			}
			params := map[string]string{
				"projectId": project,
				"action":    action,
				"outcome":   outcome,
				"since":     since,
				"until":     until,
			}
			if limit > 0 {
				params["limit"] = strconv.Itoa(limit)
			}
			resp, err := env.Client.ListAudit(ctx, params)
			if err != nil {
				return err
			}
			if env.JSON {
				b, _ := json.MarshalIndent(resp, "", "  ")
				fmt.Fprintln(env.Out, string(b))
				return nil
			}
			if len(resp.Entries) == 0 {
				fmt.Fprintln(env.Out, ui.Dim("no audit entries match the filter"))
				return nil
			}
			rows := make([][]string, 0, len(resp.Entries))
			for _, e := range resp.Entries {
				rows = append(rows, []string{
					e.CreatedAt,
					e.Action,
					colorOutcome(e.Outcome),
					trunc(e.Summary, 60),
				})
			}
			ui.RenderTable(env.Out, []string{"Time", "Action", "Outcome", "Summary"}, rows)
			fmt.Fprintln(env.Out)
			fmt.Fprintln(env.Out, ui.Dim(fmt.Sprintf("%d entr%s",
				resp.Count, plural(resp.Count, "y", "ies"))))
			return nil
		},
	}
}

func auditVerifyCmd() *Command {
	return &Command{
		Name:  "verify",
		Short: "Walk the audit log hash chain and report tamper status",
		Usage: "ironflyer audit verify [--json]",
		Run: func(ctx context.Context, env *Env, fs *flag.FlagSet, args []string) error {
			if err := requireAuth(env); err != nil {
				return err
			}
			resp, err := env.Client.VerifyAudit(ctx)
			if err != nil {
				return err
			}
			if env.JSON {
				b, _ := json.MarshalIndent(resp, "", "  ")
				fmt.Fprintln(env.Out, string(b))
				return nil
			}
			// Re-fetch a count so the success message can quote a number.
			// The verify endpoint doesn't carry one, so we issue a cheap
			// 1-row list to learn the head — that returns Count for the
			// filter window, not total entries. Fall back to omitting the
			// number if the second call fails.
			if resp.Intact {
				if list, err := env.Client.ListAudit(ctx, map[string]string{"limit": "1"}); err == nil && list.Count > 0 {
					fmt.Fprintln(env.Out, ui.Green("✓ chain intact")+ui.Dim(fmt.Sprintf(" (at least %d entr%s scanned)",
						list.Count, plural(list.Count, "y", "ies"))))
				} else {
					fmt.Fprintln(env.Out, ui.Green("✓ chain intact"))
				}
				return nil
			}
			fmt.Fprintln(env.Out, ui.Red(fmt.Sprintf("✗ chain broken at index %d", resp.FirstBadIndex)))
			return fmt.Errorf("audit chain broken at index %d", resp.FirstBadIndex)
		},
	}
}

func colorOutcome(o string) string {
	switch o {
	case "success":
		return ui.Green(o)
	case "failure":
		return ui.Red(o)
	case "blocked":
		return ui.Yellow(o)
	default:
		return o
	}
}

func plural(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}

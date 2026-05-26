package commands

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"strconv"

	"ironflyer/core/cli/internal/ui"
)

// telemetryCmd is the umbrella for /telemetry/* feeds. Right now it
// exposes the agent-call feed; future additions (cost timeline, latency
// histogram) slot in as new subcommands here.
func telemetryCmd() *Command {
	return &Command{
		Name:  "telemetry",
		Short: "Read orchestrator telemetry feeds",
		Usage: "ironflyer telemetry [agents] [flags]",
		Subs: []*Command{
			telemetryAgentsCmd(),
		},
		Run: telemetryAgentsCmd().Run,
	}
}

func telemetryAgentsCmd() *Command {
	var role, provider, model string
	var limit int
	return &Command{
		Name:  "agents",
		Short: "List recent agent calls (provider, model, tokens, cost)",
		Usage: "ironflyer telemetry agents [--limit=N] [--role=R] [--provider=P] [--model=M]",
		Examples: []string{
			"ironflyer telemetry agents --limit=20",
			"ironflyer telemetry agents --provider=anthropic --model=claude-3-5-sonnet",
		},
		RegFlags: func(fs *flag.FlagSet) {
			fs.IntVar(&limit, "limit", 0, "max rows (server default 100, cap 1000)")
			fs.StringVar(&role, "role", "", "filter by agent role")
			fs.StringVar(&provider, "provider", "", "filter by provider")
			fs.StringVar(&model, "model", "", "filter by model id")
		},
		Run: func(ctx context.Context, env *Env, fs *flag.FlagSet, args []string) error {
			if err := requireAuth(env); err != nil {
				return err
			}
			params := map[string]string{
				"role":     role,
				"provider": provider,
				"model":    model,
			}
			if limit > 0 {
				params["limit"] = strconv.Itoa(limit)
			}
			resp, err := env.Client.ListAgentTelemetry(ctx, params)
			if err != nil {
				return err
			}
			if env.JSON {
				b, _ := json.MarshalIndent(resp, "", "  ")
				fmt.Fprintln(env.Out, string(b))
				return nil
			}
			if len(resp.Calls) == 0 {
				fmt.Fprintln(env.Out, ui.Dim("no agent calls in the telemetry buffer"))
				return nil
			}
			rows := make([][]string, 0, len(resp.Calls))
			for _, c := range resp.Calls {
				rows = append(rows, []string{
					c.StartedAt,
					c.Provider,
					c.Model,
					strconv.Itoa(c.InputTokens),
					strconv.Itoa(c.OutputTokens),
					fmt.Sprintf("$%.4f", c.CostUSD),
					strconv.FormatInt(c.DurationMS, 10),
				})
			}
			ui.RenderTable(env.Out,
				[]string{"Time", "Provider", "Model", "TokensIn", "TokensOut", "Cost", "DurationMs"},
				rows)
			fmt.Fprintln(env.Out)
			fmt.Fprintln(env.Out, ui.Dim(fmt.Sprintf("%d call(s)", resp.Count)))
			return nil
		},
	}
}

package commands

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"ironflyer/apps/cli/internal/ui"
)

// deployCmd starts a deploy on the orchestrator then streams the
// deployment-specific SSE channel until it terminates.
func deployCmd() *Command {
	var provider, region string
	var envPairs stringSliceFlag
	return &Command{
		Name:  "deploy",
		Short: "Deploy a project to fly or railway and stream the build log",
		Usage: "ironflyer deploy <id> --provider fly|railway --region REGION [--env KEY=VAL]",
		Examples: []string{
			"ironflyer deploy my-project --provider fly --region iad",
			"ironflyer deploy my-project --provider railway --region us-west1 --env DATABASE_URL=...",
		},
		RegFlags: func(fs *flag.FlagSet) {
			fs.StringVar(&provider, "provider", "", "deploy target: fly | railway (required)")
			fs.StringVar(&region, "region", "", "provider region (e.g. iad, us-west1)")
			fs.Var(&envPairs, "env", "KEY=VALUE env var (repeatable)")
		},
		Run: func(ctx context.Context, env *Env, fs *flag.FlagSet, args []string) error {
			if err := requireAuth(env); err != nil {
				return err
			}
			id, err := requireProjectID(env, args)
			if err != nil {
				return err
			}
			provider = strings.ToLower(strings.TrimSpace(provider))
			if provider != "fly" && provider != "railway" {
				return fmt.Errorf("--provider must be fly or railway")
			}
			envMap := map[string]string{}
			for _, kv := range envPairs {
				if k, v, ok := strings.Cut(kv, "="); ok {
					envMap[k] = v
				}
			}
			started, err := env.Client.StartDeploy(ctx, id, provider, region, envMap)
			if err != nil {
				return err
			}
			fmt.Fprintln(env.Err, ui.Green("deploy started: ")+started.DeploymentID)
			return streamDeploy(ctx, env, started.DeploymentID)
		},
	}
}

func streamDeploy(parent context.Context, env *Env, deploymentID string) error {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	events, errs := env.Client.StreamDeployment(ctx, deploymentID)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	for {
		select {
		case <-sigCh:
			fmt.Fprintln(env.Err, ui.Dim("\ndetached. deploy continues on the orchestrator."))
			cancel()
			return nil
		case e, ok := <-events:
			if !ok {
				return nil
			}
			if env.JSON {
				fmt.Fprintln(env.Out, e.Data)
				continue
			}
			// Deploy events: kind=log|push_started|build_started|deployed|failed
			var p map[string]any
			if err := json.Unmarshal([]byte(e.Data), &p); err != nil {
				fmt.Fprintln(env.Out, e.Data)
				continue
			}
			kind := stringOr(p, "kind", "type")
			line := stringOr(p, "line", "msg", "message")
			errStr := stringOr(p, "error")
			urlStr := stringOr(p, "url", "URL")
			switch kind {
			case "deployed":
				fmt.Fprintln(env.Out, ui.Green("✓ deployed: ")+urlStr)
			case "failed":
				fmt.Fprintln(env.Out, ui.Red("✗ failed: ")+errStr)
				return fmt.Errorf("deploy failed")
			default:
				fmt.Fprintf(env.Out, "%s %s\n", ui.Dim("["+kind+"]"), line)
			}
		case err, ok := <-errs:
			if !ok {
				continue
			}
			if err != nil {
				return err
			}
		}
	}
}

// stringSliceFlag is a flag.Value that accumulates repeated string flags.
type stringSliceFlag []string

func (s *stringSliceFlag) String() string     { return strings.Join(*s, ",") }
func (s *stringSliceFlag) Set(v string) error { *s = append(*s, v); return nil }

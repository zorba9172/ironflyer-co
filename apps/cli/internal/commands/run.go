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

	"ironflyer/apps/cli/internal/client"
	"ironflyer/apps/cli/internal/ui"
)

// runCmd kicks the finisher and streams events. The finisher /run endpoint
// is synchronous (it returns a report on completion), so we run that in a
// goroutine while concurrently subscribing to /stream for live agent
// events. Ctrl+C detaches the stream without cancelling the orchestrator-
// side run — the stream's parent context cancels but /run keeps going.
func runCmd() *Command {
	return &Command{
		Name:  "run",
		Short: "Trigger the finisher engine and stream events",
		Long:  "POSTs /projects/{id}/run and concurrently subscribes to the event stream.\nPress Ctrl+C to detach the local stream — the run continues server-side.",
		Usage: "ironflyer run <id>",
		Examples: []string{
			"ironflyer run my-project",
			"ironflyer run my-project --json",
		},
		Run: func(ctx context.Context, env *Env, fs *flag.FlagSet, args []string) error {
			if err := requireAuth(env); err != nil {
				return err
			}
			id, err := requireProjectID(env, args)
			if err != nil {
				return err
			}
			return streamAndRun(ctx, env, id, true)
		},
	}
}

// logsCmd subscribes to the project's event stream without triggering a
// new run. Useful for re-attaching to an in-progress run started in
// another terminal or the web app.
func logsCmd() *Command {
	return &Command{
		Name:  "logs",
		Short: "Stream finisher events without triggering a new run",
		Usage: "ironflyer logs <id>",
		Run: func(ctx context.Context, env *Env, fs *flag.FlagSet, args []string) error {
			if err := requireAuth(env); err != nil {
				return err
			}
			id, err := requireProjectID(env, args)
			if err != nil {
				return err
			}
			return streamAndRun(ctx, env, id, false)
		},
	}
}

func streamAndRun(parent context.Context, env *Env, id string, trigger bool) error {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	// Subscribe BEFORE triggering the run — otherwise we'd miss the
	// earliest events.
	events, errs := env.Client.StreamProjectEvents(ctx, id)

	// Trigger run in the background. We capture the report at the end so
	// `--json` can return it.
	type runOutcome struct {
		report []byte
		err    error
	}
	runCh := make(chan runOutcome, 1)
	if trigger {
		go func() {
			report, err := env.Client.RunFinisher(ctx, id)
			runCh <- runOutcome{report: report, err: err}
		}()
	}

	// Handle Ctrl+C as "detach the stream" (cancel the context). The
	// run keeps going on the server.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	if !env.JSON {
		fmt.Fprintln(env.Err, ui.Dim("streaming events… press Ctrl+C to detach"))
	}
	for {
		select {
		case <-sigCh:
			if !env.JSON {
				fmt.Fprintln(env.Err, ui.Dim("\ndetached. the run continues server-side."))
			}
			cancel()
			return nil
		case e, ok := <-events:
			if !ok {
				events = nil
				// drain run outcome if we triggered one
				if trigger {
					out := <-runCh
					if out.err != nil {
						return out.err
					}
					if env.JSON && len(out.report) > 0 {
						fmt.Fprintln(env.Out, strings.TrimSpace(string(out.report)))
					}
				}
				return nil
			}
			renderEvent(env, e)
		case err, ok := <-errs:
			if !ok {
				continue
			}
			if err != nil {
				return err
			}
		case out := <-runCh:
			// /run finished. We still drain remaining events until the
			// stream closes naturally.
			if out.err != nil {
				return out.err
			}
			if env.JSON && len(out.report) > 0 {
				fmt.Fprintln(env.Out, strings.TrimSpace(string(out.report)))
			}
			runCh = nil // don't select on it again
		}
	}
}

// renderEvent formats one SSE frame for the terminal. The orchestrator's
// /stream endpoint emits `event: execution\ndata: {...}` frames where
// the data is the engine's finisher.Event JSON.
func renderEvent(env *Env, e client.SSEEvent) {
	if env.JSON {
		fmt.Fprintln(env.Out, e.Data)
		return
	}
	// Best-effort parse — if it's not JSON, just print it.
	var payload map[string]any
	if err := json.Unmarshal([]byte(e.Data), &payload); err != nil {
		fmt.Fprintln(env.Out, e.Data)
		return
	}
	role := stringOr(payload, "role", "agent", "kind", "type")
	msg := stringOr(payload, "message", "msg", "line", "text", "detail")
	gate := stringOr(payload, "gate")
	status := stringOr(payload, "status")
	prefix := "[" + role + "]"
	if role == "" {
		prefix = "[" + e.Event + "]"
	}
	colored := ui.AgentColor(role)(prefix)
	var body string
	switch {
	case gate != "" && status != "":
		body = fmt.Sprintf("gate=%s status=%s", gate, colorStatus(status))
		if msg != "" {
			body += " " + ui.Dim("— "+msg)
		}
	case msg != "":
		body = msg
	default:
		body = e.Data
	}
	fmt.Fprintln(env.Out, colored+" "+body)
}

func stringOr(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

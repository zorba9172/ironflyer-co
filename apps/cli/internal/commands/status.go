package commands

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"ironflyer/apps/cli/internal/ui"
)

// statusCmd pings the orchestrator's /health and the runtime's /healthz
// and renders both as colored status pills. The runtime URL is derived
// from $IRONFLYER_RUNTIME_HOST when set; otherwise we substitute the
// orchestrator host's port 8081 (the dev compose default).
func statusCmd() *Command {
	var runtimeHost string
	return &Command{
		Name:  "status",
		Short: "Check orchestrator + runtime health",
		Usage: "ironflyer status [--runtime URL]",
		RegFlags: func(fs *flag.FlagSet) {
			fs.StringVar(&runtimeHost, "runtime", "", "runtime base URL (overrides $IRONFLYER_RUNTIME_HOST)")
		},
		Run: func(ctx context.Context, env *Env, fs *flag.FlagSet, args []string) error {
			ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			orchStatus := "DOWN"
			orchDetail := ""
			if h, err := env.Client.Health(ctx); err == nil && h.OK {
				orchStatus = "OK"
				if h.Version != "" {
					orchDetail = h.Version
				}
			} else if err != nil {
				orchDetail = err.Error()
			}

			runtimeURL := runtimeHost
			if runtimeURL == "" {
				runtimeURL = os.Getenv("IRONFLYER_RUNTIME_HOST")
			}
			if runtimeURL == "" {
				runtimeURL = deriveRuntimeURL(env.Host)
			}
			runtimeStatus := "DOWN"
			runtimeDetail := ""
			if h, err := env.Client.HealthAt(ctx, runtimeURL); err == nil && h.OK {
				runtimeStatus = "OK"
				if h.Version != "" {
					runtimeDetail = h.Version
				}
			} else if err != nil {
				runtimeDetail = err.Error()
			}

			fmt.Fprintf(env.Out, "%s  %s  %s\n", ui.Pill("orchestrator", orchStatus), ui.Dim(env.Host), ui.Dim(orchDetail))
			fmt.Fprintf(env.Out, "%s  %s  %s\n", ui.Pill("runtime    ", runtimeStatus), ui.Dim(runtimeURL), ui.Dim(runtimeDetail))
			return nil
		},
	}
}

// deriveRuntimeURL maps an orchestrator URL to a best-guess runtime URL
// for the dev compose layout (orchestrator on 8080, runtime on 8081).
func deriveRuntimeURL(orchHost string) string {
	u, err := url.Parse(orchHost)
	if err != nil || u.Host == "" {
		return "http://localhost:8081"
	}
	host := u.Host
	// Swap a trailing :8080 → :8081. If no port present, append :8081.
	if h, _, hasPort := strings.Cut(host, ":"); hasPort {
		u.Host = h + ":8081"
	} else {
		u.Host = host + ":8081"
	}
	return u.String()
}

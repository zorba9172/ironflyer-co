package temporalworker

import (
	"os"
	"strings"
)

// Config carries the runtime knobs for connecting to Temporal and
// hosting the worker. All values are loaded from environment
// variables so the integration agent never has to hard-code them.
//
// Canonical names follow the IRONFLYER_TEMPORAL_* convention; the
// legacy TEMPORAL_* names from the v1 dev pack are NOT read here.
// The integration agent can map the legacy names into the canonical
// ones in main.go if backwards compatibility is required.
type Config struct {
	// Host is the Temporal frontend address (host:port). Empty
	// disables the worker entirely.
	Host string
	// Namespace is the Temporal namespace; defaults to "default".
	Namespace string
	// TaskQueue is the queue the worker polls; defaults to
	// "ironflyer-finisher" (Phase 1, single queue).
	TaskQueue string
	// Enabled is the master switch: true when IRONFLYER_EXECUTOR is
	// set to "temporal" (case-insensitive). The integration agent
	// checks this before constructing the worker so the embedded
	// path stays the default for dev / review apps.
	Enabled bool
}

const (
	defaultNamespace = "default"
	defaultTaskQueue = "ironflyer-finisher"
)

// LoadConfig reads the worker configuration from the process
// environment. Missing or blank values fall back to the documented
// defaults; the returned Config is always safe to pass to NewWorker
// (which validates Host before connecting).
func LoadConfig() Config {
	cfg := Config{
		Host:      strings.TrimSpace(os.Getenv("IRONFLYER_TEMPORAL_HOST")),
		Namespace: strings.TrimSpace(os.Getenv("IRONFLYER_TEMPORAL_NAMESPACE")),
		TaskQueue: strings.TrimSpace(os.Getenv("IRONFLYER_TEMPORAL_TASK_QUEUE")),
	}
	if cfg.Namespace == "" {
		cfg.Namespace = defaultNamespace
	}
	if cfg.TaskQueue == "" {
		cfg.TaskQueue = defaultTaskQueue
	}
	executor := strings.TrimSpace(strings.ToLower(os.Getenv("IRONFLYER_EXECUTOR")))
	cfg.Enabled = executor == "temporal" && cfg.Host != ""
	return cfg
}

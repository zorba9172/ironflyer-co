package temporalworker

import (
	"context"
	"errors"
	"strings"
	"sync"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

// Runtime is the live Temporal client + worker pair. Stop must be
// called during orchestrator shutdown so polling stops before the
// process closes its DB/provider connections.
type Runtime struct {
	client client.Client
	worker worker.Worker
	once   sync.Once
}

func Start(ctx context.Context, cfg Config, deps *Deps) (*Runtime, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	cfg.Host = strings.TrimSpace(cfg.Host)
	if cfg.Host == "" {
		return nil, errors.New("temporalworker: host required")
	}
	if cfg.Namespace == "" {
		cfg.Namespace = defaultNamespace
	}
	if cfg.TaskQueue == "" {
		cfg.TaskQueue = defaultTaskQueue
	}

	c, err := client.Dial(client.Options{
		HostPort:  cfg.Host,
		Namespace: cfg.Namespace,
	})
	if err != nil {
		return nil, err
	}

	SetActivityDeps(deps)
	w := worker.New(c, cfg.TaskQueue, worker.Options{})
	w.RegisterWorkflowWithOptions(FinisherExecutionWorkflow, workflow.RegisterOptions{Name: WorkflowName})
	w.RegisterActivity(AdmitExecutionActivity)
	w.RegisterActivity(StartExecutionActivity)
	w.RegisterActivity(ProfitGuardBeforeStepActivity)
	w.RegisterActivity(RunGateActivity)
	w.RegisterActivity(SettleExecutionActivity)
	w.RegisterActivity(EmitExecutionEventActivity)

	if err := w.Start(); err != nil {
		c.Close()
		SetActivityDeps(nil)
		return nil, err
	}
	rt := &Runtime{client: c, worker: w}
	go func() {
		<-ctx.Done()
		rt.Stop()
	}()
	return rt, nil
}

func (r *Runtime) Stop() {
	if r == nil {
		return
	}
	r.once.Do(func() {
		if r.worker != nil {
			r.worker.Stop()
		}
		if r.client != nil {
			r.client.Close()
		}
		SetActivityDeps(nil)
	})
}

package workflow

import (
	"context"

	"github.com/rs/zerolog"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

type WorkerOptions struct {
	TemporalAddr string
	Namespace    string
	TaskQueue    string
}

// StartWorker boots a Temporal client + worker, registers FinisherWorkflow
// and Activities, and returns the client + stop func. Caller is responsible
// for keeping the process alive.
func StartWorker(opts WorkerOptions, acts *Activities, logger zerolog.Logger) (client.Client, func(), error) {
	if opts.TaskQueue == "" {
		opts.TaskQueue = TaskQueueDefault
	}
	if opts.Namespace == "" {
		opts.Namespace = "default"
	}
	c, err := client.Dial(client.Options{
		HostPort:  opts.TemporalAddr,
		Namespace: opts.Namespace,
	})
	if err != nil {
		return nil, nil, err
	}
	w := worker.New(c, opts.TaskQueue, worker.Options{})
	w.RegisterWorkflow(FinisherWorkflow)
	w.RegisterActivityWithOptions(acts.CheckGate, activity.RegisterOptions{Name: ActivityCheckGate})
	w.RegisterActivityWithOptions(acts.RunAgent, activity.RegisterOptions{Name: ActivityRunAgent})

	if err := w.Start(); err != nil {
		c.Close()
		return nil, nil, err
	}
	logger.Info().Str("queue", opts.TaskQueue).Str("ns", opts.Namespace).Msg("temporal worker started")

	stop := func() {
		w.Stop()
		c.Close()
	}
	return c, stop, nil
}

// StartFinisher kicks off the finisher workflow for a project and returns the
// workflow run handle. Caller can wait on result or signal it.
func StartFinisher(ctx context.Context, c client.Client, opts WorkerOptions, projectID string) (client.WorkflowRun, error) {
	if opts.TaskQueue == "" {
		opts.TaskQueue = TaskQueueDefault
	}
	return c.ExecuteWorkflow(
		ctx,
		client.StartWorkflowOptions{
			ID:        "finisher-" + projectID,
			TaskQueue: opts.TaskQueue,
		},
		FinisherWorkflow,
		FinisherInput{ProjectID: projectID},
	)
}

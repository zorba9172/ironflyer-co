package events

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type PumpConfig struct {
	WorkerID     string
	BatchSize    int
	PollInterval time.Duration
	Lease        time.Duration
	MaxAttempts  int
}

type Pump struct {
	outbox    Outbox
	publisher Publisher
	cfg       PumpConfig
	log       zerolog.Logger
}

func NewPump(outbox Outbox, publisher Publisher, cfg PumpConfig, log zerolog.Logger) *Pump {
	if cfg.WorkerID == "" {
		cfg.WorkerID = "orchestrator-" + uuid.NewString()
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 50
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = time.Second
	}
	if cfg.Lease <= 0 {
		cfg.Lease = 30 * time.Second
	}
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 10
	}
	return &Pump{outbox: outbox, publisher: publisher, cfg: cfg, log: log}
}

func (p *Pump) Run(ctx context.Context) error {
	if p == nil || p.outbox == nil || p.publisher == nil {
		return errors.New("events: pump not configured")
	}
	t := time.NewTicker(p.cfg.PollInterval)
	defer t.Stop()
	for {
		if err := p.drainOnce(ctx); err != nil {
			p.log.Warn().Err(err).Msg("events pump drain")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
		}
	}
}

func (p *Pump) drainOnce(ctx context.Context) error {
	events, err := p.outbox.Claim(ctx, p.cfg.WorkerID, p.cfg.BatchSize, p.cfg.Lease)
	if err != nil {
		return err
	}
	for _, e := range events {
		if err := p.publisher.Publish(ctx, e); err != nil {
			dead := e.Attempts >= p.cfg.MaxAttempts
			retry := backoff(e.Attempts)
			if markErr := p.outbox.MarkFailed(ctx, e.ID, err, retry, dead); markErr != nil {
				p.log.Warn().Err(markErr).Str("event_id", e.ID.String()).Msg("events mark failed")
			}
			continue
		}
		if err := p.outbox.MarkPublished(ctx, e.ID); err != nil {
			p.log.Warn().Err(err).Str("event_id", e.ID.String()).Msg("events mark published")
		}
	}
	return nil
}

func backoff(attempts int) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	seconds := math.Pow(2, float64(attempts-1))
	if seconds > 300 {
		seconds = 300
	}
	return time.Duration(seconds) * time.Second
}

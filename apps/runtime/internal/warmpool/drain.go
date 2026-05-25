package warmpool

import (
	"context"
	"time"

	"github.com/rs/zerolog"
)

// Drainer is the periodic loop that calls Pool.Drain. The integration
// agent starts it under the runtime's errgroup; it stops cleanly on
// context cancellation.
type Drainer struct {
	pool     Pool
	interval time.Duration
	cooldown time.Duration
	logger   zerolog.Logger
}

// NewDrainer builds a Drainer with the given interval (how often to
// check for drainable slots) and cooldown (how long the paid queue
// must remain empty before drain proceeds).
func NewDrainer(pool Pool, interval, cooldown time.Duration, logger zerolog.Logger) *Drainer {
	if interval <= 0 {
		interval = 60 * time.Second
	}
	return &Drainer{
		pool:     pool,
		interval: interval,
		cooldown: cooldown,
		logger:   logger.With().Str("component", "warmpool-drainer").Logger(),
	}
}

// Run blocks until ctx is cancelled.
func (d *Drainer) Run(ctx context.Context) {
	t := time.NewTicker(d.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			n, err := d.pool.Drain(ctx, d.cooldown)
			if err != nil {
				d.logger.Warn().Err(err).Msg("warmpool drain")
				continue
			}
			if n > 0 {
				d.logger.Info().Int("drained", n).Msg("warmpool drained idle slots")
			}
		}
	}
}

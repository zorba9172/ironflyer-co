package eas

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"ironflyer/core/orchestrator/internal/business/ledger"
)

// Publisher is the minimal event-emission contract the poller needs.
// The orchestrator's events package targets a Postgres outbox shape
// with a typed Event envelope; we keep the poller-side contract
// deliberately simple so resolvers can subscribe via an in-memory
// fan-out without dragging the durable outbox into the hot path.
//
// A nil Publisher is no-op: the poller skips emission and only does
// ledger + artifact work.
type Publisher interface {
	Publish(ctx context.Context, event string, payload any)
}

// ArtifactSink is the orchestrator-side callback the poller invokes
// once a build succeeds and the artifact has been downloaded. The
// implementation owns uploading to S3 / R2 / Spaces / MinIO via the
// existing storage helpers and stamping the project with the
// resulting artifact pointer.
//
// `artifactSize` is the size in bytes when known (0 when EAS did not
// surface it), and `reader` streams the binary payload. The sink MUST
// fully drain `reader` before returning, even on error.
type ArtifactSink interface {
	StoreMobileArtifact(ctx context.Context, ref MobileArtifactRef, reader io.Reader) error
}

// ArtifactSinkFunc adapts a function value to ArtifactSink so
// integrations that don't want to introduce a new struct can wire the
// callback inline.
type ArtifactSinkFunc func(ctx context.Context, ref MobileArtifactRef, reader io.Reader) error

// StoreMobileArtifact satisfies ArtifactSink.
func (f ArtifactSinkFunc) StoreMobileArtifact(ctx context.Context, ref MobileArtifactRef, reader io.Reader) error {
	return f(ctx, ref, reader)
}

// MobileArtifactRef is the small struct the artifact sink stamps onto
// the project on completion. The orchestrator's S3 layout convention
// is documented in internal/storage/s3client.go — sinks should follow
// `projects/<projectID>/artifacts/mobile/<buildID>/<filename>`.
type MobileArtifactRef struct {
	ProjectID    string
	WorkspaceID  string
	BuildID      string
	Platform     string
	ArtifactURL  string // upstream EAS download URL (signed, expires)
	ArtifactSize int64
}

// Poller drives the EAS GetBuild loop for every build the orchestrator
// kicked off via the runtime. On status change it emits an event, on
// success it records a ledger entry and hands the artifact off to the
// sink. Terminal builds drop out of the queue.
type Poller struct {
	client      *Client
	ledgerSvc   ledger.Service
	publisher   Publisher
	sink        ArtifactSink
	logger      zerolog.Logger
	interval    time.Duration
	maxAttempts int

	mu       sync.Mutex
	tracking map[string]*trackedBuild

	stopOnce sync.Once
	stopCh   chan struct{}
}

// trackedBuild is the per-build state the poller mutates on each tick.
type trackedBuild struct {
	BuildID     string
	ProjectID   string
	WorkspaceID string
	TenantID    uuid.UUID
	ExecutionID uuid.UUID

	LastStatus BuildStatus
	Attempts   int
	AddedAt    time.Time
}

// PollerOption tweaks Poller defaults at construction time.
type PollerOption func(*Poller)

// WithPollerInterval overrides the default 20s tick.
func WithPollerInterval(d time.Duration) PollerOption {
	return func(p *Poller) {
		if d > 0 {
			p.interval = d
		}
	}
}

// WithPollerMaxAttempts caps how many ticks a single build may stay in
// the queue before the poller drops it. Default is 360 (= 2h at 20s).
func WithPollerMaxAttempts(n int) PollerOption {
	return func(p *Poller) {
		if n > 0 {
			p.maxAttempts = n
		}
	}
}

// WithPollerPublisher injects an event publisher. nil = no-op.
func WithPollerPublisher(pub Publisher) PollerOption {
	return func(p *Poller) { p.publisher = pub }
}

// WithPollerArtifactSink injects the artifact-completion callback.
func WithPollerArtifactSink(sink ArtifactSink) PollerOption {
	return func(p *Poller) { p.sink = sink }
}

// WithPollerLogger swaps the default zerolog logger.
func WithPollerLogger(l zerolog.Logger) PollerOption {
	return func(p *Poller) { p.logger = l }
}

// NewPoller builds a Poller. The ledger service is optional — when
// nil, build-completion events still fire but the EAS-build-credit
// entry is skipped.
func NewPoller(client *Client, ledgerSvc ledger.Service, opts ...PollerOption) *Poller {
	p := &Poller{
		client:      client,
		ledgerSvc:   ledgerSvc,
		logger:      zerolog.Nop(),
		interval:    20 * time.Second,
		maxAttempts: 360,
		tracking:    make(map[string]*trackedBuild),
		stopCh:      make(chan struct{}),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// TrackOpts is the typed bag the Track() entrypoint accepts. The
// minimal shape carries the EAS build id; the orchestrator-side
// identifiers (project + workspace) flow through so the artifact sink
// and ledger entry can correlate.
type TrackOpts struct {
	BuildID     string
	ProjectID   string
	WorkspaceID string
	TenantID    uuid.UUID
	ExecutionID uuid.UUID
}

// Track enqueues a build for polling. Idempotent: re-tracking an
// already-tracked id is a no-op.
func (p *Poller) Track(opts TrackOpts) {
	if p == nil || opts.BuildID == "" {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.tracking[opts.BuildID]; ok {
		return
	}
	p.tracking[opts.BuildID] = &trackedBuild{
		BuildID:     opts.BuildID,
		ProjectID:   opts.ProjectID,
		WorkspaceID: opts.WorkspaceID,
		TenantID:    opts.TenantID,
		ExecutionID: opts.ExecutionID,
		AddedAt:     time.Now().UTC(),
	}
	p.logger.Info().
		Str("build_id", opts.BuildID).
		Str("project_id", opts.ProjectID).
		Msg("eas poller: tracking build")
}

// Untrack removes a build from the queue without waiting for it to
// terminate. Used when the operator cancels.
func (p *Poller) Untrack(buildID string) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.tracking, buildID)
}

// Tracked returns the number of in-flight builds — handy for
// /admin/metrics surfacing and tests.
func (p *Poller) Tracked() int {
	if p == nil {
		return 0
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.tracking)
}

// Start blocks until ctx cancels or Stop is called. Each tick polls
// every tracked build sequentially — the EAS API tolerates parallel
// reads but sequencing keeps log lines deterministic and avoids
// hammering the rate-limit ceiling when the queue is large.
func (p *Poller) Start(ctx context.Context) error {
	if p == nil {
		return errors.New("eas: Poller is nil")
	}
	if p.client == nil {
		return errors.New("eas: Poller has nil Client")
	}
	t := time.NewTicker(p.interval)
	defer t.Stop()

	p.logger.Info().
		Dur("interval", p.interval).
		Int("max_attempts", p.maxAttempts).
		Msg("eas poller: started")
	defer p.logger.Info().Msg("eas poller: stopped")

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-p.stopCh:
			return nil
		case <-t.C:
			p.tickOnce(ctx)
		}
	}
}

// Stop signals Start() to return. Safe to call multiple times.
func (p *Poller) Stop() {
	if p == nil {
		return
	}
	p.stopOnce.Do(func() { close(p.stopCh) })
}

// tickOnce polls every tracked build. Snapshot the map under lock so
// the HTTP layer can call Track/Untrack while a tick is in flight.
func (p *Poller) tickOnce(ctx context.Context) {
	p.mu.Lock()
	pending := make([]*trackedBuild, 0, len(p.tracking))
	for _, tb := range p.tracking {
		pending = append(pending, tb)
	}
	p.mu.Unlock()

	for _, tb := range pending {
		p.checkOne(ctx, tb)
	}
}

// checkOne polls one build, fires the events, runs the on-complete
// side-effects, and removes terminal entries.
func (p *Poller) checkOne(ctx context.Context, tb *trackedBuild) {
	tb.Attempts++
	build, err := p.client.GetBuild(ctx, tb.BuildID)
	if err != nil {
		p.logger.Warn().
			Err(err).
			Str("build_id", tb.BuildID).
			Int("attempts", tb.Attempts).
			Msg("eas poller: GetBuild failed")
		if tb.Attempts >= p.maxAttempts {
			p.logger.Error().
				Str("build_id", tb.BuildID).
				Msg("eas poller: dropping build after max attempts")
			p.publish(ctx, "mobile.build.dropped", tb, nil)
			p.untrack(tb.BuildID)
		}
		return
	}

	if build.Status != tb.LastStatus {
		eventName := "mobile.build." + string(build.Status)
		p.publish(ctx, eventName, tb, build)
		tb.LastStatus = build.Status
	}

	if !build.Status.Terminal() {
		if tb.Attempts >= p.maxAttempts {
			p.logger.Error().
				Str("build_id", tb.BuildID).
				Str("status", string(build.Status)).
				Msg("eas poller: dropping non-terminal build after max attempts")
			p.publish(ctx, "mobile.build.timeout", tb, build)
			p.untrack(tb.BuildID)
		}
		return
	}

	// Terminal — run the per-status side effects.
	if build.Status.Succeeded() {
		p.onSuccess(ctx, tb, build)
	}
	p.untrack(tb.BuildID)
}

// onSuccess records the ledger entry and streams the artifact through
// to the sink. Failures here log loudly but never re-queue; the EAS
// build is done and the orchestrator owns the recovery flow.
func (p *Poller) onSuccess(ctx context.Context, tb *trackedBuild, build *Build) {
	if p.ledgerSvc != nil && tb.TenantID != uuid.Nil {
		if _, err := ledger.RecordEASBuild(ctx, p.ledgerSvc, tb.TenantID, tb.ExecutionID, build.ID); err != nil {
			p.logger.Error().
				Err(err).
				Str("build_id", build.ID).
				Msg("eas poller: RecordEASBuild failed")
		}
	}

	if p.sink != nil && build.ArtifactURL != "" {
		reader, err := p.client.DownloadArtifact(ctx, build.ArtifactURL)
		if err != nil {
			p.logger.Error().
				Err(err).
				Str("build_id", build.ID).
				Msg("eas poller: artifact download failed")
			return
		}
		defer reader.Close()
		ref := MobileArtifactRef{
			ProjectID:    tb.ProjectID,
			WorkspaceID:  tb.WorkspaceID,
			BuildID:      build.ID,
			Platform:     build.Platform,
			ArtifactURL:  build.ArtifactURL,
			ArtifactSize: build.ArtifactSize,
		}
		if err := p.sink.StoreMobileArtifact(ctx, ref, reader); err != nil {
			p.logger.Error().
				Err(err).
				Str("build_id", build.ID).
				Msg("eas poller: artifact sink failed")
		}
	}
}

// publish forwards an orchestration event to the Publisher when one is
// wired. Payload carries both the tracked-build metadata and (when
// available) the latest Build snapshot so subscribers can correlate
// without a second round-trip.
func (p *Poller) publish(ctx context.Context, event string, tb *trackedBuild, build *Build) {
	if p.publisher == nil {
		return
	}
	payload := map[string]any{
		"build_id":     tb.BuildID,
		"project_id":   tb.ProjectID,
		"workspace_id": tb.WorkspaceID,
	}
	if tb.TenantID != uuid.Nil {
		payload["tenant_id"] = tb.TenantID.String()
	}
	if build != nil {
		payload["status"] = string(build.Status)
		payload["platform"] = build.Platform
		payload["artifact_url"] = build.ArtifactURL
		payload["app_version"] = build.AppVersion
		if build.Error != nil {
			payload["error"] = build.Error.Message
		}
	}
	defer func() {
		// Subscribers must not bring the poller down.
		if r := recover(); r != nil {
			p.logger.Error().
				Str("event", event).
				Interface("recover", r).
				Msg("eas poller: publisher panicked")
		}
	}()
	p.publisher.Publish(ctx, event, payload)
}

// untrack removes a build from the queue.
func (p *Poller) untrack(buildID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.tracking, buildID)
}

// String returns a one-line summary of the poller state — used by the
// /admin/metrics surface today and a future dashboard tile.
func (p *Poller) String() string {
	if p == nil {
		return "eas.Poller<nil>"
	}
	return fmt.Sprintf("eas.Poller(interval=%s, tracking=%d)", p.interval, p.Tracked())
}

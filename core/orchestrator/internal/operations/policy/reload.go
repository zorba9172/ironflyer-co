package policy

import (
	"context"
	"errors"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog"
)

// PDPRebinder is the slice of PDP that policy.Reloader needs: the
// atomic-swap entry point that replaces the in-memory bundle set + the
// derived prepared query with a freshly-loaded snapshot from disk.
//
// LocalPDP satisfies this via the Rebind method in opa_local_reload.go.
// Remote and disabled PDPs deliberately do NOT — reloading a remote
// OPA sidecar happens out-of-band (operator pushes a new bundle to
// the sidecar's bundle service), and the disabled stub has no policy
// state to swap. The reloader logs and exits cleanly when handed a
// nil rebinder.
type PDPRebinder interface {
	Rebind(ctx context.Context, bundles map[string]string, version string) error
}

// defaultReloadInterval is the polling cadence Reloader.Run falls
// back to when fsnotify is unavailable (or as the safety re-check
// when fsnotify is active and we want to catch atomic-replace
// patterns that some editors trigger).
const defaultReloadInterval = 30 * time.Second

// Reloader watches cfg.BundleDir for changes and pushes new bundles
// into the PDP via Rebind on every observed change. The watch is
// best-effort: when fsnotify cannot be initialised (no syscall
// support in the running container, /tmp dir caps blown, etc.) the
// reloader falls back to interval-based polling so hot-reload is
// never silently disabled.
//
// Embedded-only deployments (cfg.BundleDir == "") get a Reloader that
// returns immediately from Run — there is nothing on disk to watch.
type Reloader struct {
	cfg      Config
	pdp      PDPRebinder
	log      zerolog.Logger
	interval time.Duration

	// currentVersion is the last bundle hash we successfully pushed
	// into the PDP. We use it to suppress redundant Rebind calls when
	// the filesystem fires multiple events for the same logical
	// change (e.g. editor save → rename → fsync triple).
	currentVersion string
}

// NewReloader wires the reloader. interval <= 0 falls back to the
// defaultReloadInterval; the integration agent overrides via env
// (IRONFLYER_OPA_RELOAD_INTERVAL_SECONDS).
func NewReloader(cfg Config, pdp PDPRebinder, log zerolog.Logger) *Reloader {
	return &Reloader{
		cfg:      cfg,
		pdp:      pdp,
		log:      log.With().Str("subsystem", "policy.reload").Logger(),
		interval: defaultReloadInterval,
	}
}

// WithInterval overrides the polling cadence used for both the
// fsnotify safety re-check and the fallback polling loop. Returns the
// receiver for fluent wiring at construction time.
func (r *Reloader) WithInterval(d time.Duration) *Reloader {
	if d > 0 {
		r.interval = d
	}
	return r
}

// Run blocks until ctx is cancelled, watching cfg.BundleDir and
// pushing changes into the PDP via Rebind. Returns nil when the dir
// is empty (embedded-only mode), ctx.Err() on cancellation, or a
// non-nil error only on a hard configuration mistake (nil rebinder
// with a bundle dir configured).
func (r *Reloader) Run(ctx context.Context) error {
	if r == nil || r.cfg.BundleDir == "" {
		// Embedded-only mode: nothing to watch, exit cleanly so the
		// integration agent's errgroup doesn't trip on a missing
		// reloader.
		if r != nil {
			r.log.Info().Msg("policy reloader inactive (no BundleDir configured)")
		}
		return nil
	}
	if r.pdp == nil {
		// A remote / disabled PDP cannot be rebound from disk;
		// surface the misconfiguration loudly rather than spinning
		// silently.
		return errors.New("policy: Reloader requires a PDPRebinder when BundleDir is set")
	}

	// Prime the current version so the first fsnotify event after
	// startup only fires when the on-disk bundle set actually differs
	// from what the PDP already holds.
	if _, version, err := ReloadFromDisk(r.cfg.BundleDir); err == nil {
		r.currentVersion = version
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		r.log.Warn().Err(err).Msg("fsnotify unavailable, falling back to interval polling")
		return r.runPolling(ctx)
	}
	defer func() { _ = watcher.Close() }()

	if err := watcher.Add(r.cfg.BundleDir); err != nil {
		r.log.Warn().Err(err).Str("dir", r.cfg.BundleDir).Msg("fsnotify add failed, falling back to interval polling")
		return r.runPolling(ctx)
	}

	r.log.Info().
		Str("dir", r.cfg.BundleDir).
		Dur("safety_poll", r.interval).
		Msg("policy reloader watching bundle dir")

	// debounceUntil collapses bursts of editor events (vim writes
	// trigger CREATE+RENAME+CHMOD; VSCode triggers two CREATEs) into
	// a single reload. The window is intentionally small so operators
	// see their change land within a second of saving.
	const debounce = 250 * time.Millisecond
	var debounceUntil time.Time
	safety := time.NewTicker(r.interval)
	defer safety.Stop()

	for {
		select {
		case <-ctx.Done():
			r.log.Info().Msg("policy reloader stopped")
			return ctx.Err()
		case ev, ok := <-watcher.Events:
			if !ok {
				return r.runPolling(ctx)
			}
			// Only react to events on .rego files; ignore swap files
			// and the directory itself.
			if !isRegoEvent(ev) {
				continue
			}
			now := time.Now()
			if now.Before(debounceUntil) {
				continue
			}
			debounceUntil = now.Add(debounce)
			r.reloadOnce(ctx)
		case err, ok := <-watcher.Errors:
			if !ok {
				return r.runPolling(ctx)
			}
			r.log.Warn().Err(err).Msg("fsnotify error (continuing)")
		case <-safety.C:
			// Safety re-check catches atomic-replace patterns
			// (editor writes to a tmp file then renames over the
			// target) that some platforms surface only as a delete
			// on the watched dir.
			r.reloadOnce(ctx)
		}
	}
}

// runPolling is the interval-driven fallback when fsnotify cannot be
// initialised or the watcher channel closes mid-flight.
func (r *Reloader) runPolling(ctx context.Context) error {
	t := time.NewTicker(r.interval)
	defer t.Stop()
	r.log.Info().
		Str("dir", r.cfg.BundleDir).
		Dur("interval", r.interval).
		Msg("policy reloader polling bundle dir")
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			r.reloadOnce(ctx)
		}
	}
}

// reloadOnce loads the on-disk bundle set, compares its version to
// the last-pushed one, and calls Rebind on a real change. Failures
// are logged and swallowed so a bad bundle never disables the
// reloader entirely — the existing prepared query stays bound.
func (r *Reloader) reloadOnce(ctx context.Context) {
	bundles, version, err := ReloadFromDisk(r.cfg.BundleDir)
	if err != nil {
		r.log.Warn().Err(err).Str("dir", r.cfg.BundleDir).Msg("policy reload: disk read failed")
		return
	}
	if len(bundles) == 0 {
		// Empty dir is almost certainly an operator mistake mid-edit;
		// keep the existing PDP state rather than swapping in nothing.
		return
	}
	if version == r.currentVersion {
		return
	}
	if err := r.pdp.Rebind(ctx, bundles, version); err != nil {
		r.log.Warn().
			Err(err).
			Str("policy_bundle_version", version).
			Msg("policy reload: PDP rebind failed (old bundle remains active)")
		return
	}
	r.log.Info().
		Str("policy_bundle_version", version).
		Int("modules", len(bundles)).
		Msg("policy bundle hot-reloaded")
	r.currentVersion = version
}

// isRegoEvent filters fsnotify events down to the ones that matter
// for bundle reload: any create/write/remove/rename on a .rego file.
// Chmod events are ignored — they don't change bundle contents and
// would trigger spurious reloads on container start when umask
// settles.
func isRegoEvent(ev fsnotify.Event) bool {
	if ev.Name == "" {
		return false
	}
	if !hasRegoSuffix(ev.Name) {
		return false
	}
	switch {
	case ev.Op&fsnotify.Create != 0,
		ev.Op&fsnotify.Write != 0,
		ev.Op&fsnotify.Remove != 0,
		ev.Op&fsnotify.Rename != 0:
		return true
	}
	return false
}

// hasRegoSuffix is the lightweight equivalent of
// strings.HasSuffix(name, ".rego") that avoids importing strings just
// for one call. Inlined here to keep the file's import surface tight.
func hasRegoSuffix(name string) bool {
	const suf = ".rego"
	if len(name) < len(suf) {
		return false
	}
	return name[len(name)-len(suf):] == suf
}

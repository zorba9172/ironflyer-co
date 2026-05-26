// Package verifier — playwright_install.go ensures the workspace
// has chromium downloaded before the first verifier run. We install
// once per workspace lifetime and record a marker file at
// `.ironflyer/.playwright-installed` so subsequent gate iterations
// short-circuit. A failed install is non-fatal: the gate's actual
// `npx playwright test` invocation will surface a structured error,
// and the operator can pre-bake chromium into their workspace image
// to skip this step entirely.

package verifier

import (
	"context"
	"errors"
	"strings"

	"github.com/rs/zerolog"

	"ironflyer/core/orchestrator/internal/operations/runtime"
)

// installMarker is the path inside the workspace the verifier
// touches after a successful chromium install. The presence of the
// file is the only signal we use — its contents are unused.
const installMarker = ".ironflyer/.playwright-installed"

// EnsurePlaywright runs `npx playwright install chromium --with-deps`
// the first time the workspace needs it. Subsequent calls are no-ops
// because the marker file exists.
//
// The function is best-effort by design — every failure path returns
// an error, and the caller decides whether to abort the gate or
// continue (the verifier package currently chooses to continue,
// because a chromium image preloaded by the operator is the
// production path).
func EnsurePlaywright(ctx context.Context, rt *runtime.Client, bearer, workspaceID string, lg zerolog.Logger) error {
	if rt == nil || !rt.Enabled() {
		return errors.New("runtime not configured")
	}
	if workspaceID == "" {
		return errors.New("workspace required")
	}
	// Cheap check: is the marker there already?
	check := "test -f " + shellQuote(installMarker) + " && echo 'present' || echo 'absent'"
	res, err := rt.Exec(ctx, bearer, workspaceID, runtime.ExecOpts{
		Shell: check, TimeoutSeconds: 10,
	})
	if err == nil && !res.TimedOut && strings.Contains(res.Stdout, "present") {
		lg.Debug().Msg("verifier: chromium already installed in workspace")
		return nil
	}
	lg.Info().Msg("verifier: installing chromium for the first time in this workspace")
	// `--with-deps` pulls the apt packages chromium needs (libnss,
	// libatk, fonts). On Alpine-based images the apt-get call will
	// fail; we tolerate that — chromium itself will still install,
	// and the subsequent playwright test will surface any real
	// runtime breakage.
	install := "mkdir -p .ironflyer && npx --yes playwright install chromium --with-deps 2>&1 || npx --yes playwright install chromium 2>&1"
	res, err = rt.Exec(ctx, bearer, workspaceID, runtime.ExecOpts{
		Shell: install, TimeoutSeconds: 300,
	})
	if err != nil {
		return err
	}
	if res.TimedOut {
		return errors.New("playwright install timed out")
	}
	if res.ExitCode != 0 {
		return errors.New("playwright install failed: " + tail(res.Stderr, 400))
	}
	// Touch the marker. We don't fail the install when this errors —
	// the next gate run will simply re-attempt, which is harmless.
	mark := "touch " + shellQuote(installMarker)
	_, _ = rt.Exec(ctx, bearer, workspaceID, runtime.ExecOpts{
		Shell: mark, TimeoutSeconds: 5,
	})
	return nil
}

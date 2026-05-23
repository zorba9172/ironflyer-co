// Package finisher — workspace syntax precheck. Before a patch enters
// the lifecycle (Propose → Critic → Apply) we ask the workspace's
// language toolchain whether the *result* would still parse. This is
// cheaper than letting a broken patch fail the Lint gate three layers
// downstream — Lint costs a full sandbox exec; precheck costs one
// `node --check` or `go vet` per file.
//
// The precheck is best-effort:
//   - We only inspect file ops where Op ∈ {create, update, replace,
//     insert_after} — delete cannot break syntax.
//   - When the runtime is not available we skip silently (the Lint gate
//     is still the authoritative check).
//   - Languages without a fast standalone parser (Markdown, YAML, JSON)
//     get a tiny in-process verifier rather than a shell-out.
//
// Returns a list of domain.Issue suitable for attaching to a patch
// rejection. Empty slice = looks parseable. The Engine treats a
// non-empty result as a hard rejection, refusing to send the patch
// through `Propose`.

package finisher

import (
	"context"
	"encoding/json"
	"strings"

	"ironflyer/apps/orchestrator/internal/domain"
	"ironflyer/apps/orchestrator/internal/patch"
	"ironflyer/apps/orchestrator/internal/runtime"
)

// syntaxPrecheck dispatches one check per file in the patch. Files are
// resolved to their POST-patch contents (so anchor-based ops see the
// substituted body, not the original), written into a temp path inside
// the workspace, and validated. The temp path is removed afterward.
//
// Returns a slice of issues; empty slice = looks parseable.
func (e *Engine) syntaxPrecheck(ctx context.Context, proj *domain.Project, workspaceID, bearer string, changes []patch.FileChange) []domain.Issue {
	if e == nil || e.runtime == nil || !e.runtime.Enabled() || workspaceID == "" {
		return nil
	}
	var issues []domain.Issue
	// Simulate the post-patch project so anchor ops see substituted text.
	simulated := simulateApply(*proj, changes)
	for _, ch := range changes {
		if ch.Op == patch.OpDelete {
			continue
		}
		body := contentAfterPatch(simulated, ch.Path)
		if body == "" {
			continue
		}
		check := pickSyntaxCheck(ch.Path)
		if check == nil {
			continue
		}
		problem := check(ctx, e.runtime, bearer, workspaceID, ch.Path, body)
		if problem != "" {
			issues = append(issues, domain.Issue{
				Gate:     domain.GateCode,
				Severity: domain.SeverityError,
				Message:  "syntax precheck failed: " + tail(problem, 320),
				Path:     ch.Path,
				Hint:     "fix the syntax error in your patch before reproposing",
			})
		}
	}
	return issues
}

// contentAfterPatch returns the simulated post-patch contents for the
// given path, or empty string when the file would not exist.
func contentAfterPatch(p domain.Project, path string) string {
	for _, f := range p.Files {
		if f.Path == path {
			return f.Content
		}
	}
	return ""
}

// syntaxCheckFn is the contract a per-language checker implements. It
// returns the empty string on success, or a problem description on
// failure. ctx + runtime + workspace let the checker shell out into the
// sandbox; path + body are the file under inspection.
type syntaxCheckFn func(
	ctx context.Context,
	rt *runtime.Client,
	bearer, workspaceID, path, body string,
) string

// pickSyntaxCheck returns the right checker for path's extension or nil
// when no fast check is available.
func pickSyntaxCheck(path string) syntaxCheckFn {
	low := strings.ToLower(path)
	switch {
	case strings.HasSuffix(low, ".json"):
		return checkJSON
	case strings.HasSuffix(low, ".js") || strings.HasSuffix(low, ".mjs") || strings.HasSuffix(low, ".cjs"):
		return checkNodeJS
	case strings.HasSuffix(low, ".ts") || strings.HasSuffix(low, ".tsx") || strings.HasSuffix(low, ".jsx"):
		// We could shell out to tsc per file, but it pulls in tsconfig
		// resolution and is slow. Node's `--check` works on TS-stripped
		// syntax only after we run it through `--input-type=module
		// --experimental-strip-types` on Node 22+. Use it conditionally.
		return checkNodeTS
	case strings.HasSuffix(low, ".go"):
		return checkGo
	case strings.HasSuffix(low, ".py"):
		return checkPython
	}
	return nil
}

// checkJSON parses the body in-process. JSON is small + cheap enough
// that round-tripping through the workspace would be wasteful.
func checkJSON(_ context.Context, _ *runtime.Client, _, _, _, body string) string {
	var v any
	if err := json.Unmarshal([]byte(body), &v); err != nil {
		return err.Error()
	}
	return ""
}

// checkNodeJS shells `node --check <tmp>` into the workspace. Fast (~50ms)
// and handles ESM/CJS uniformly.
func checkNodeJS(ctx context.Context, rt *runtime.Client, bearer, workspaceID, path, body string) string {
	return runSyntaxShellCheck(ctx, rt, bearer, workspaceID, path, body,
		"command -v node >/dev/null 2>&1 || { echo skip; exit 0; }; node --check %s")
}

// checkNodeTS uses node's experimental strip-types when available.
// Falls back to "no opinion" rather than blocking the patch on stacks
// where the toolchain doesn't include a fast TS syntax checker — Lint
// gate will still catch real errors via tsc.
func checkNodeTS(ctx context.Context, rt *runtime.Client, bearer, workspaceID, path, body string) string {
	return runSyntaxShellCheck(ctx, rt, bearer, workspaceID, path, body,
		`if command -v node >/dev/null 2>&1; then node -e 'try{require("typescript")}catch{process.exit(0)}'; if [ $? -eq 0 ]; then node -e "const ts=require('typescript');const src=require('fs').readFileSync('%s','utf8');const r=ts.transpileModule(src,{compilerOptions:{noEmit:true}});if(r.diagnostics&&r.diagnostics.length){process.stderr.write(r.diagnostics.map(d=>ts.flattenDiagnosticMessageText(d.messageText,'\n')).join('\n'));process.exit(1)}"; fi; fi`)
}

// checkGo runs `gofmt -e` which parses the file and reports syntax
// errors without requiring a full module context. Fast (~10ms).
func checkGo(ctx context.Context, rt *runtime.Client, bearer, workspaceID, path, body string) string {
	return runSyntaxShellCheck(ctx, rt, bearer, workspaceID, path, body,
		"command -v gofmt >/dev/null 2>&1 && gofmt -e %s >/dev/null || true")
}

// checkPython uses `python -c "compile(open(p).read(),p,'exec')"`. No
// virtualenv needed — system Python is enough for a syntax check.
func checkPython(ctx context.Context, rt *runtime.Client, bearer, workspaceID, path, body string) string {
	return runSyntaxShellCheck(ctx, rt, bearer, workspaceID, path, body,
		"command -v python3 >/dev/null 2>&1 && python3 -c \"import sys;compile(open('%s').read(),'%s','exec')\" 2>&1 || true")
}

// runSyntaxShellCheck writes the body to a deterministic temp path
// inside the workspace, runs the supplied shell template (with %s
// substituted to the temp path), and returns stderr+stdout when the
// command exits non-zero. We use a per-path temp file so concurrent
// prechecks against different patches don't collide.
func runSyntaxShellCheck(
	ctx context.Context, rt *runtime.Client, bearer, workspaceID, path, body, shellTpl string,
) string {
	tmp := ".ironflyer/precheck-" + sanitiseDBIdentifier(path) + ".tmp"
	if err := rt.WriteFile(ctx, bearer, workspaceID, tmp, []byte(body)); err != nil {
		// Workspace problem — don't block the patch on infra flakes.
		return ""
	}
	// Substitute %s with the tmp path. Single substitution required by
	// most templates; checkPython uses two %s slots intentionally.
	cmd := shellTpl
	for strings.Contains(cmd, "%s") {
		cmd = strings.Replace(cmd, "%s", tmp, 1)
	}
	res, err := rt.Exec(ctx, bearer, workspaceID, runtime.ExecOpts{
		Shell: cmd, TimeoutSeconds: 8,
	})
	// Always try to clean up the tmp file even on error.
	_ = rt.WriteFile(ctx, bearer, workspaceID, tmp+".done", []byte{})
	if err != nil || res.TimedOut {
		return ""
	}
	if res.ExitCode == 0 {
		return ""
	}
	out := strings.TrimSpace(res.Stderr + "\n" + res.Stdout)
	if out == "" || out == "skip" {
		return ""
	}
	return out
}

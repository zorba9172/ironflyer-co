// Package scaffold turns an Architect-chosen stack into a complete, instantly
// runnable starter project. It maps a StackSpec to a starter template
// directory under templates/starters/, walks every file, performs token
// substitution against a known-allowed set of variables, and produces a
// patch.Patch the finisher loop can route through its normal validate +
// apply pipeline.
//
// The package is intentionally dependency-light: no template engine, no
// YAML, no codegen. Variable substitution is a single pass over each file's
// bytes using strings.ReplaceAll with an allow-listed token map, so the
// scaffolder can never accidentally leak an environment value into the
// generated project tree.
package scaffold

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ironflyer/apps/orchestrator/internal/patch"
)

// StackSpec mirrors the Architect's stack decision in the minimal form the
// scaffolder needs. Fields are intentionally loose strings so callers can
// pass through whatever the LLM produced and let the engine normalise.
type StackSpec struct {
	Framework string // "next" | "vite" | "astro" | "go" | "static"
	Language  string // "ts" | "js" | "go"
	Database  string // "postgres" | "sqlite" | "none"
	Auth      bool
	Style     string // "mui" | "tailwind" | "plain"
}

// Engine is the scaffold producer. It is safe for concurrent use; the only
// state is the immutable templates root path.
type Engine struct {
	// Root is the directory that contains the starter template tree
	// (templates/starters/). When empty, the engine discovers it at first
	// use by walking the working directory upward until it finds a folder
	// named templates/starters/ with go.mod nearby.
	Root string
}

// New returns an Engine using Root for template discovery. Pass an empty
// string to enable auto-discovery from cwd.
func New(root string) *Engine { return &Engine{Root: root} }

// Default discovers a templates root and returns an Engine. If discovery
// fails the engine still functions but every Scaffold call will return an
// error — callers should surface that as a fatal startup mistake rather
// than skip silently.
func Default() *Engine {
	root, _ := discoverRoot()
	return &Engine{Root: root}
}

// MaxFiles caps how many files a single scaffold patch may contain. The
// orchestrator's patch engine also enforces a cap but we keep a tighter
// scaffolder-specific limit so a malformed template can't DoS the loop.
const MaxFiles = 60

// MaxBytes caps total content size across a scaffold patch. 1 MiB is plenty
// for realistic starter projects and small enough that the runtime apply
// step finishes well under a second.
const MaxBytes = 1024 * 1024

// SentinelPath is the marker file the scaffolder writes so subsequent runs
// can detect that scaffolding has already happened and skip.
const SentinelPath = ".ironflyer/scaffold.json"

// Scaffold builds a patch.Patch that, when applied, materialises a complete
// runnable starter project matching stack. projectName is used for the
// {{PROJECT_NAME}} substitution; it is sanitised but never echoed back into
// the file content beyond the documented tokens.
//
// The returned patch has ProjectID empty — the caller (the finisher loop)
// fills that in before calling patch.Engine.Propose.
func (e *Engine) Scaffold(stack StackSpec, projectName string) (patch.Patch, error) {
	if e == nil {
		return patch.Patch{}, errors.New("scaffold: nil engine")
	}
	root := e.Root
	if root == "" {
		discovered, err := discoverRoot()
		if err != nil {
			return patch.Patch{}, fmt.Errorf("scaffold: discover templates: %w", err)
		}
		root = discovered
	}

	dir := starterDir(stack)
	templateDir := filepath.Join(root, dir)
	info, err := os.Stat(templateDir)
	if err != nil || !info.IsDir() {
		return patch.Patch{}, fmt.Errorf("scaffold: template %q not found at %s", dir, templateDir)
	}

	vars := buildVars(projectName)

	var changes []patch.FileChange
	total := 0

	walkErr := filepath.WalkDir(templateDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(templateDir, path)
		if relErr != nil {
			return relErr
		}
		// Normalise to forward slashes for cross-OS workspace paths.
		rel = filepath.ToSlash(rel)
		if !safePath(rel) {
			return fmt.Errorf("scaffold: unsafe template path %q", rel)
		}
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		out := substitute(string(raw), vars)
		total += len(out)
		if total > MaxBytes {
			return fmt.Errorf("scaffold: template %s exceeds %d bytes", dir, MaxBytes)
		}
		changes = append(changes, patch.FileChange{
			Op:      patch.OpCreate,
			Path:    rel,
			Content: out,
		})
		if len(changes) > MaxFiles {
			return fmt.Errorf("scaffold: template %s exceeds %d files", dir, MaxFiles)
		}
		return nil
	})
	if walkErr != nil {
		return patch.Patch{}, walkErr
	}
	if len(changes) == 0 {
		return patch.Patch{}, fmt.Errorf("scaffold: template %s contained no files", dir)
	}

	// Append the sentinel last so it's the final breadcrumb of a successful
	// scaffold. If anything earlier failed, the sentinel never lands and the
	// next run will redo the work cleanly.
	sentinel := fmt.Sprintf(`{
  "starter": %q,
  "framework": %q,
  "language": %q,
  "style": %q,
  "createdAt": %q,
  "projectName": %q
}
`, dir, stack.Framework, stack.Language, stack.Style, vars["TODAY"], vars["PROJECT_NAME"])
	changes = append(changes, patch.FileChange{
		Op:      patch.OpCreate,
		Path:    SentinelPath,
		Content: sentinel,
	})

	return patch.Patch{
		Author:  "scaffold",
		Title:   "scaffold " + dir,
		Summary: "Initial " + dir + " starter for " + vars["PROJECT_NAME"] + ".",
		Changes: changes,
	}, nil
}

// starterDir picks the template directory name for a StackSpec. The mapping
// is deliberately permissive on the input strings (the LLM-generated stack
// may say "Next.js", "next", "nextjs", "react"); we lowercase + match
// substrings so close-but-not-exact answers still scaffold the right thing.
//
// The mapping is the single source of truth — keep it in sync with
// templates/starters/.
func starterDir(s StackSpec) string {
	fw := strings.ToLower(strings.TrimSpace(s.Framework))
	lang := strings.ToLower(strings.TrimSpace(s.Language))

	switch {
	case strings.Contains(fw, "next"):
		return "nextjs-ts"
	case strings.Contains(fw, "vite"), strings.Contains(fw, "react") && lang != "go":
		return "vite-react-ts"
	case strings.Contains(fw, "astro"):
		return "astro"
	case strings.Contains(fw, "go") || strings.Contains(fw, "chi") || lang == "go":
		return "go-chi"
	case strings.Contains(fw, "static"), strings.Contains(fw, "html"):
		return "static-html"
	default:
		// Sensible default: a Next.js TS app is the broadest match for what
		// users typically ask for.
		return "nextjs-ts"
	}
}

// buildVars returns the allow-listed token map used by substitute. The set
// is intentionally tiny and contains no env values, no secrets, no paths.
// If a template references a token not in this map it is left untouched.
func buildVars(projectName string) map[string]string {
	name := strings.TrimSpace(projectName)
	if name == "" {
		name = "Ironflyer Project"
	}
	return map[string]string{
		"PROJECT_NAME": name,
		"PROJECT_SLUG": slugify(name),
		"TODAY":        time.Now().UTC().Format("2006-01-02"),
	}
}

// substitute performs allow-listed token replacement. Only keys present in
// vars are recognised; the function never reads environment variables or
// any other state. We replace the longest tokens first to avoid any
// accidental nested-token collision (none exists today, but it's cheap
// insurance).
func substitute(in string, vars map[string]string) string {
	// Sort keys by length descending to keep substitution deterministic.
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	// Simple insertion sort; the map is tiny (3 entries).
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && len(keys[j]) > len(keys[j-1]); j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	out := in
	for _, k := range keys {
		out = strings.ReplaceAll(out, "{{"+k+"}}", vars[k])
	}
	return out
}

// slugify is a conservative project-name → slug. It only emits a-z 0-9 and
// '-'; everything else collapses to '-'. Used for package.json "name" and
// go.mod module name.
func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	dash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		default:
			if !dash && b.Len() > 0 {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	out := strings.TrimRight(b.String(), "-")
	if out == "" {
		return "ironflyer-project"
	}
	return out
}

// safePath rejects paths the scaffolder must never emit. The patch engine
// also rejects these, but failing fast in the scaffolder yields a clearer
// error message and prevents us from reading a template file we shouldn't.
func safePath(p string) bool {
	if p == "" {
		return false
	}
	if strings.HasPrefix(p, "/") {
		return false
	}
	if strings.Contains(p, "..") {
		return false
	}
	return true
}

// discoverRoot walks up from cwd until it finds a directory whose
// templates/starters subtree exists. The IRONFLYER_SCAFFOLD_ROOT env var,
// when set, short-circuits discovery — operators use it in container
// deployments where the binary lives outside the source tree.
func discoverRoot() (string, error) {
	if env := strings.TrimSpace(os.Getenv("IRONFLYER_SCAFFOLD_ROOT")); env != "" {
		if info, err := os.Stat(env); err == nil && info.IsDir() {
			return env, nil
		}
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := cwd
	for i := 0; i < 8; i++ {
		candidate := filepath.Join(dir, "templates", "starters")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", errors.New("templates/starters not found in cwd or any ancestor")
}

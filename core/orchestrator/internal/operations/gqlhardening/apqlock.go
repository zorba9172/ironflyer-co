package gqlhardening

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/errcode"
	"github.com/rs/zerolog"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
	"github.com/vektah/gqlparser/v2/parser"
)

// errPersistedQueryNotRegistered is returned in locked mode when the
// client sends an APQ hash that is not present in the startup-seeded
// registry. The wire code follows the V22 hardening contract
// (DEPLOY.md §5) and is distinct from the upstream gqlgen
// PERSISTED_QUERY_NOT_FOUND code so clients can tell the two failure
// modes apart and skip the "retry with full query" fallback.
const errPersistedQueryNotRegistered = "PERSISTED_QUERY_NOT_REGISTERED"

// errPersistedQueryRequired is returned in locked mode when the client
// sends a raw query string with no APQ extension at all. Operators are
// exempted via OperatorCheck so Sandbox / CLI ad-hoc queries keep
// working in production.
const errPersistedQueryRequired = "PERSISTED_QUERY_REQUIRED"

// LockedAPQ is a gqlgen OperationParameterMutator that enforces the
// "registered hashes only" mode of the APQ surface. It replaces the
// upstream extension.AutomaticPersistedQuery when GRAPHQL_APQ_LOCKED
// is true: the registry is pre-populated at startup, every Get() that
// misses returns PERSISTED_QUERY_NOT_REGISTERED, and Add() is a no-op
// so a client cannot grow the allowlist by sending a fresh
// (query, hash) pair on the wire.
//
// Operators (per the OperatorCheck) bypass the lock so the
// Sandbox / CLI still ships ad-hoc queries through the surface
// without pre-registration.
type LockedAPQ struct {
	registry   *RegistryCache
	logger     *zerolog.Logger
	isOperator OperatorCheck
}

// NewLockedAPQ builds a LockedAPQ wired around an existing
// RegistryCache. registry must be non-nil; pass an empty one if no
// queries have been seeded yet (every request will then be rejected,
// which is the correct fail-closed default).
func NewLockedAPQ(registry *RegistryCache, logger *zerolog.Logger, isOperator OperatorCheck) *LockedAPQ {
	return &LockedAPQ{registry: registry, logger: logger, isOperator: isOperator}
}

var _ interface {
	graphql.HandlerExtension
	graphql.OperationParameterMutator
} = (*LockedAPQ)(nil)

func (l *LockedAPQ) ExtensionName() string                     { return "IronflyerLockedAPQ" }
func (l *LockedAPQ) Validate(_ graphql.ExecutableSchema) error { return nil }

func (l *LockedAPQ) MutateOperationParameters(ctx context.Context, rawParams *graphql.RawParams) *gqlerror.Error {
	if l == nil || l.registry == nil {
		return nil
	}
	// Operator bypass — Sandbox and the CLI still need an escape hatch
	// in production.
	if operatorAllowed(ctx, l.isOperator) {
		return nil
	}

	pqRaw, hasExt := rawParams.Extensions["persistedQuery"]
	if !hasExt {
		// Raw query, no APQ extension. In locked mode this is the
		// "ad-hoc query" path and is rejected.
		if rawParams.Query == "" {
			// Neither hash nor query — let downstream parse error.
			return nil
		}
		l.logRejection("raw_query", "", rawParams.OperationName)
		persistedHits.WithLabelValues("locked_raw_query").Inc()
		err := gqlerror.Errorf("persisted query required in locked mode")
		errcode.Set(err, errPersistedQueryRequired)
		return err
	}

	hash := extractSha256(pqRaw)
	if hash == "" {
		err := gqlerror.Errorf("invalid APQ extension data")
		errcode.Set(err, errPersistedQueryNotRegistered)
		return err
	}

	query, ok := l.registry.Get(ctx, hash)
	if !ok {
		l.logRejection("unknown_hash", hash, rawParams.OperationName)
		persistedHits.WithLabelValues("locked_miss").Inc()
		err := gqlerror.Errorf("persisted query not registered")
		errcode.Set(err, errPersistedQueryNotRegistered)
		return err
	}

	// If the client also sent the full query, verify it matches the
	// registered shape — protects against a client trying to smuggle a
	// different query under a known hash.
	if rawParams.Query != "" && rawParams.Query != query {
		l.logRejection("hash_mismatch", hash, rawParams.OperationName)
		persistedHits.WithLabelValues("locked_mismatch").Inc()
		err := gqlerror.Errorf("APQ hash does not match registered query")
		errcode.Set(err, errPersistedQueryNotRegistered)
		return err
	}

	rawParams.Query = query
	persistedHits.WithLabelValues("locked_hit").Inc()
	return nil
}

func (l *LockedAPQ) logRejection(reason, hash, opName string) {
	if l.logger == nil {
		return
	}
	ev := l.logger.Warn().
		Str("reason", reason).
		Str("code", errPersistedQueryNotRegistered)
	if hash != "" {
		ev = ev.Str("hash", hash)
	}
	if opName != "" {
		ev = ev.Str("operation", opName)
	}
	ev.Msg("graphql: rejected operation outside locked APQ registry")
}

func extractSha256(raw any) string {
	m, ok := raw.(map[string]any)
	if !ok {
		return ""
	}
	h, _ := m["sha256Hash"].(string)
	return h
}

// RegistryCache is a thread-safe set of known (hash → canonical query)
// entries. It implements graphql.Cache[string] so it plugs straight
// into the upstream extension.AutomaticPersistedQuery, but in locked
// mode Add() is a no-op so the allowlist cannot grow at runtime.
type RegistryCache struct {
	mu      sync.RWMutex
	entries map[string]string
	// locked controls whether Add() persists new entries. The startup
	// seeder calls AddDirect to bypass the lock; the gqlgen runtime
	// calls Add which respects it.
	locked bool
}

// NewRegistryCache returns an empty cache. Pass locked=true to make
// Add() a no-op once the seeder has populated the registry.
func NewRegistryCache(locked bool) *RegistryCache {
	return &RegistryCache{entries: map[string]string{}, locked: locked}
}

// Get implements graphql.Cache[string].
func (r *RegistryCache) Get(_ context.Context, key string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.entries[key]
	return v, ok
}

// Add implements graphql.Cache[string]. In locked mode the call is a
// no-op — only AddDirect (called by the startup seeder) can grow the
// allowlist.
func (r *RegistryCache) Add(_ context.Context, key string, value string) {
	if r.locked {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[key] = value
}

// AddDirect bypasses the lock so the startup seeder can populate the
// allowlist. Returns the sha256 hash that was stored.
func (r *RegistryCache) AddDirect(query string) string {
	hash := hashAPQQuery(query)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[hash] = query
	return hash
}

// Len returns the number of registered entries. Used by the startup
// banner so operators see the allowlist size at boot.
func (r *RegistryCache) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.entries)
}

// hashAPQQuery matches the Apollo + upstream gqlgen APQ hash —
// lowercase hex sha256 of the canonical query text.
func hashAPQQuery(query string) string {
	sum := sha256.Sum256([]byte(query))
	return hex.EncodeToString(sum[:])
}

// SeedRegistryFromDir walks dir and registers every named operation
// found in .graphql / .gql files. Each operation is re-emitted as a
// canonical string (deterministic across whitespace + comments) and
// hashed with the Apollo APQ algorithm. Returns the number of
// operations registered + any walk error.
//
// The canonical form is what the web client's codegen pipeline emits
// at build time, so the hashes computed here line up with the hashes
// the browser sends on the wire. Operators that need byte-exact
// control over the registered text can drop pre-hashed files in a
// dedicated directory and point GRAPHQL_APQ_REGISTRY_DIR there.
func SeedRegistryFromDir(reg *RegistryCache, dir string, logger *zerolog.Logger) (int, error) {
	if reg == nil {
		return 0, errors.New("gqlhardening: nil RegistryCache")
	}
	if dir == "" {
		return 0, nil
	}
	info, err := os.Stat(dir)
	if err != nil {
		return 0, err
	}
	if !info.IsDir() {
		return 0, errors.New("gqlhardening: APQ registry path is not a directory")
	}

	count := 0
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".graphql" && ext != ".gql" {
			return nil
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			if logger != nil {
				logger.Warn().Err(readErr).Str("file", path).Msg("graphql: APQ registry: skipping unreadable file")
			}
			return nil
		}
		// Hash the raw file body as well so clients that ship the
		// whole file as one operation (e.g. the SDK's compiled
		// `__generated__.ts` bundles) still hit the cache.
		reg.AddDirect(string(body))
		count++

		doc, parseErr := parser.ParseQuery(&ast.Source{Name: path, Input: string(body)})
		if parseErr != nil {
			if logger != nil {
				logger.Warn().Err(parseErr).Str("file", path).Msg("graphql: APQ registry: parse failed, indexed file body only")
			}
			return nil
		}
		for _, op := range doc.Operations {
			canonical := canonicalizeOperation(op, doc.Fragments)
			reg.AddDirect(canonical)
			count++
		}
		return nil
	})
	if err != nil {
		return count, err
	}
	return count, nil
}

// canonicalizeOperation emits a stable text form of op together with
// every fragment it transitively references. The output is suitable
// for hashing — fragment ordering is deterministic, whitespace is
// collapsed, and comments are stripped (the upstream gqlparser
// formatter is the source of truth here).
func canonicalizeOperation(op *ast.OperationDefinition, frags ast.FragmentDefinitionList) string {
	var sb strings.Builder
	formatOperation(&sb, op)
	used := map[string]bool{}
	collectFragmentRefs(op.SelectionSet, used)
	// Walk until no new fragments are pulled in — fragments may
	// reference other fragments transitively.
	for {
		before := len(used)
		for name := range used {
			f := frags.ForName(name)
			if f == nil {
				continue
			}
			collectFragmentRefs(f.SelectionSet, used)
		}
		if len(used) == before {
			break
		}
	}
	// Emit the referenced fragments in sorted order for determinism.
	names := make([]string, 0, len(used))
	for n := range used {
		names = append(names, n)
	}
	sortStrings(names)
	for _, n := range names {
		f := frags.ForName(n)
		if f == nil {
			continue
		}
		sb.WriteByte('\n')
		formatFragment(&sb, f)
	}
	return sb.String()
}

func collectFragmentRefs(set ast.SelectionSet, out map[string]bool) {
	for _, sel := range set {
		switch s := sel.(type) {
		case *ast.Field:
			collectFragmentRefs(s.SelectionSet, out)
		case *ast.InlineFragment:
			collectFragmentRefs(s.SelectionSet, out)
		case *ast.FragmentSpread:
			out[s.Name] = true
		}
	}
}

// formatOperation writes a deterministic single-line-ish form of op.
// Comments and original whitespace are dropped — what survives is the
// structural shape that gqlgen will execute against.
func formatOperation(sb *strings.Builder, op *ast.OperationDefinition) {
	sb.WriteString(string(op.Operation))
	if op.Name != "" {
		sb.WriteByte(' ')
		sb.WriteString(op.Name)
	}
	if len(op.VariableDefinitions) > 0 {
		sb.WriteByte('(')
		for i, vd := range op.VariableDefinitions {
			if i > 0 {
				sb.WriteByte(',')
			}
			sb.WriteByte('$')
			sb.WriteString(vd.Variable)
			sb.WriteByte(':')
			sb.WriteString(formatType(vd.Type))
		}
		sb.WriteByte(')')
	}
	formatSelectionSet(sb, op.SelectionSet)
}

func formatFragment(sb *strings.Builder, f *ast.FragmentDefinition) {
	sb.WriteString("fragment ")
	sb.WriteString(f.Name)
	sb.WriteString(" on ")
	sb.WriteString(f.TypeCondition)
	formatSelectionSet(sb, f.SelectionSet)
}

func formatSelectionSet(sb *strings.Builder, set ast.SelectionSet) {
	sb.WriteByte('{')
	for i, sel := range set {
		if i > 0 {
			sb.WriteByte(' ')
		}
		switch s := sel.(type) {
		case *ast.Field:
			if s.Alias != "" && s.Alias != s.Name {
				sb.WriteString(s.Alias)
				sb.WriteByte(':')
			}
			sb.WriteString(s.Name)
			if len(s.Arguments) > 0 {
				sb.WriteByte('(')
				for j, a := range s.Arguments {
					if j > 0 {
						sb.WriteByte(',')
					}
					sb.WriteString(a.Name)
					sb.WriteByte(':')
					sb.WriteString(a.Value.String())
				}
				sb.WriteByte(')')
			}
			if len(s.SelectionSet) > 0 {
				formatSelectionSet(sb, s.SelectionSet)
			}
		case *ast.InlineFragment:
			sb.WriteString("...")
			if s.TypeCondition != "" {
				sb.WriteString(" on ")
				sb.WriteString(s.TypeCondition)
			}
			formatSelectionSet(sb, s.SelectionSet)
		case *ast.FragmentSpread:
			sb.WriteString("...")
			sb.WriteString(s.Name)
		}
	}
	sb.WriteByte('}')
}

func formatType(t *ast.Type) string {
	if t == nil {
		return ""
	}
	if t.Elem != nil {
		inner := formatType(t.Elem)
		if t.NonNull {
			return "[" + inner + "]!"
		}
		return "[" + inner + "]"
	}
	if t.NonNull {
		return t.NamedType + "!"
	}
	return t.NamedType
}

// sortStrings is a tiny insertion sort — we avoid the sort package
// import for a cleaner dependency footprint in this hot file.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

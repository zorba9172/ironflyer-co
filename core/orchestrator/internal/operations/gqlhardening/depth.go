package gqlhardening

import (
	"context"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/errcode"
	"github.com/rs/zerolog"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

// DefaultMaxDepth is the depth ceiling enforced when DepthExtension is
// constructed with a zero or negative limit. 15 is the documented
// production default in DEPLOY.md §5 — comfortable for the
// orchestrator's schema (project → executions → ledger entries →
// wallet tops out at ~7) with headroom for nested viz queries, and
// aggressive enough to stop the classic fragment-cycle DoS.
const DefaultMaxDepth = 15

// errDepthLimit is the GraphQL error extension code returned when an
// operation exceeds the configured depth. The wire code is
// OPERATION_TOO_DEEP per the V22 hardening contract (DEPLOY.md §5).
const errDepthLimit = "OPERATION_TOO_DEEP"

// DepthExtension returns a gqlgen Extension that rejects any operation
// whose deepest field path exceeds maxDepth. The walk understands
// inline + named fragments and counts each Field exactly once per path.
//
// Fragment cycles are bounded by the visited-fragments set so a query
// like `fragment A on T { ...A }` cannot drive the walker into an
// infinite loop.
//
// logger is optional — when non-nil, every reject is logged at WARN
// with the operation name + measured depth so operators can tune the
// limit against real traffic.
func DepthExtension(maxDepth int, logger *zerolog.Logger) graphql.HandlerExtension {
	if maxDepth <= 0 {
		maxDepth = DefaultMaxDepth
	}
	return depthLimit{max: maxDepth, logger: logger}
}

type depthLimit struct {
	max    int
	logger *zerolog.Logger
}

var _ interface {
	graphql.OperationContextMutator
	graphql.HandlerExtension
} = depthLimit{}

func (depthLimit) ExtensionName() string                     { return "IronflyerDepthLimit" }
func (depthLimit) Validate(_ graphql.ExecutableSchema) error { return nil }

func (d depthLimit) MutateOperationContext(_ context.Context, opCtx *graphql.OperationContext) *gqlerror.Error {
	if opCtx == nil || opCtx.Doc == nil {
		return nil
	}
	op := opCtx.Operation
	if op == nil && opCtx.OperationName != "" {
		op = opCtx.Doc.Operations.ForName(opCtx.OperationName)
	}
	if op == nil && len(opCtx.Doc.Operations) > 0 {
		op = opCtx.Doc.Operations[0]
	}
	if op == nil {
		return nil
	}
	visited := map[string]bool{}
	depth := walkDepth(op.SelectionSet, opCtx.Doc.Fragments, visited, 1)
	if depth > d.max {
		opName := opCtx.OperationName
		if opName == "" {
			opName = "anonymous"
		}
		depthRejects.WithLabelValues(opName).Inc()
		if d.logger != nil {
			d.logger.Warn().
				Str("operation", opName).
				Int("depth", depth).
				Int("limit", d.max).
				Str("code", errDepthLimit).
				Msg("graphql: rejected operation exceeding depth limit")
		}
		err := gqlerror.Errorf("query depth %d exceeds the limit of %d", depth, d.max)
		errcode.Set(err, errDepthLimit)
		return err
	}
	return nil
}

// walkDepth walks the SelectionSet tree and returns the deepest field
// path observed. Each *ast.Field increments the depth by 1. Inline
// fragments are transparent (they don't add depth). Named fragments
// are resolved against the document's FragmentDefinitionList and are
// also transparent in terms of depth — they only contribute the depth
// of their inner selections.
func walkDepth(set ast.SelectionSet, frags ast.FragmentDefinitionList, visited map[string]bool, current int) int {
	max := current
	for _, sel := range set {
		switch s := sel.(type) {
		case *ast.Field:
			child := walkDepth(s.SelectionSet, frags, visited, current+1)
			if child > max {
				max = child
			}
		case *ast.InlineFragment:
			child := walkDepth(s.SelectionSet, frags, visited, current)
			if child > max {
				max = child
			}
		case *ast.FragmentSpread:
			if visited[s.Name] {
				continue
			}
			visited[s.Name] = true
			def := frags.ForName(s.Name)
			if def == nil {
				continue
			}
			child := walkDepth(def.SelectionSet, frags, visited, current)
			if child > max {
				max = child
			}
		}
	}
	return max
}

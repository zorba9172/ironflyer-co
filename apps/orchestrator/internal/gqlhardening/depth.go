package gqlhardening

import (
	"context"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/errcode"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

// DefaultMaxDepth is the depth ceiling enforced when DepthExtension is
// constructed with a zero or negative limit. 10 is comfortable for the
// orchestrator's schema (nested project → executions → ledger entries
// → wallet tops out at ~7) and aggressive enough to stop the
// classic fragment-cycle DoS.
const DefaultMaxDepth = 10

const errDepthLimit = "DEPTH_LIMIT_EXCEEDED"

// DepthExtension returns a gqlgen Extension that rejects any operation
// whose deepest field path exceeds maxDepth. The walk understands
// inline + named fragments and counts each Field exactly once per path.
//
// Fragment cycles are bounded by the visited-fragments set so a query
// like `fragment A on T { ...A }` cannot drive the walker into an
// infinite loop.
func DepthExtension(maxDepth int) graphql.HandlerExtension {
	if maxDepth <= 0 {
		maxDepth = DefaultMaxDepth
	}
	return depthLimit{max: maxDepth}
}

type depthLimit struct {
	max int
}

var _ interface {
	graphql.OperationContextMutator
	graphql.HandlerExtension
} = depthLimit{}

func (depthLimit) ExtensionName() string                          { return "IronflyerDepthLimit" }
func (depthLimit) Validate(_ graphql.ExecutableSchema) error      { return nil }

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

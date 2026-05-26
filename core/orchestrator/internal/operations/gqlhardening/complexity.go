package gqlhardening

import (
	"context"

	"github.com/99designs/gqlgen/complexity"
	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/errcode"
	"github.com/rs/zerolog"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

// DefaultComplexityLimit is the per-operation complexity ceiling
// applied when ComplexityExtension is constructed with a non-positive
// limit. 1000 is large enough for fully-loaded dashboard queries and
// small enough that an attacker can't fan-out an unauthenticated
// "list everything" attempt without tripping the limit.
const DefaultComplexityLimit = 1000

// errComplexityLimit is the GraphQL error extension code returned when
// an operation exceeds the configured complexity. The wire code is
// OPERATION_TOO_COMPLEX per the V22 hardening contract (DEPLOY.md §5).
const errComplexityLimit = "OPERATION_TOO_COMPLEX"

// FieldCostMap maps "Type.Field" (e.g. "Query.dashboards") → per-field
// extra cost. The cost is *added* on top of gqlgen's base complexity
// calculation for matching fields the operation actually references,
// so expensive resolvers stay visible in the limit even when the
// schema doesn't carry a @complexity directive yet.
type FieldCostMap map[string]int

// ComplexityExtension returns a gqlgen Extension that rejects any
// operation whose computed complexity exceeds the limit. The base
// number comes from gqlgen's calculator (the same one
// extension.FixedComplexityLimit uses); per-field extra costs are
// layered on top via the FieldCostMap.
//
// logger is optional — when non-nil, every reject is logged at WARN
// with the operation name + computed complexity score so operators
// can tune the limit against real traffic.
func ComplexityExtension(limit int, costs FieldCostMap, logger *zerolog.Logger) graphql.HandlerExtension {
	if limit <= 0 {
		limit = DefaultComplexityLimit
	}
	return &complexityExt{max: limit, costs: costs, logger: logger}
}

type complexityExt struct {
	max    int
	costs  FieldCostMap
	logger *zerolog.Logger

	es graphql.ExecutableSchema
}

var _ interface {
	graphql.OperationContextMutator
	graphql.HandlerExtension
} = (*complexityExt)(nil)

func (c *complexityExt) ExtensionName() string { return "IronflyerComplexityLimit" }

func (c *complexityExt) Validate(schema graphql.ExecutableSchema) error {
	c.es = schema
	return nil
}

func (c *complexityExt) MutateOperationContext(ctx context.Context, opCtx *graphql.OperationContext) *gqlerror.Error {
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
	calc := 0
	if c.es != nil {
		calc = complexity.Calculate(ctx, c.es, op, opCtx.Variables)
	}
	if extra := c.applyFieldCosts(op.SelectionSet, opCtx.Doc.Fragments, map[string]bool{}); extra > 0 {
		calc += extra
	}
	if calc > c.max {
		opName := opCtx.OperationName
		if opName == "" {
			opName = "anonymous"
		}
		complexityRejects.WithLabelValues(opName).Inc()
		if c.logger != nil {
			c.logger.Warn().
				Str("operation", opName).
				Int("complexity", calc).
				Int("limit", c.max).
				Str("code", errComplexityLimit).
				Msg("graphql: rejected operation exceeding complexity limit")
		}
		err := gqlerror.Errorf("operation has complexity %d, which exceeds the limit of %d", calc, c.max)
		errcode.Set(err, errComplexityLimit)
		return err
	}
	return nil
}

// applyFieldCosts walks the operation and totals up the per-field
// extra cost for entries the FieldCostMap declares. We resolve the
// parent type from the field's ObjectDefinition so the cost map can
// be written in the familiar "Query.dashboards" form.
func (c *complexityExt) applyFieldCosts(set ast.SelectionSet, frags ast.FragmentDefinitionList, visited map[string]bool) int {
	if len(c.costs) == 0 {
		return 0
	}
	total := 0
	for _, sel := range set {
		switch s := sel.(type) {
		case *ast.Field:
			parent := ""
			if s.ObjectDefinition != nil {
				parent = s.ObjectDefinition.Name
			}
			if parent != "" {
				if extra, ok := c.costs[parent+"."+s.Name]; ok {
					total += extra
				}
			}
			total += c.applyFieldCosts(s.SelectionSet, frags, visited)
		case *ast.InlineFragment:
			total += c.applyFieldCosts(s.SelectionSet, frags, visited)
		case *ast.FragmentSpread:
			if visited[s.Name] {
				continue
			}
			visited[s.Name] = true
			def := frags.ForName(s.Name)
			if def == nil {
				continue
			}
			total += c.applyFieldCosts(def.SelectionSet, frags, visited)
		}
	}
	return total
}

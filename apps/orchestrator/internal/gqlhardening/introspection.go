package gqlhardening

import (
	"context"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/errcode"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

const errIntrospectionDisabled = "INTROSPECTION_DISABLED"

// IntrospectionGate returns a gqlgen Extension that rejects operations
// whose root selection set touches __schema or __type when prodMode is
// true *and* the request's principal is not an operator.
//
// The gate inspects the parsed operation rather than the raw query so
// it transparently catches introspection requested via aliases or
// fragments. Operator detection is delegated to the supplied closure
// so the wiring agent can plug in the canonical isOperator predicate
// without this package importing the auth package directly.
//
// When prodMode is false (dev), the gate is a no-op — introspection
// remains useful for tooling. The package also flips
// OperationContext.DisableIntrospection on reject so any downstream
// extension that consults it observes a consistent decision.
func IntrospectionGate(prodMode bool, isOperator func(ctx context.Context) bool) graphql.HandlerExtension {
	return &introspectionGate{prodMode: prodMode, isOperator: isOperator}
}

type introspectionGate struct {
	prodMode   bool
	isOperator func(ctx context.Context) bool
}

var _ interface {
	graphql.OperationContextMutator
	graphql.HandlerExtension
} = (*introspectionGate)(nil)

func (introspectionGate) ExtensionName() string                          { return "IronflyerIntrospectionGate" }
func (introspectionGate) Validate(_ graphql.ExecutableSchema) error      { return nil }

func (g *introspectionGate) MutateOperationContext(ctx context.Context, opCtx *graphql.OperationContext) *gqlerror.Error {
	if !g.prodMode || opCtx == nil || opCtx.Doc == nil {
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
	if !introspectionUsed(op.SelectionSet) {
		return nil
	}
	if g.isOperator != nil && g.isOperator(ctx) {
		return nil
	}
	introspectionRejects.Inc()
	opCtx.DisableIntrospection = true
	err := gqlerror.Errorf("introspection is disabled in production")
	errcode.Set(err, errIntrospectionDisabled)
	return err
}

func introspectionUsed(set ast.SelectionSet) bool {
	for _, sel := range set {
		f, ok := sel.(*ast.Field)
		if !ok {
			continue
		}
		switch f.Name {
		case "__schema", "__type":
			return true
		}
	}
	return false
}

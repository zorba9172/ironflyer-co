package policy

import (
	"context"
	"strings"

	"github.com/99designs/gqlgen/graphql"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

// GraphQLOperationMiddleware returns a gqlgen OperationMiddleware that
// runs the PEP against every operation. Install it on the server:
//
//	srv.AroundOperations(policy.GraphQLOperationMiddleware(pep))
//
// The middleware:
//   - Computes action = "graphql.<opType>.<opName>" (e.g.
//     "graphql.mutation.startDeploy").
//   - Skips operations the bundle explicitly allowlists (no-op for
//     introspection and __schema; those have their own gates).
//   - On deny, returns a single redacted GraphQL error citing the
//     decision_id. The Reason is NOT leaked to the client — it can
//     contain internal policy detail.
func GraphQLOperationMiddleware(pep *PEP) graphql.OperationMiddleware {
	return func(ctx context.Context, next graphql.OperationHandler) graphql.ResponseHandler {
		opCtx := graphql.GetOperationContext(ctx)
		opName, opType := operationCoords(opCtx)
		// Allowlist operations the orchestrator wants to skip at the
		// PEP layer. Introspection is gated separately (gqlgen has its
		// own DisableIntrospection flag).
		if opType == "query" && (opName == "" || strings.HasPrefix(opName, "__")) {
			return next(ctx)
		}

		action := "graphql." + opType + "." + opName
		dec, err := pep.Allow(ctx, action, Resource{Kind: "graphql_op", ID: opName})
		if err != nil || dec.Effect != EffectAllow {
			return denyResponse(dec)
		}
		return next(ctx)
	}
}

// operationCoords extracts (operationName, operationType) from the
// gqlgen op context. Anonymous ops get an empty name; the bundle can
// still match on op type.
func operationCoords(opCtx *graphql.OperationContext) (string, string) {
	name := opCtx.OperationName
	opType := "query"
	if opCtx.Operation != nil {
		switch opCtx.Operation.Operation {
		case ast.Mutation:
			opType = "mutation"
		case ast.Subscription:
			opType = "subscription"
		case ast.Query:
			opType = "query"
		}
		if name == "" {
			name = opCtx.Operation.Name
		}
	}
	return name, opType
}

// denyResponse returns a graphql.ResponseHandler that emits a single
// redacted error. The decision_id is exposed in the error extensions
// so an operator can grep the audit chain.
func denyResponse(dec Decision) graphql.ResponseHandler {
	return func(ctx context.Context) *graphql.Response {
		ext := map[string]any{
			"code":        "POLICY_DENY",
			"decision_id": dec.DecisionID,
		}
		// Risk is non-sensitive (low/medium/high/critical) and useful
		// for client-side error categorisation.
		if dec.Risk != "" {
			ext["risk"] = dec.Risk
		}
		return &graphql.Response{
			Errors: gqlerror.List{{
				Message:    "policy denied operation",
				Extensions: ext,
			}},
		}
	}
}

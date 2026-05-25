// Package operator implements the operator-only service layer used by
// the GraphQL operator schema and the ironflyer-ops CLI.
//
// Every method on OperatorService MUST be called only by principals
// that pass RequireOperator (authz.go). The GraphQL resolver layer
// gates each query with that check; the CLI runs out-of-band with
// direct DB credentials so authn there is the operator's responsibility
// (the binary itself is operator-only).
package operator

import "errors"

// ErrNotOperator is returned by RequireOperator when the principal on
// the context either does not exist (no auth.User attached) or is
// authenticated but not flagged as an operator. Resolvers surface this
// as a FORBIDDEN GraphQL error.
var ErrNotOperator = errors.New("operator: principal is not an operator")

// ErrNotConfigured signals that a downstream V22 service the operator
// surface depends on (wallet / execution / deploy / abuse / audit) was
// not wired by main.go. Resolvers should surface this as
// NOT_CONFIGURED so the operator sees the gap without a 500.
var ErrNotConfigured = errors.New("operator: downstream service not configured")

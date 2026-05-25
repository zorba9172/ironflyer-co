// Helpers used by operator.resolver.go. Kept in a non-resolver file
// so gqlgen does not strip them on regenerate.
package resolver

import (
	"errors"

	"github.com/shopspring/decimal"
	"github.com/vektah/gqlparser/v2/gqlerror"

	"ironflyer/apps/orchestrator/internal/operator"
)

func gqlForbidden(reason string) *gqlerror.Error {
	if reason == "" {
		reason = "operator role required"
	}
	return &gqlerror.Error{
		Message: reason,
		Extensions: map[string]any{
			"code": "FORBIDDEN",
		},
	}
}

func operatorErrorToGQL(err error) error {
	switch {
	case errors.Is(err, operator.ErrNotOperator):
		return gqlForbidden("operator role required")
	case errors.Is(err, operator.ErrNotConfigured):
		return gqlNotConfigured("operator")
	default:
		return err
	}
}

func decToFloat(d decimal.Decimal) float64 {
	f, _ := d.Float64()
	return f
}

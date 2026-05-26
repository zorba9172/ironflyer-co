package logctx

import "context"

// stringValue is the shared helper for the typed accessors. Keeps the
// nil-ctx + wrong-type handling in one spot.
func stringValue(ctx context.Context, key ctxKey) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(key).(string)
	return v
}

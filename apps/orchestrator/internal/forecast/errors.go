package forecast

import "errors"

// ErrInvalidInput is returned when EstimateInput is missing a value
// the estimator cannot synthesise. Currently only TenantID is
// mandatory.
var ErrInvalidInput = errors.New("forecast: invalid input")

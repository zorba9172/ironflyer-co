package wowloop

import "errors"

// ErrNotConfigured is returned by the builder when a required source
// was not wired. The resolver maps this to a typed GraphQL error so
// the client knows the surface is unavailable rather than failing.
var ErrNotConfigured = errors.New("wowloop: builder not configured")

// ErrExecutionRequired is returned when Build is called with an empty
// execution id.
var ErrExecutionRequired = errors.New("wowloop: executionID required")

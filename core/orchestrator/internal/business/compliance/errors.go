package compliance

import "errors"

// ErrUnknownFramework is returned when a caller names a framework key
// that is not present in the static catalogue. The catalogue is the
// source of truth — adding a new framework is a code change so prices
// and gate lists stay reviewable.
var ErrUnknownFramework = errors.New("compliance: unknown framework")

// ErrNotFound is returned when a lookup by enrollment id (or
// tenant/project/framework) finds no row. Cross-tenant reads collapse
// to ErrNotFound rather than surfacing the existence of a foreign
// resource.
var ErrNotFound = errors.New("compliance: not found")

// ErrAlreadyEnrolled is returned by Enroll when the (tenant, project,
// framework) tuple already has an active enrolment row. Callers
// should treat it as a soft success (return the existing row).
var ErrAlreadyEnrolled = errors.New("compliance: already enrolled")

// ErrAttestationDisabled is returned by ExportAuditBundle when no
// IRONFLYER_ATTESTATION_SECRET is configured. Without the secret we
// refuse to mint an unsigned attestation rather than silently
// downgrading the artefact.
var ErrAttestationDisabled = errors.New("compliance: attestation secret not configured")

// ErrInsufficientBalance is returned by the billing path when the
// wallet refuses the monthly debit. Callers transition the enrolment
// into the dunning state.
var ErrInsufficientBalance = errors.New("compliance: wallet hold rejected")

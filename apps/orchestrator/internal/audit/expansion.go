package audit

import (
	"context"
	"time"
)

// V22 audit-helper layer.
//
// The hash-chain core (audit.go) intentionally exposes the lowest
// possible surface: one `Store.Record(ctx, Entry)`. That makes the
// chain easy to reason about but pushes the burden of building a
// well-shaped Entry onto every caller — which in turn invites drift:
// one resolver stamps `attrs["decision_id"]`, the next stamps
// `attrs["decisionId"]`, and the SIEM dashboard breaks.
//
// The helpers below are the canonical Entry constructors for every
// mandatory event class (events.go). Each helper:
//
//   - sets Action to the V22 event-name constant,
//   - stamps the standard structured attrs (tenant, decision_id,
//     policy_bundle_version, etc.) under the same key for every caller,
//   - merges any caller-supplied Attrs on top so feature-specific
//     fields ride in the same row,
//   - delegates to Store.Record so the hash chain, region stamp, and
//     PrevHash linkage all behave exactly as they do for native callers.
//
// Helpers are intentionally Store-typed (not method-receivers on Store)
// so they can be invoked from any package without touching the Store
// interface itself — keeping the canonical write `Record` as the only
// way to mutate the chain.

// mergeAttrs returns a single attrs map populated with `base` first and
// then any non-empty values from `extra` overwriting it. Callers pass
// extra=nil when they have no feature-specific fields to add.
func mergeAttrs(base, extra map[string]any) map[string]any {
	if base == nil {
		base = map[string]any{}
	}
	for k, v := range extra {
		if v == nil {
			continue
		}
		base[k] = v
	}
	return base
}

// outcomeFor maps a high-level effect string into the coarse Outcome
// vocabulary. Anything obviously denied/blocked becomes OutcomeBlocked,
// anything failing becomes OutcomeFailure, the rest is success — keeps
// dashboards aggregating without parsing free-form effect labels.
func outcomeFor(effect string) Outcome {
	switch effect {
	case "deny", "denied", "blocked", "block", "stop", "kill_branch":
		return OutcomeBlocked
	case "fail", "failed", "failure", "error":
		return OutcomeFailure
	default:
		return OutcomeSuccess
	}
}

// RecordPolicyDecision writes one row for a policy plane verdict —
// works for both deny and high-risk-allow surfaces. `kind` is the
// caller-supplied event-name constant (EventPolicyDeny /
// EventPolicyHighRiskAllow / EventGraphQLPolicyDeny / ...) so the same
// helper services every PDP integration point.
func RecordPolicyDecision(ctx context.Context, store Store, kind, tenantID, decisionID, policyBundleVersion, effect, reason string, attrs map[string]any) (Entry, error) {
	if store == nil {
		return Entry{}, nil
	}
	merged := mergeAttrs(map[string]any{
		"event":                 kind,
		"tenant_id":             tenantID,
		"decision_id":           decisionID,
		"policy_bundle_version": policyBundleVersion,
		"effect":                effect,
		"reason":                reason,
	}, attrs)
	return store.Record(ctx, Entry{
		Action:    Action(kind),
		Outcome:   outcomeFor(effect),
		UserID:    tenantID,
		Summary:   "policy " + effect + " (" + decisionID + ")",
		Attrs:     merged,
		CreatedAt: time.Now().UTC(),
	})
}

// RecordProfitGuardDecision lands a single ProfitGuard verdict on the
// chain. enforcementPoint is the profitguard.EnforcementPoint string,
// action is the chosen profitguard.Action.
func RecordProfitGuardDecision(ctx context.Context, store Store, tenantID, executionID, enforcementPoint, action, reason string, attrs map[string]any) (Entry, error) {
	if store == nil {
		return Entry{}, nil
	}
	merged := mergeAttrs(map[string]any{
		"event":             EventProfitGuardDecision,
		"tenant_id":         tenantID,
		"execution_id":      executionID,
		"enforcement_point": enforcementPoint,
		"action":            action,
		"reason":            reason,
	}, attrs)
	return store.Record(ctx, Entry{
		Action:    EventProfitGuardDecision,
		Outcome:   outcomeFor(action),
		UserID:    tenantID,
		Summary:   "profitguard " + enforcementPoint + " -> " + action,
		Attrs:     merged,
		CreatedAt: time.Now().UTC(),
	})
}

// RecordProviderDispatch records one outbound provider call. We never
// log raw prompts — only the hashes plus the redaction proof — so the
// audit row is safe to ship to a SIEM that doesn't carry the same
// data-residency posture as the orchestrator itself.
func RecordProviderDispatch(ctx context.Context, store Store, tenantID, executionID, provider, model, inputHash, outputHash, redactionProof string, attrs map[string]any) (Entry, error) {
	if store == nil {
		return Entry{}, nil
	}
	merged := mergeAttrs(map[string]any{
		"event":           EventProviderDispatch,
		"tenant_id":       tenantID,
		"execution_id":    executionID,
		"provider":        provider,
		"model":           model,
		"redaction_proof": redactionProof,
	}, attrs)
	return store.Record(ctx, Entry{
		Action:     EventProviderDispatch,
		Outcome:    OutcomeSuccess,
		UserID:     tenantID,
		Summary:    "provider " + provider + "/" + model,
		InputHash:  inputHash,
		OutputHash: outputHash,
		Attrs:      merged,
		CreatedAt:  time.Now().UTC(),
	})
}

// RecordCommandExec lands one row for a workspace command execution.
// The command body is captured as a hash + normalized argv only;
// touched paths are recorded so the audit answers "what files did this
// command write?" without re-running the command.
func RecordCommandExec(ctx context.Context, store Store, tenantID, workspaceID, executionID, cmdHash, normalizedArgv string, exitCode int, durationMS int, touchedPaths []string, policyDecisionID string) (Entry, error) {
	if store == nil {
		return Entry{}, nil
	}
	outcome := OutcomeSuccess
	if exitCode != 0 {
		outcome = OutcomeFailure
	}
	merged := map[string]any{
		"event":              EventWorkspaceCommandExec,
		"tenant_id":          tenantID,
		"workspace_id":       workspaceID,
		"execution_id":       executionID,
		"normalized_argv":    normalizedArgv,
		"exit_code":          exitCode,
		"duration_ms":        durationMS,
		"touched_paths":      touchedPaths,
		"policy_decision_id": policyDecisionID,
	}
	return store.Record(ctx, Entry{
		Action:    EventWorkspaceCommandExec,
		Outcome:   outcome,
		UserID:    tenantID,
		Summary:   "workspace exec " + normalizedArgv,
		InputHash: cmdHash,
		Attrs:     merged,
		CreatedAt: time.Now().UTC(),
	})
}

// RecordSecretRefWrite records a secret-broker write of a secret ref
// pointer. The secret value never lands on the row — only the ref id +
// the storage backend.
func RecordSecretRefWrite(ctx context.Context, store Store, tenantID, secretRefID, backend string, attrs map[string]any) (Entry, error) {
	if store == nil {
		return Entry{}, nil
	}
	merged := mergeAttrs(map[string]any{
		"event":         EventSecretRefWrite,
		"tenant_id":     tenantID,
		"secret_ref_id": secretRefID,
		"backend":       backend,
	}, attrs)
	return store.Record(ctx, Entry{
		Action:    EventSecretRefWrite,
		Outcome:   OutcomeSuccess,
		UserID:    tenantID,
		Summary:   "secret_ref write " + secretRefID,
		Attrs:     merged,
		CreatedAt: time.Now().UTC(),
	})
}

// RecordSecretRelease records a successful secret release to a
// short-lived consumer (workspace process, deploy step). releasedTo is
// the audience id (workspaceID, deploy step name, etc.). redactionProof
// is the hash the consumer must echo back to prove the released value
// was used in a redacted-output context.
func RecordSecretRelease(ctx context.Context, store Store, tenantID, secretRefID, releasedTo, policyDecisionID, redactionProof string) (Entry, error) {
	if store == nil {
		return Entry{}, nil
	}
	return store.Record(ctx, Entry{
		Action:  EventSecretRelease,
		Outcome: OutcomeSuccess,
		UserID:  tenantID,
		Summary: "secret release " + secretRefID + " -> " + releasedTo,
		Attrs: map[string]any{
			"event":              EventSecretRelease,
			"tenant_id":          tenantID,
			"secret_ref_id":      secretRefID,
			"released_to":        releasedTo,
			"policy_decision_id": policyDecisionID,
			"redaction_proof":    redactionProof,
		},
		CreatedAt: time.Now().UTC(),
	})
}

// RecordSecretReleaseDeny records a denied secret release. reason is the
// policy reason string; policyDecisionID links back to the PDP row.
func RecordSecretReleaseDeny(ctx context.Context, store Store, tenantID, secretRefID, releasedTo, policyDecisionID, reason string) (Entry, error) {
	if store == nil {
		return Entry{}, nil
	}
	return store.Record(ctx, Entry{
		Action:  EventSecretReleaseDeny,
		Outcome: OutcomeBlocked,
		UserID:  tenantID,
		Summary: "secret release DENIED " + secretRefID + " -> " + releasedTo,
		Attrs: map[string]any{
			"event":              EventSecretReleaseDeny,
			"tenant_id":          tenantID,
			"secret_ref_id":      secretRefID,
			"released_to":        releasedTo,
			"policy_decision_id": policyDecisionID,
			"reason":             reason,
		},
		CreatedAt: time.Now().UTC(),
	})
}

// RecordSecretRotation records a successful secret-rotation operation.
func RecordSecretRotation(ctx context.Context, store Store, tenantID, secretRefID, backend, previousVersion, newVersion string) (Entry, error) {
	if store == nil {
		return Entry{}, nil
	}
	return store.Record(ctx, Entry{
		Action:  EventSecretRotation,
		Outcome: OutcomeSuccess,
		UserID:  tenantID,
		Summary: "secret rotated " + secretRefID,
		Attrs: map[string]any{
			"event":            EventSecretRotation,
			"tenant_id":        tenantID,
			"secret_ref_id":    secretRefID,
			"backend":          backend,
			"previous_version": previousVersion,
			"new_version":      newVersion,
		},
		CreatedAt: time.Now().UTC(),
	})
}

// RecordPatchLifecycle is the shared writer for the four patch
// lifecycle event classes (proposed, approved, applied, rolled_back).
// The pre-V22 audit.go already records patch.proposed / patch.applied /
// patch.rolled_back under different Action names — this helper is the
// canonical V22 path for NEW callers that want the .v1 schema.
func RecordPatchLifecycle(ctx context.Context, store Store, eventName, tenantID, projectID, patchID string, attrs map[string]any) (Entry, error) {
	if store == nil {
		return Entry{}, nil
	}
	merged := mergeAttrs(map[string]any{
		"event":      eventName,
		"tenant_id":  tenantID,
		"project_id": projectID,
		"patch_id":   patchID,
	}, attrs)
	return store.Record(ctx, Entry{
		Action:    Action(eventName),
		Outcome:   OutcomeSuccess,
		UserID:    tenantID,
		ProjectID: projectID,
		Summary:   eventName + " patch=" + patchID,
		Attrs:     merged,
		CreatedAt: time.Now().UTC(),
	})
}

// RecordGateVerdict records one gate verdict under the V22 .v1 schema.
// The legacy ActionGateVerdict is still used by the engine loop —
// migrate callers gradually.
func RecordGateVerdict(ctx context.Context, store Store, tenantID, projectID, executionID, gateName, status string, issueCount int, attrs map[string]any) (Entry, error) {
	if store == nil {
		return Entry{}, nil
	}
	outcome := OutcomeSuccess
	if status == "failed" {
		outcome = OutcomeFailure
	} else if status == "blocked" {
		outcome = OutcomeBlocked
	}
	merged := mergeAttrs(map[string]any{
		"event":        EventGateVerdict,
		"tenant_id":    tenantID,
		"execution_id": executionID,
		"gate_name":    gateName,
		"status":       status,
		"issue_count":  issueCount,
	}, attrs)
	return store.Record(ctx, Entry{
		Action:    EventGateVerdict,
		Outcome:   outcome,
		UserID:    tenantID,
		ProjectID: projectID,
		GateName:  gateName,
		Summary:   "gate " + gateName + " " + status,
		Attrs:     merged,
		CreatedAt: time.Now().UTC(),
	})
}

// RecordGateWaiver records an operator-issued gate waiver. waiverID is
// the durable id of the waiver record so operators can pull the
// approval chain back from the audit row.
func RecordGateWaiver(ctx context.Context, store Store, tenantID, projectID, gateName, waiverID, operatorID, reason string, expiresAt time.Time) (Entry, error) {
	if store == nil {
		return Entry{}, nil
	}
	return store.Record(ctx, Entry{
		Action:    EventGateWaiver,
		Outcome:   OutcomeSuccess,
		UserID:    tenantID,
		ProjectID: projectID,
		GateName:  gateName,
		Summary:   "gate waiver " + gateName + " by " + operatorID,
		Attrs: map[string]any{
			"event":       EventGateWaiver,
			"tenant_id":   tenantID,
			"gate_name":   gateName,
			"waiver_id":   waiverID,
			"operator_id": operatorID,
			"reason":      reason,
			"expires_at":  expiresAt.UTC().Format(time.RFC3339Nano),
		},
		CreatedAt: time.Now().UTC(),
	})
}

// RecordDeployPlan records the deploy plan event — the deploy.Service
// "planned" state.
func RecordDeployPlan(ctx context.Context, store Store, tenantID, deployID, target, environment, diffHash, artifactHash string, attrs map[string]any) (Entry, error) {
	if store == nil {
		return Entry{}, nil
	}
	merged := mergeAttrs(map[string]any{
		"event":         EventDeployPlan,
		"tenant_id":     tenantID,
		"deploy_id":     deployID,
		"target":        target,
		"environment":   environment,
		"diff_hash":     diffHash,
		"artifact_hash": artifactHash,
	}, attrs)
	return store.Record(ctx, Entry{
		Action:    EventDeployPlan,
		Outcome:   OutcomeSuccess,
		UserID:    tenantID,
		Summary:   "deploy plan " + target + "/" + environment,
		Attrs:     merged,
		CreatedAt: time.Now().UTC(),
	})
}

// RecordDeployApproval records an approval (or revocation) decision
// against a planned deploy. action is "approved" | "revoked" |
// "expired".
func RecordDeployApproval(ctx context.Context, store Store, tenantID, deployID, approvalID, actorUserID, action string, expiresAt time.Time, diffHash, artifactHash string) (Entry, error) {
	if store == nil {
		return Entry{}, nil
	}
	return store.Record(ctx, Entry{
		Action:  EventDeployApproval,
		Outcome: outcomeFor(action),
		UserID:  tenantID,
		Summary: "deploy " + action + " " + deployID + " by " + actorUserID,
		Attrs: map[string]any{
			"event":         EventDeployApproval,
			"tenant_id":     tenantID,
			"deploy_id":     deployID,
			"approval_id":   approvalID,
			"actor_user_id": actorUserID,
			"action":        action,
			"expires_at":    expiresAt.UTC().Format(time.RFC3339Nano),
			"diff_hash":     diffHash,
			"artifact_hash": artifactHash,
		},
		CreatedAt: time.Now().UTC(),
	})
}

// RecordDeployProviderAction records a provider-side action taken by a
// deploy adapter (start build, promote, rollback API call).
func RecordDeployProviderAction(ctx context.Context, store Store, tenantID, deployID, target, providerAction, providerRefID string, attrs map[string]any) (Entry, error) {
	if store == nil {
		return Entry{}, nil
	}
	merged := mergeAttrs(map[string]any{
		"event":           EventDeployProviderAction,
		"tenant_id":       tenantID,
		"deploy_id":       deployID,
		"target":          target,
		"provider_action": providerAction,
		"provider_ref_id": providerRefID,
	}, attrs)
	return store.Record(ctx, Entry{
		Action:    EventDeployProviderAction,
		Outcome:   OutcomeSuccess,
		UserID:    tenantID,
		Summary:   "deploy provider " + target + " action=" + providerAction,
		Attrs:     merged,
		CreatedAt: time.Now().UTC(),
	})
}

// RecordDeploySmokeResult records the outcome of a post-deploy smoke
// check. ok=true is OutcomeSuccess, ok=false is OutcomeFailure.
func RecordDeploySmokeResult(ctx context.Context, store Store, tenantID, deployID string, ok bool, summary string, attrs map[string]any) (Entry, error) {
	if store == nil {
		return Entry{}, nil
	}
	outcome := OutcomeSuccess
	if !ok {
		outcome = OutcomeFailure
	}
	merged := mergeAttrs(map[string]any{
		"event":     EventDeploySmokeResult,
		"tenant_id": tenantID,
		"deploy_id": deployID,
		"ok":        ok,
	}, attrs)
	return store.Record(ctx, Entry{
		Action:    EventDeploySmokeResult,
		Outcome:   outcome,
		UserID:    tenantID,
		Summary:   "deploy smoke " + deployID + ": " + summary,
		Attrs:     merged,
		CreatedAt: time.Now().UTC(),
	})
}

// RecordDeployRollback records a rollback of a promoted deploy.
func RecordDeployRollback(ctx context.Context, store Store, tenantID, deployID, target, actorUserID, reason string) (Entry, error) {
	if store == nil {
		return Entry{}, nil
	}
	return store.Record(ctx, Entry{
		Action:  EventDeployRollback,
		Outcome: OutcomeSuccess,
		UserID:  tenantID,
		Summary: "deploy rollback " + deployID,
		Attrs: map[string]any{
			"event":         EventDeployRollback,
			"tenant_id":     tenantID,
			"deploy_id":     deployID,
			"target":        target,
			"actor_user_id": actorUserID,
			"reason":        reason,
		},
		CreatedAt: time.Now().UTC(),
	})
}

// RecordBreakGlass records an operator break-glass production access
// event. Two-person approval is mandatory per POLICY_SECURITY.md;
// callers MUST supply the second approver's user id.
func RecordBreakGlass(ctx context.Context, store Store, operatorID, reason, twoPersonApproverID string, ttl time.Duration) (Entry, error) {
	if store == nil {
		return Entry{}, nil
	}
	return store.Record(ctx, Entry{
		Action:  EventBreakGlass,
		Outcome: OutcomeSuccess,
		UserID:  operatorID,
		Summary: "break-glass by " + operatorID + " (approver " + twoPersonApproverID + ")",
		Attrs: map[string]any{
			"event":                  EventBreakGlass,
			"operator_id":            operatorID,
			"two_person_approver_id": twoPersonApproverID,
			"reason":                 reason,
			"ttl_seconds":            int64(ttl / time.Second),
		},
		CreatedAt: time.Now().UTC(),
	})
}

// RecordAbuse records an abuse-pipeline state transition (escalation,
// throttle, suspension). eventName MUST be one of EventAbuseEscalation
// / EventAbuseThrottle / EventAbuseSuspension.
func RecordAbuse(ctx context.Context, store Store, eventName, tenantID string, score int, reason string, attrs map[string]any) (Entry, error) {
	if store == nil {
		return Entry{}, nil
	}
	merged := mergeAttrs(map[string]any{
		"event":     eventName,
		"tenant_id": tenantID,
		"score":     score,
		"reason":    reason,
	}, attrs)
	return store.Record(ctx, Entry{
		Action:    Action(eventName),
		Outcome:   OutcomeBlocked,
		UserID:    tenantID,
		Summary:   eventName + " score=" + reason,
		Attrs:     merged,
		CreatedAt: time.Now().UTC(),
	})
}

// RecordAuthLifecycle is the V22 .v1 helper for auth lifecycle events
// (signup, email verification, password reset, MFA enroll/confirm,
// email change, account close). action is the lifecycle phase string
// (e.g. "signup.completed", "mfa.enrolled").
func RecordAuthLifecycle(ctx context.Context, store Store, tenantID, userID, action string, attrs map[string]any) (Entry, error) {
	if store == nil {
		return Entry{}, nil
	}
	merged := mergeAttrs(map[string]any{
		"event":     EventAuthLifecycle,
		"tenant_id": tenantID,
		"user_id":   userID,
		"action":    action,
	}, attrs)
	return store.Record(ctx, Entry{
		Action:    EventAuthLifecycle,
		Outcome:   OutcomeSuccess,
		UserID:    userID,
		Summary:   "auth " + action,
		Attrs:     merged,
		CreatedAt: time.Now().UTC(),
	})
}

// RecordSessionChange records a session-level change (created, revoked,
// MFA-upgraded, IP migrated).
func RecordSessionChange(ctx context.Context, store Store, tenantID, userID, sessionID, change string, attrs map[string]any) (Entry, error) {
	if store == nil {
		return Entry{}, nil
	}
	merged := mergeAttrs(map[string]any{
		"event":      EventSessionChange,
		"tenant_id":  tenantID,
		"user_id":    userID,
		"session_id": sessionID,
		"change":     change,
	}, attrs)
	return store.Record(ctx, Entry{
		Action:    EventSessionChange,
		Outcome:   OutcomeSuccess,
		UserID:    userID,
		Summary:   "session " + change + " " + sessionID,
		Attrs:     merged,
		CreatedAt: time.Now().UTC(),
	})
}

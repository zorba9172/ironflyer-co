package resolver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"

	"github.com/shopspring/decimal"
	"github.com/vektah/gqlparser/v2/gqlerror"

	"ironflyer/core/orchestrator/internal/ai/agents"
	"ironflyer/core/orchestrator/internal/operations/audit"
	"ironflyer/core/orchestrator/internal/operations/auditexport"
	"ironflyer/core/orchestrator/internal/business/budget"
	"ironflyer/core/orchestrator/internal/ai/domain"
	"ironflyer/core/orchestrator/internal/operations/graph/model"
	"ironflyer/core/orchestrator/internal/operations/patch"
	"ironflyer/core/orchestrator/internal/ai/providers"
)

func agentToGraphQL(a agents.Agent) model.Agent {
	caps := make([]string, 0, len(a.Capabilities))
	for _, c := range a.Capabilities {
		caps = append(caps, string(c))
	}
	out := model.Agent{Role: string(a.Role), Capabilities: caps, EnableThinking: a.EnableThinking}
	if a.System != "" {
		out.System = stringPtr(a.System)
	}
	return out
}

func telemetryCallToGraphQL(c providers.AgentCall) model.AgentCall {
	out := model.AgentCall{
		ID:               telemetryID(c),
		Ts:               c.StartedAt,
		Provider:         c.Provider,
		PromptTokens:     c.InputTokens,
		CompletionTokens: c.OutputTokens,
		CacheReadTokens:  c.CacheReadTokens,
		CacheWriteTokens: c.CacheNewTokens,
		CostUsd:          model.NewDecimal(decimal.NewFromFloat(c.CostUSD)),
		DurationMs:       int(c.DurationMS),
		Capabilities:     append([]string(nil), c.Capabilities...),
	}
	if c.Role != "" {
		out.Role = stringPtr(c.Role)
	}
	if c.Model != "" {
		out.Model = stringPtr(c.Model)
	}
	if c.Error != "" {
		out.Error = stringPtr(c.Error)
	}
	if c.UserID != "" {
		out.UserID = stringPtr(c.UserID)
	}
	if c.ProjectID != "" {
		out.ProjectID = stringPtr(c.ProjectID)
	}
	return out
}

func agentCallToCostDelta(c providers.AgentCall) *model.CostDelta {
	out := &model.CostDelta{Ts: c.StartedAt, UsdSpent: model.NewDecimal(decimal.NewFromFloat(c.CostUSD))}
	if c.Model != "" {
		out.Model = stringPtr(c.Model)
	}
	if c.Provider != "" {
		out.Provider = stringPtr(c.Provider)
	}
	if c.Role != "" {
		out.Agent = stringPtr(c.Role)
	}
	if c.DurationMS > 0 {
		v := int(c.DurationMS)
		out.DurationMs = &v
	}
	return out
}

func telemetryID(c providers.AgentCall) string {
	sum := sha256.Sum256([]byte(c.StartedAt.UTC().Format("2006-01-02T15:04:05.000000000Z07:00") + "|" + c.UserID + "|" + c.ProjectID + "|" + c.Role + "|" + c.Provider + "|" + c.Model + "|" + strconv.FormatInt(c.DurationMS, 10)))
	return hex.EncodeToString(sum[:8])
}

func mapAuditOutcome(in model.AuditOutcome) audit.Outcome {
	switch in {
	case model.AuditOutcomeSuccess:
		return audit.OutcomeSuccess
	case model.AuditOutcomeFailure:
		return audit.OutcomeFailure
	case model.AuditOutcomeBlocked:
		return audit.OutcomeBlocked
	default:
		return audit.Outcome("")
	}
}

func auditEntryToGraphQL(e audit.Entry) model.AuditEntry {
	out := model.AuditEntry{
		ID:      e.ID,
		Ts:      e.CreatedAt,
		Action:  string(e.Action),
		Outcome: auditOutcomeToGraphQL(e.Outcome),
		Hash:    e.ContentHash,
		Payload: model.JSON(e.Attrs),
		Ok:      e.Outcome == audit.OutcomeSuccess,
	}
	if e.UserID != "" {
		out.UserID = stringPtr(e.UserID)
		out.Actor = stringPtr(e.UserID)
	}
	if e.ProjectID != "" {
		out.ProjectID = stringPtr(e.ProjectID)
		out.Resource = stringPtr(e.ProjectID)
	}
	if e.PrevHash != "" {
		out.PrevHash = stringPtr(e.PrevHash)
	}
	if e.StoryID != "" {
		out.StoryID = stringPtr(e.StoryID)
	}
	if e.GateName != "" {
		out.GateName = stringPtr(e.GateName)
	}
	if e.AgentRole != "" {
		out.AgentRole = stringPtr(e.AgentRole)
	}
	if e.Summary != "" {
		out.Summary = stringPtr(e.Summary)
	}
	if e.InputHash != "" {
		out.InputHash = stringPtr(e.InputHash)
	}
	if e.OutputHash != "" {
		out.OutputHash = stringPtr(e.OutputHash)
	}
	return out
}

func auditOutcomeToGraphQL(o audit.Outcome) model.AuditOutcome {
	switch o {
	case audit.OutcomeSuccess:
		return model.AuditOutcomeSuccess
	case audit.OutcomeBlocked:
		return model.AuditOutcomeBlocked
	case audit.OutcomeFailure:
		return model.AuditOutcomeFailure
	default:
		return model.AuditOutcomeSkipped
	}
}

func (r *queryResolver) signedExportURL(ctx context.Context, _ *model.AuditQueryInput, format auditexport.Format) (string, error) {
	u, err := currentUser(ctx)
	if err != nil {
		return "", err
	}
	cfg := r.AuditExportConfig
	if cfg.SignedURLBase == "" && r.PublicBaseURL != "" {
		cfg.SignedURLBase = r.PublicBaseURL
	}
	url, _, err := cfg.BuildDownloadURL(tenantFor(u), format)
	return url, err
}

func planToGraphQL(p budget.Plan) model.Plan {
	out := model.Plan{
		Tier:       string(p.Tier),
		Name:       p.Name,
		PriceUsd:   model.NewDecimal(p.MonthlyPrice),
		CostCapUsd: model.NewDecimal(p.CostCapUSD),
		Features:   append([]string(nil), p.AllowList...),
	}
	if len(out.Features) == 0 {
		out.Features = []string{"AI execution", "AppSec gates", "Deploy plane"}
	}
	if p.StripeID != "" {
		out.StripePriceID = stringPtr(p.StripeID)
	}
	return out
}

func ledgerEntryToGraphQL(e budget.LedgerEntry) model.LedgerEntry {
	out := model.LedgerEntry{
		ID: e.ID, UserID: e.UserID, PromptTokens: e.InputTokens, CompletionTokens: e.OutputTokens,
		CostUsd: model.NewDecimal(e.CostUSD), RevenueUsd: model.NewDecimal(e.CostUSD), Ts: e.CreatedAt,
	}
	if e.ProjectID != "" {
		out.ProjectID = stringPtr(e.ProjectID)
	}
	if e.Provider != "" {
		out.Provider = stringPtr(e.Provider)
	}
	if e.Model != "" {
		out.Model = stringPtr(e.Model)
	}
	return out
}

func gqlForbiddenOperator() *gqlerror.Error {
	return &gqlerror.Error{Message: "operator access required", Extensions: map[string]any{"code": "FORBIDDEN"}}
}

func filterIssuesBySubproject(issues []model.GateIssue, p domain.Project, sub string) []model.GateIssue {
	sub = strings.Trim(strings.TrimSpace(sub), "/")
	if sub == "" {
		return issues
	}
	if sp := p.SubprojectByPath(sub + "/"); sp != nil {
		sub = strings.Trim(sp.Path, "/")
	}
	prefix := sub + "/"
	out := make([]model.GateIssue, 0, len(issues))
	for _, i := range issues {
		if i.Path == nil {
			continue
		}
		path := strings.TrimPrefix(strings.TrimSpace(*i.Path), "./")
		if path == sub || strings.HasPrefix(path, prefix) {
			out = append(out, i)
		}
	}
	return out
}

func patchToGraphQL(p patch.Patch) *model.Patch {
	out := &model.Patch{
		ID: p.ID, ProjectID: p.ProjectID, Status: patchStatusToGraphQL(p.Status),
		CreatedAt: p.CreatedAt, AppliedAt: p.AppliedAt, Changes: make([]model.PatchChange, 0, len(p.Changes)),
	}
	if p.Title != "" {
		out.Title = stringPtr(p.Title)
	}
	if p.Summary != "" {
		out.Summary = stringPtr(p.Summary)
	}
	if p.Author != "" {
		out.Author = stringPtr(p.Author)
	}
	if p.StageID != "" {
		out.StageID = stringPtr(p.StageID)
	}
	for _, ch := range p.Changes {
		got := model.PatchChange{Op: patchChangeOpToGraphQL(ch.Op), Path: ch.Path}
		if ch.Content != "" {
			got.Content = stringPtr(ch.Content)
		}
		if ch.Anchor != "" {
			got.Anchor = stringPtr(ch.Anchor)
		}
		if ch.Replacement != "" {
			got.Replacement = stringPtr(ch.Replacement)
		}
		if ch.Symbol != nil && ch.Symbol.Name != "" {
			got.Symbol = stringPtr(ch.Symbol.Name)
		}
		out.Changes = append(out.Changes, got)
	}
	for _, c := range p.Conflicts {
		out.Conflicts = append(out.Conflicts, model.PatchConflict{Path: c.Path, Base: c.Base, Ours: c.Ours, Theirs: c.Theirs, Markers: c.Markers})
	}
	return out
}

func patchStageToGraphQL(s patch.PatchStage) *model.PatchStage {
	out := &model.PatchStage{ID: s.ID, ProjectID: s.ProjectID, Name: s.Name, PatchIds: append([]string(nil), s.PatchIDs...), Status: patchStageStatusToGraphQL(s.Status), CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt}
	if s.Description != "" {
		out.Description = stringPtr(s.Description)
	}
	if s.RejectionReason != "" {
		out.RejectionReason = stringPtr(s.RejectionReason)
	}
	return out
}

func patchStatusToGraphQL(s patch.Status) model.PatchStatus {
	switch s {
	case patch.StatusApplied:
		return model.PatchStatusApplied
	case patch.StatusRejected:
		return model.PatchStatusRejected
	case patch.StatusRolled:
		return model.PatchStatusRolledBack
	case patch.StatusConflicted:
		return model.PatchStatusConflicted
	default:
		return model.PatchStatusProposed
	}
}

func patchStageStatusToGraphQL(s patch.StageStatus) model.PatchStageStatus {
	switch s {
	case patch.StageStatusReviewed:
		return model.PatchStageStatusReviewed
	case patch.StageStatusApplied:
		return model.PatchStageStatusApplied
	case patch.StageStatusRejected:
		return model.PatchStageStatusRejected
	default:
		return model.PatchStageStatusOpen
	}
}

func patchChangeOpToGraphQL(op patch.Op) model.PatchChangeOp {
	switch op {
	case patch.OpCreate:
		return model.PatchChangeOpCreate
	case patch.OpReplace:
		return model.PatchChangeOpReplace
	case patch.OpDelete:
		return model.PatchChangeOpDelete
	case patch.OpInsertAfter:
		return model.PatchChangeOpInsertAfter
	case patch.OpSymbol:
		return model.PatchChangeOpSymbolReplace
	default:
		return model.PatchChangeOpReplace
	}
}

const inlineSystemPrompt = `You are an in-editor code completion engine.
Continue the user's code at the cursor. Output ONLY the inserted
code, no explanation, no markdown fences, no commentary. Match the
surrounding language, indentation, and style.`

func buildInlinePrompt(in model.InlineInput) string {
	var b strings.Builder
	if in.Language != nil && *in.Language != "" {
		b.WriteString("Language: ")
		b.WriteString(*in.Language)
		b.WriteByte('\n')
	}
	if in.Path != nil && *in.Path != "" {
		b.WriteString("Path: ")
		b.WriteString(*in.Path)
		b.WriteByte('\n')
	}
	b.WriteString("\n<<<PREFIX>>>\n")
	b.WriteString(in.Prefix)
	b.WriteString("\n<<<CURSOR>>>\n")
	if in.Suffix != nil {
		b.WriteString(*in.Suffix)
	}
	b.WriteString("\n<<<SUFFIX_END>>>\n")
	return b.String()
}

func inlineMaxTokens(effort *string) int {
	if effort == nil {
		return 96
	}
	switch strings.ToLower(strings.TrimSpace(*effort)) {
	case "low", "fast":
		return 48
	case "high", "extended":
		return 256
	default:
		return 96
	}
}

func stringPtr(s string) *string {
	if s == "" {
		return nil
	}
	v := s
	return &v
}

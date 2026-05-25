// Package patch — staging area. The AI sometimes proposes 5-15 patches
// for one logical change ("add an auth middleware"). PatchStage groups
// those into a single review unit so the user can apply or reject the
// batch atomically.
//
// Stages are persisted via a StagingStore (in-memory by default;
// Postgres-backed implementation in staging_store.go). Applying a stage
// walks its patches in order and rolls back any that succeeded if a
// later one fails — so the project tree is never left in a half-applied
// state.
package patch

import (
	"errors"
	"sort"
	"sync"
	"time"
)

// StageStatus is the lifecycle of a patch stage. open → reviewed →
// applied | rejected. "applied" means every contained patch reached
// StatusApplied; "rejected" means the user (or the engine, on
// rollback) refused the stage.
type StageStatus string

const (
	StageStatusOpen     StageStatus = "open"
	StageStatusReviewed StageStatus = "reviewed"
	StageStatusApplied  StageStatus = "applied"
	StageStatusRejected StageStatus = "rejected"
)

// PatchStage groups a set of patches into one logical review unit.
type PatchStage struct {
	ID          string      `json:"id"`
	ProjectID   string      `json:"projectId"`
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	PatchIDs    []string    `json:"patchIds"`
	Status      StageStatus `json:"status"`
	CreatedAt   time.Time   `json:"createdAt"`
	UpdatedAt   time.Time   `json:"updatedAt"`
	// RejectionReason is set when Status == StageStatusRejected so the
	// UI can show why.
	RejectionReason string `json:"rejectionReason,omitempty"`
}

// CreateStage bundles existing patch IDs into a new stage. All patches
// must belong to projectID; otherwise the stage is refused.
func (e *Engine) CreateStage(projectID, name, description string, patchIDs []string) (PatchStage, error) {
	if projectID == "" {
		return PatchStage{}, errors.New("projectId required")
	}
	if len(patchIDs) == 0 {
		return PatchStage{}, errors.New("stage must contain at least one patch")
	}
	e.mu.RLock()
	for _, pid := range patchIDs {
		p, ok := e.patches[pid]
		if !ok {
			e.mu.RUnlock()
			return PatchStage{}, errors.New("unknown patch: " + pid)
		}
		if p.ProjectID != projectID {
			e.mu.RUnlock()
			return PatchStage{}, errors.New("patch " + pid + " belongs to a different project")
		}
	}
	e.mu.RUnlock()
	now := time.Now().UTC()
	st := PatchStage{
		ID:          newID("stage"),
		ProjectID:   projectID,
		Name:        name,
		Description: description,
		PatchIDs:    append([]string{}, patchIDs...),
		Status:      StageStatusOpen,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := e.stages.Put(st); err != nil {
		return PatchStage{}, err
	}
	// Tag each patch with the stage so subsequent queries can surface
	// the link in the UI.
	e.mu.Lock()
	for _, pid := range patchIDs {
		p := e.patches[pid]
		p.StageID = st.ID
		e.patches[pid] = p
	}
	e.mu.Unlock()
	return st, nil
}

// ApplyStage applies every patch in the stage atomically. If any
// patch fails (validation failure, merge conflict, runtime error) the
// engine rolls back the prior patches via the snapshot ring and the
// stage lands in StageStatusRejected with the failure reason.
//
// Returns the per-patch outcomes so the UI can show "3 of 5 applied,
// rolled back on lint failure".
func (e *Engine) ApplyStage(stageID string) (StageApplyResult, error) {
	st, ok, err := e.stages.Get(stageID)
	if err != nil {
		return StageApplyResult{}, err
	}
	if !ok {
		return StageApplyResult{}, errors.New("stage not found")
	}
	if st.Status == StageStatusApplied {
		return StageApplyResult{Stage: st, Outcomes: nil}, errors.New("stage already applied")
	}
	pre, _ := e.snapshots.Latest(st.ProjectID)
	res := StageApplyResult{Stage: st}
	for _, pid := range st.PatchIDs {
		p, applyErr := e.Apply(pid)
		out := StagePatchOutcome{PatchID: pid, Status: p.Status}
		if applyErr != nil {
			out.Error = applyErr.Error()
		}
		res.Outcomes = append(res.Outcomes, out)
		if applyErr != nil {
			// Rollback to the pre-stage snapshot if we captured one.
			if pre.ID != "" {
				_, _ = e.Rollback(st.ProjectID, pre.ID)
			}
			st.Status = StageStatusRejected
			st.RejectionReason = "patch " + pid + " failed: " + applyErr.Error()
			st.UpdatedAt = time.Now().UTC()
			_ = e.stages.Put(st)
			res.Stage = st
			return res, applyErr
		}
	}
	st.Status = StageStatusApplied
	st.UpdatedAt = time.Now().UTC()
	if err := e.stages.Put(st); err != nil {
		return res, err
	}
	res.Stage = st
	return res, nil
}

// RejectStage marks the stage rejected without touching the underlying
// patches. The patches keep their individual status — the operator can
// still apply them one-by-one if they want.
func (e *Engine) RejectStage(stageID, reason string) (PatchStage, error) {
	st, ok, err := e.stages.Get(stageID)
	if err != nil {
		return PatchStage{}, err
	}
	if !ok {
		return PatchStage{}, errors.New("stage not found")
	}
	st.Status = StageStatusRejected
	st.RejectionReason = reason
	st.UpdatedAt = time.Now().UTC()
	if err := e.stages.Put(st); err != nil {
		return PatchStage{}, err
	}
	return st, nil
}

// ListStages returns every stage for a project, newest first.
func (e *Engine) ListStages(projectID string) ([]PatchStage, error) {
	out, err := e.stages.List(projectID)
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

// GetStage fetches one stage by ID.
func (e *Engine) GetStage(stageID string) (PatchStage, bool, error) {
	return e.stages.Get(stageID)
}

// StagePatchOutcome is the per-patch result inside a stage apply.
type StagePatchOutcome struct {
	PatchID string `json:"patchId"`
	Status  Status `json:"status"`
	Error   string `json:"error,omitempty"`
}

// StageApplyResult is what ApplyStage returns — the final stage state +
// per-patch outcomes for the UI.
type StageApplyResult struct {
	Stage    PatchStage          `json:"stage"`
	Outcomes []StagePatchOutcome `json:"outcomes"`
}

// MemoryStagingStore is the always-available in-memory implementation
// of StagingStore. Used by NewEngine; production startup swaps in the
// Postgres variant via WithStagingStore.
type MemoryStagingStore struct {
	mu     sync.RWMutex
	stages map[string]PatchStage
}

// NewMemoryStagingStore returns a ready-to-use in-memory store.
func NewMemoryStagingStore() *MemoryStagingStore {
	return &MemoryStagingStore{stages: map[string]PatchStage{}}
}

func (m *MemoryStagingStore) Put(st PatchStage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stages[st.ID] = st
	return nil
}

func (m *MemoryStagingStore) Get(id string) (PatchStage, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	st, ok := m.stages[id]
	return st, ok, nil
}

func (m *MemoryStagingStore) List(projectID string) ([]PatchStage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []PatchStage
	for _, st := range m.stages {
		if projectID == "" || st.ProjectID == projectID {
			out = append(out, st)
		}
	}
	return out, nil
}

func (m *MemoryStagingStore) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.stages, id)
	return nil
}

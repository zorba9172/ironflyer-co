// Package patch — transactional snapshot + rollback. A snapshot captures
// the full Project.Files slice at the moment a patch is about to be applied
// so that, if a follow-up gate verification fails, the engine can restore
// the previous tree with a single atomic Update on the store.
//
// We keep snapshots in-process and bounded (most recent N per project).
// Persistent / git-backed snapshots can layer on top later by implementing
// a SnapshotStore; the public API here is the same.
package patch

import (
	"errors"
	"sync"
	"time"

	"ironflyer/core/orchestrator/internal/ai/domain"
)

const defaultSnapshotsPerProject = 32

// Snapshot captures the project file tree at a point in time, tagged with
// the patch that the snapshot was taken to protect. Rollback restores the
// captured slice byte-for-byte.
type Snapshot struct {
	ID        string             `json:"id"`
	ProjectID string             `json:"projectId"`
	PatchID   string             `json:"patchId,omitempty"`
	Title     string             `json:"title,omitempty"`
	Files     []domain.FileNode  `json:"files"`
	Spec      *domain.ProductSpec `json:"spec,omitempty"`
	CreatedAt time.Time          `json:"createdAt"`
}

// snapshotStore is the in-memory ring buffer per project. NewEngine wires
// one in automatically; tests can swap in their own.
type snapshotStore struct {
	mu       sync.Mutex
	perProj  map[string][]Snapshot
	capacity int
}

func newSnapshotStore() *snapshotStore {
	return &snapshotStore{perProj: map[string][]Snapshot{}, capacity: defaultSnapshotsPerProject}
}

// Push adds s to projectID's ring, evicting the oldest entry when the
// per-project cap is exceeded.
func (s *snapshotStore) Push(snap Snapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	list := s.perProj[snap.ProjectID]
	list = append(list, snap)
	if len(list) > s.capacity {
		list = list[len(list)-s.capacity:]
	}
	s.perProj[snap.ProjectID] = list
}

// Latest returns the most recent snapshot for a project, or (zero, false).
func (s *snapshotStore) Latest(projectID string) (Snapshot, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	list := s.perProj[projectID]
	if len(list) == 0 {
		return Snapshot{}, false
	}
	return list[len(list)-1], true
}

// Get returns the snapshot with the given ID, scoped to projectID.
func (s *snapshotStore) Get(projectID, snapID string) (Snapshot, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, snap := range s.perProj[projectID] {
		if snap.ID == snapID {
			return snap, true
		}
	}
	return Snapshot{}, false
}

// List returns the snapshots known for a project, oldest first.
func (s *snapshotStore) List(projectID string) []Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	src := s.perProj[projectID]
	out := make([]Snapshot, len(src))
	copy(out, src)
	return out
}

// Snapshot the current project state before a patch is applied. Called
// internally by Engine.Apply but also exposed so an operator could force
// a checkpoint via an HTTP route.
func (e *Engine) Snapshot(projectID, patchID, title string) (Snapshot, error) {
	proj, err := e.projects.Get(projectID)
	if err != nil {
		return Snapshot{}, err
	}
	files := make([]domain.FileNode, len(proj.Files))
	copy(files, proj.Files)
	specCopy := proj.Spec
	snap := Snapshot{
		ID:        newID("snap"),
		ProjectID: projectID,
		PatchID:   patchID,
		Title:     title,
		Files:     files,
		Spec:      &specCopy,
		CreatedAt: time.Now().UTC(),
	}
	e.snapshots.Push(snap)
	return snap, nil
}

// Rollback restores the project tree to the most recent snapshot (or a
// specific snapshot if snapID != ""). Returns the snapshot that was
// applied and stamps a "rolled-back" event on the project history so the
// SSE timeline shows the transaction unwound.
func (e *Engine) Rollback(projectID, snapID string) (Snapshot, error) {
	var (
		snap Snapshot
		ok   bool
	)
	if snapID == "" {
		snap, ok = e.snapshots.Latest(projectID)
	} else {
		snap, ok = e.snapshots.Get(projectID, snapID)
	}
	if !ok {
		return Snapshot{}, errors.New("no snapshot for project")
	}

	_, err := e.projects.Update(projectID, func(proj *domain.Project) {
		files := make([]domain.FileNode, len(snap.Files))
		copy(files, snap.Files)
		proj.Files = files
		if snap.Spec != nil {
			proj.Spec = *snap.Spec
		}
		proj.Events = append(proj.Events, domain.Event{
			ID:        newID("evt"),
			Step:      "rollback",
			Message:   "rolled back to snapshot " + snap.ID,
			Status:    "done",
			CreatedAt: time.Now().UTC(),
		})
	})
	if err != nil {
		return Snapshot{}, err
	}

	// Mark any patches applied AFTER this snapshot as rolled-back.
	var rolled []Patch
	e.mu.Lock()
	for id, p := range e.patches {
		if p.ProjectID != projectID {
			continue
		}
		if p.AppliedAt != nil && !p.AppliedAt.Before(snap.CreatedAt) {
			p.Status = StatusRolled
			e.patches[id] = p
			rolled = append(rolled, p)
		}
	}
	cb := e.onRolledBack
	e.mu.Unlock()
	if cb != nil {
		for _, p := range rolled {
			cb(p, snap.ID)
		}
	}
	return snap, nil
}

// Snapshots returns the in-memory snapshot log for a project, oldest
// first. Empty slice when no snapshots have been taken.
func (e *Engine) Snapshots(projectID string) []Snapshot {
	return e.snapshots.List(projectID)
}

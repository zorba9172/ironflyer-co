package snapshots

import (
	"sync"
	"sync/atomic"
	"time"
)

// Metrics is an additive in-process counter set. Designed so a future
// Prometheus exporter can read these fields without holding a mutex.
type Metrics struct {
	RestoreCount       atomic.Int64
	RestoreBytes       atomic.Int64
	RestoreFailures    atomic.Int64
	CheckpointCount    atomic.Int64
	CheckpointBytes    atomic.Int64
	CheckpointFailures atomic.Int64
	ArchiveCount       atomic.Int64
	ArchiveBytes       atomic.Int64
	ArchiveFailures    atomic.Int64

	mu           sync.Mutex
	lastRestore  time.Duration
	lastCheckpt  time.Duration
}

// ObserveRestore records a successful restore.
func (m *Metrics) ObserveRestore(bytes int64, d time.Duration) {
	if m == nil {
		return
	}
	m.RestoreCount.Add(1)
	m.RestoreBytes.Add(bytes)
	m.mu.Lock()
	m.lastRestore = d
	m.mu.Unlock()
}

// ObserveCheckpoint records a successful checkpoint.
func (m *Metrics) ObserveCheckpoint(bytes int64, d time.Duration) {
	if m == nil {
		return
	}
	m.CheckpointCount.Add(1)
	m.CheckpointBytes.Add(bytes)
	m.mu.Lock()
	m.lastCheckpt = d
	m.mu.Unlock()
}

// ObserveArchive records a successful archive operation.
func (m *Metrics) ObserveArchive(bytes int64) {
	if m == nil {
		return
	}
	m.ArchiveCount.Add(1)
	m.ArchiveBytes.Add(bytes)
}

// ObserveRestoreFailure increments the restore-failure counter.
func (m *Metrics) ObserveRestoreFailure() {
	if m == nil {
		return
	}
	m.RestoreFailures.Add(1)
}

// ObserveCheckpointFailure increments the checkpoint-failure counter.
func (m *Metrics) ObserveCheckpointFailure() {
	if m == nil {
		return
	}
	m.CheckpointFailures.Add(1)
}

// ObserveArchiveFailure increments the archive-failure counter.
func (m *Metrics) ObserveArchiveFailure() {
	if m == nil {
		return
	}
	m.ArchiveFailures.Add(1)
}

// LastRestoreDuration returns the most recent successful restore time.
func (m *Metrics) LastRestoreDuration() time.Duration {
	if m == nil {
		return 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastRestore
}

// LastCheckpointDuration returns the most recent successful checkpoint
// time.
func (m *Metrics) LastCheckpointDuration() time.Duration {
	if m == nil {
		return 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastCheckpt
}

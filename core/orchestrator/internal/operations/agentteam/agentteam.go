// Package agentteam holds the operator-defined agent layer that sits on top of
// the built-in finisher roster: custom agents (a mission + skills + tools +
// guardrails + a model + an autonomy level) and crews (several agents grouped
// to run together — in parallel, in a chain, or under a manager).
//
// These are owner-scoped, persisted resources. The store is in-memory now and
// swaps to Postgres later behind the same interface, matching the convention in
// internal/operations/store.
package agentteam

import "time"

// Autonomy mirrors the studio's agent-autonomy levels and the orchestrator's
// patch lifecycle: suggest = propose only, approval = apply behind a human
// gate, autonomous = apply within budget + guardrails without waiting.
type Autonomy string

const (
	AutonomySuggest    Autonomy = "suggest"
	AutonomyApproval   Autonomy = "approval"
	AutonomyAutonomous Autonomy = "autonomous"
)

// Process decides how a crew's members collaborate.
type Process string

const (
	// ProcessParallel runs every member at once as a worker. Fastest.
	ProcessParallel Process = "parallel"
	// ProcessSequential runs members in order, each handing off to the next.
	ProcessSequential Process = "sequential"
	// ProcessHierarchical has a manager agent plan and delegate to members.
	ProcessHierarchical Process = "hierarchical"
)

// Schedule is when an agent or crew runs on its own. Mirrors the studio's
// AgentSchedule. Empty Mode ("" / "manual") means dispatch-only.
type Schedule struct {
	Mode    string `json:"mode"`
	Every   string `json:"every,omitempty"`
	At      string `json:"at,omitempty"`
	Weekday *int   `json:"weekday,omitempty"`
	Trigger string `json:"trigger,omitempty"`
	Enabled bool   `json:"enabled"`
}

// CustomAgent is an operator-composed agent. BaseRole binds it to a runnable
// role in the agent registry (planner/coder/security/…) so a crew can actually
// dispatch it; the rest is configuration the orchestrator threads into the run.
type CustomAgent struct {
	ID               string    `json:"id"`
	OwnerID          string    `json:"ownerId"`
	Name             string    `json:"name"`
	Role             string    `json:"role"` // one-line objective
	Description      string    `json:"description,omitempty"`
	Instructions     string    `json:"instructions,omitempty"`
	BaseRole         string    `json:"baseRole,omitempty"` // registry Role to execute as
	GateID           string    `json:"gateId,omitempty"`
	Skills           []string  `json:"skills,omitempty"`
	Tools            []string  `json:"tools,omitempty"`
	Responsibilities []string  `json:"responsibilities,omitempty"`
	Guardrails       []string  `json:"guardrails,omitempty"`
	Knowledge        []string  `json:"knowledge,omitempty"`
	Model            string    `json:"model,omitempty"`
	Autonomy         Autonomy  `json:"autonomy,omitempty"`
	CanDelegate      bool      `json:"canDelegate,omitempty"`
	HandoffTo        []string  `json:"handoffTo,omitempty"`
	Schedule         *Schedule `json:"schedule,omitempty"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

// Crew groups agents to run together toward one goal.
type Crew struct {
	ID        string    `json:"id"`
	OwnerID   string    `json:"ownerId"`
	Name      string    `json:"name"`
	Goal      string    `json:"goal"`
	Process   Process   `json:"process"`
	MemberIDs []string  `json:"memberIds"`
	ManagerID string    `json:"managerId,omitempty"`
	Schedule  *Schedule `json:"schedule,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// MemberResult is one agent's output from a crew run.
type MemberResult struct {
	AgentID  string  `json:"agentId"`
	Name     string  `json:"name"`
	Role     string  `json:"role"`
	Output   string  `json:"output"`
	Provider string  `json:"provider,omitempty"`
	Tokens   int     `json:"tokens,omitempty"`
	CostUSD  float64 `json:"costUsd"`
	Error    string  `json:"error,omitempty"`
	StartedAt time.Time `json:"startedAt"`
	EndedAt   time.Time `json:"endedAt"`
}

// RunResult is the outcome of dispatching a crew.
type RunResult struct {
	CrewID       string         `json:"crewId"`
	Process      Process        `json:"process"`
	Members      []MemberResult `json:"members"`
	TotalCostUSD float64        `json:"totalCostUsd"`
	StartedAt    time.Time      `json:"startedAt"`
	EndedAt      time.Time      `json:"endedAt"`
}

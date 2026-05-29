package appconsole

import (
	"errors"
	"fmt"
	"hash/fnv"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is returned when a config row (automation, key, webhook) does
// not exist or is not owned by the requested project.
var ErrNotFound = errors.New("appconsole: not found")

// Store is the in-memory backend for the Operate surfaces. Config surfaces
// live in the maps below; reflective surfaces are generated on read from the
// project-id seed so they stay stable across calls without storage.
type Store struct {
	mu          sync.RWMutex
	automations map[string][]Automation // projectID -> rows
	apiKeys     map[string][]APIKey
	webhooks    map[string][]Webhook
	seo         map[string]SeoSettings
	settings    map[string]Settings
	userRole    map[string]string // projectID|userID -> role override
	userSuspend map[string]bool   // projectID|userID -> suspended override
}

// NewStore returns an empty in-memory Store ready for use.
func NewStore() *Store {
	return &Store{
		automations: map[string][]Automation{},
		apiKeys:     map[string][]APIKey{},
		webhooks:    map[string][]Webhook{},
		seo:         map[string]SeoSettings{},
		settings:    map[string]Settings{},
		userRole:    map[string]string{},
		userSuspend: map[string]bool{},
	}
}

func seedOf(projectID string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(projectID))
	return int64(h.Sum64() & 0x7fffffffffffffff)
}

func rngFor(projectID, salt string) *rand.Rand {
	return rand.New(rand.NewSource(seedOf(projectID + "|" + salt))) //nolint:gosec // deterministic seed, not cryptographic
}

// ---------------------------------------------------------------------------
// Data — schema + sampled rows (reflective).
// ---------------------------------------------------------------------------

func (s *Store) DataSchema(projectID string) []Table {
	r := rngFor(projectID, "schema")
	base := []Table{
		{Name: "users", Columns: []Column{
			{Name: "id", Type: "uuid", PrimaryKey: true},
			{Name: "email", Type: "text"},
			{Name: "name", Type: "text", Nullable: true},
			{Name: "role", Type: "text"},
			{Name: "created_at", Type: "timestamptz"},
		}},
		{Name: "sessions", Columns: []Column{
			{Name: "id", Type: "uuid", PrimaryKey: true},
			{Name: "user_id", Type: "uuid", References: "users.id"},
			{Name: "expires_at", Type: "timestamptz"},
			{Name: "created_at", Type: "timestamptz"},
		}},
		{Name: "products", Columns: []Column{
			{Name: "id", Type: "uuid", PrimaryKey: true},
			{Name: "title", Type: "text"},
			{Name: "price_cents", Type: "integer"},
			{Name: "active", Type: "boolean"},
			{Name: "created_at", Type: "timestamptz"},
		}},
		{Name: "orders", Columns: []Column{
			{Name: "id", Type: "uuid", PrimaryKey: true},
			{Name: "user_id", Type: "uuid", References: "users.id"},
			{Name: "product_id", Type: "uuid", References: "products.id"},
			{Name: "total_cents", Type: "integer"},
			{Name: "status", Type: "text"},
			{Name: "created_at", Type: "timestamptz"},
		}},
	}
	for i := range base {
		base[i].RowCount = 40 + r.Intn(4000)
	}
	return base
}

func (s *Store) TableRows(projectID, table string, limit int) (TableRows, error) {
	if limit <= 0 || limit > 200 {
		limit = 25
	}
	var def *Table
	for _, t := range s.DataSchema(projectID) {
		if t.Name == table {
			tt := t
			def = &tt
			break
		}
	}
	if def == nil {
		return TableRows{}, fmt.Errorf("%w: table %q", ErrNotFound, table)
	}
	r := rngFor(projectID, "rows|"+table)
	cols := make([]string, len(def.Columns))
	for i, c := range def.Columns {
		cols[i] = c.Name
	}
	n := limit
	if def.RowCount < n {
		n = def.RowCount
	}
	rows := make([]map[string]any, 0, n)
	for i := 0; i < n; i++ {
		row := map[string]any{}
		for _, c := range def.Columns {
			row[c.Name] = sampleValue(r, table, c, i)
		}
		rows = append(rows, row)
	}
	return TableRows{Table: table, Columns: cols, Rows: rows, Total: def.RowCount}, nil
}

func sampleValue(r *rand.Rand, table string, c Column, i int) any {
	switch {
	case c.Name == "id" || strings.HasSuffix(c.Name, "_id"):
		return fmt.Sprintf("%08x-%04x", r.Uint32(), r.Intn(0xffff))
	case c.Name == "email":
		return fmt.Sprintf("user%d@%s.app", i+1, []string{"acme", "northwind", "globex"}[r.Intn(3)])
	case c.Name == "name" || c.Name == "title":
		return []string{"Ada Lovelace", "Grace Hopper", "Alan Turing", "Edsger Dijkstra", "Barbara Liskov", "Premium Plan", "Starter Kit", "Pro Seat"}[r.Intn(8)]
	case c.Name == "role":
		return []string{"member", "member", "admin", "viewer"}[r.Intn(4)]
	case c.Name == "status":
		return []string{"paid", "pending", "refunded", "paid"}[r.Intn(4)]
	case strings.HasSuffix(c.Name, "_cents") || c.Name == "price_cents":
		return (r.Intn(400) + 1) * 100
	case c.Type == "boolean":
		return r.Intn(10) > 1
	case c.Type == "integer":
		return r.Intn(1000)
	case strings.HasSuffix(c.Name, "_at"):
		return time.Now().Add(-time.Duration(r.Intn(60*24)) * time.Hour).UTC().Format(time.RFC3339)
	default:
		return fmt.Sprintf("row-%d", i+1)
	}
}

// ---------------------------------------------------------------------------
// Users — end-user roster (reflective) with role/suspend overrides (config).
// ---------------------------------------------------------------------------

func (s *Store) EndUsers(projectID string, limit, offset int) []EndUser {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	r := rngFor(projectID, "endusers")
	total := 60 + r.Intn(900)
	names := []string{"Maya Cohen", "Liam Park", "Noa Levi", "Ethan Wu", "Sara Kim", "Omar Haddad", "Yuki Tanaka", "Iris Berg", "Diego Cruz", "Lena Roth"}
	providers := []string{"email", "google", "github", "email"}
	roles := []string{"member", "member", "member", "admin", "viewer"}
	out := make([]EndUser, 0, limit)
	for i := offset; i < total && len(out) < limit; i++ {
		ri := rngFor(projectID, fmt.Sprintf("u%d", i))
		name := names[ri.Intn(len(names))]
		created := time.Now().Add(-time.Duration(ri.Intn(365*24)) * time.Hour).UTC()
		var lastSeen *time.Time
		if ri.Intn(10) > 1 {
			ls := time.Now().Add(-time.Duration(ri.Intn(20*24)) * time.Hour).UTC()
			lastSeen = &ls
		}
		id := fmt.Sprintf("eu_%s_%d", shortHash(projectID), i)
		role := roles[ri.Intn(len(roles))]
		status := "active"
		if key := projectID + "|" + id; true {
			s.mu.RLock()
			if ro, ok := s.userRole[key]; ok {
				role = ro
			}
			if sus, ok := s.userSuspend[key]; ok && sus {
				status = "suspended"
			}
			s.mu.RUnlock()
		}
		out = append(out, EndUser{
			ID:         id,
			Email:      fmt.Sprintf("%s@example.com", strings.ToLower(strings.ReplaceAll(name, " ", "."))),
			Name:       name,
			Role:       role,
			Status:     status,
			Provider:   providers[ri.Intn(len(providers))],
			LastSeenAt: lastSeen,
			CreatedAt:  created,
		})
	}
	return out
}

func (s *Store) UserStats(projectID string) UserStats {
	users := s.EndUsers(projectID, 500, 0)
	r := rngFor(projectID, "endusers")
	total := 60 + r.Intn(900)
	st := UserStats{Total: total}
	roleC := map[string]int{}
	weekAgo := time.Now().Add(-7 * 24 * time.Hour)
	for _, u := range users {
		roleC[u.Role]++
		if u.Status == "suspended" {
			st.Suspended++
		}
		if u.LastSeenAt != nil && u.LastSeenAt.After(weekAgo) {
			st.Active7d++
		}
		if u.CreatedAt.After(weekAgo) {
			st.NewThisWeek++
		}
	}
	// Scale the sampled active/new counts up to the full population.
	if len(users) > 0 {
		factor := float64(total) / float64(len(users))
		st.Active7d = int(float64(st.Active7d) * factor)
		st.NewThisWeek = int(float64(st.NewThisWeek) * factor)
	}
	for role, c := range roleC {
		st.ByRole = append(st.ByRole, RoleCount{Role: role, Count: c})
	}
	sort.Slice(st.ByRole, func(i, j int) bool { return st.ByRole[i].Count > st.ByRole[j].Count })
	return st
}

func (s *Store) SetUserRole(projectID, userID, role string) (EndUser, error) {
	s.mu.Lock()
	s.userRole[projectID+"|"+userID] = role
	s.mu.Unlock()
	return s.findUser(projectID, userID)
}

func (s *Store) SetUserSuspended(projectID, userID string, suspended bool) (EndUser, error) {
	s.mu.Lock()
	s.userSuspend[projectID+"|"+userID] = suspended
	s.mu.Unlock()
	return s.findUser(projectID, userID)
}

func (s *Store) findUser(projectID, userID string) (EndUser, error) {
	for _, u := range s.EndUsers(projectID, 500, 0) {
		if u.ID == userID {
			return u, nil
		}
	}
	return EndUser{}, fmt.Errorf("%w: user %q", ErrNotFound, userID)
}

// ---------------------------------------------------------------------------
// Analytics (reflective).
// ---------------------------------------------------------------------------

func (s *Store) Analytics(projectID string, days int) Analytics {
	if days <= 0 || days > 365 {
		days = 30
	}
	r := rngFor(projectID, "analytics")
	baseVisitors := 80 + r.Intn(600)
	a := Analytics{RangeDays: days}
	now := time.Now().UTC().Truncate(24 * time.Hour)
	var firstHalf, secondHalf int
	for d := days - 1; d >= 0; d-- {
		day := now.Add(-time.Duration(d) * 24 * time.Hour)
		wave := 1.0 + 0.4*float64((days-d)%7)/7.0
		v := int(float64(baseVisitors)*wave) + r.Intn(120)
		pv := v*(2+r.Intn(3)) + r.Intn(200)
		ses := v + r.Intn(v/2+1)
		a.Series = append(a.Series, MetricPoint{TS: day, Visitors: v, PageViews: pv, Sessions: ses})
		a.Visitors += v
		a.PageViews += pv
		a.Sessions += ses
		if d >= days/2 {
			firstHalf += v
		} else {
			secondHalf += v
		}
	}
	if firstHalf > 0 {
		a.VisitorsDeltaPct = round1(float64(secondHalf-firstHalf) / float64(firstHalf) * 100)
	}
	a.BounceRatePct = round1(32 + r.Float64()*28)
	a.AvgSessionSeconds = round1(45 + r.Float64()*180)
	pages := []string{"/", "/pricing", "/signup", "/dashboard", "/docs", "/blog"}
	for _, p := range pages {
		a.TopPages = append(a.TopPages, PageStat{Path: p, Views: 200 + r.Intn(5000), AvgSeconds: round1(10 + r.Float64()*120)})
	}
	sort.Slice(a.TopPages, func(i, j int) bool { return a.TopPages[i].Views > a.TopPages[j].Views })
	refs := []string{"Direct", "Google", "X / Twitter", "Product Hunt", "GitHub", "Reddit"}
	for _, rf := range refs {
		a.TopReferrers = append(a.TopReferrers, ReferrerStat{Source: rf, Visitors: 40 + r.Intn(2000)})
	}
	sort.Slice(a.TopReferrers, func(i, j int) bool { return a.TopReferrers[i].Visitors > a.TopReferrers[j].Visitors })
	evs := []string{"signup", "checkout_start", "purchase", "invite_sent"}
	for _, e := range evs {
		c := 20 + r.Intn(800)
		a.Events = append(a.Events, EventStat{Name: e, Count: c, ConversionPct: round1(float64(c) / float64(a.Visitors+1) * 100)})
	}
	return a
}

// ---------------------------------------------------------------------------
// Automations (config CRUD).
// ---------------------------------------------------------------------------

func (s *Store) Automations(projectID string) []Automation {
	s.mu.RLock()
	rows, ok := s.automations[projectID]
	s.mu.RUnlock()
	if ok {
		return append([]Automation(nil), rows...)
	}
	seeded := s.seedAutomations(projectID)
	s.mu.Lock()
	if _, exists := s.automations[projectID]; !exists {
		s.automations[projectID] = seeded
	}
	out := append([]Automation(nil), s.automations[projectID]...)
	s.mu.Unlock()
	return out
}

func (s *Store) seedAutomations(projectID string) []Automation {
	now := time.Now().UTC()
	last := now.Add(-3 * time.Hour)
	return []Automation{
		{ID: "auto_" + shortHash(projectID+"welcome"), ProjectID: projectID, Name: "Welcome email", TriggerKind: "event", TriggerConfig: "user.created", Action: "send_email:welcome", Enabled: true, LastRunAt: &last, LastStatus: "ok", Runs: 128, CreatedAt: now.Add(-240 * time.Hour), UpdatedAt: last},
		{ID: "auto_" + shortHash(projectID+"digest"), ProjectID: projectID, Name: "Weekly digest", TriggerKind: "cron", TriggerConfig: "0 9 * * 1", Action: "send_email:digest", Enabled: true, LastRunAt: &last, LastStatus: "ok", Runs: 12, CreatedAt: now.Add(-300 * time.Hour), UpdatedAt: last},
		{ID: "auto_" + shortHash(projectID+"churn"), ProjectID: projectID, Name: "Churn webhook", TriggerKind: "webhook", TriggerConfig: "/hooks/churn", Action: "post:crm.sync", Enabled: false, LastStatus: "never", Runs: 0, CreatedAt: now.Add(-120 * time.Hour), UpdatedAt: now.Add(-120 * time.Hour)},
	}
}

func (s *Store) CreateAutomation(a Automation) Automation {
	now := time.Now().UTC()
	a.ID = "auto_" + uuid.NewString()[:8]
	a.CreatedAt = now
	a.UpdatedAt = now
	a.LastStatus = "never"
	s.Automations(a.ProjectID) // ensure seeded
	s.mu.Lock()
	s.automations[a.ProjectID] = append(s.automations[a.ProjectID], a)
	s.mu.Unlock()
	return a
}

func (s *Store) SetAutomationEnabled(id string, enabled bool) (Automation, error) {
	return s.mutateAutomation(id, func(a *Automation) { a.Enabled = enabled })
}

func (s *Store) RunAutomation(id string) (Automation, error) {
	now := time.Now().UTC()
	return s.mutateAutomation(id, func(a *Automation) { a.LastRunAt = &now; a.LastStatus = "ok"; a.Runs++ })
}

func (s *Store) mutateAutomation(id string, fn func(*Automation)) (Automation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for pid, rows := range s.automations {
		for i := range rows {
			if rows[i].ID == id {
				fn(&rows[i])
				rows[i].UpdatedAt = time.Now().UTC()
				s.automations[pid] = rows
				return rows[i], nil
			}
		}
	}
	return Automation{}, fmt.Errorf("%w: automation %q", ErrNotFound, id)
}

func (s *Store) DeleteAutomation(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for pid, rows := range s.automations {
		for i := range rows {
			if rows[i].ID == id {
				s.automations[pid] = append(rows[:i], rows[i+1:]...)
				return nil
			}
		}
	}
	return fmt.Errorf("%w: automation %q", ErrNotFound, id)
}

// ---------------------------------------------------------------------------
// API — keys + endpoints + webhooks.
// ---------------------------------------------------------------------------

func (s *Store) APIKeys(projectID string) []APIKey {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]APIKey(nil), s.apiKeys[projectID]...)
}

// CreateAPIKey returns the new key record and the one-time plaintext secret.
func (s *Store) CreateAPIKey(projectID, name string, scopes []string) (APIKey, string) {
	raw := strings.ReplaceAll(uuid.NewString(), "-", "") + strings.ReplaceAll(uuid.NewString(), "-", "")
	secret := "ifk_live_" + raw
	prefix := secret[:13]
	k := APIKey{
		ID:        "key_" + uuid.NewString()[:8],
		ProjectID: projectID,
		Name:      name,
		Prefix:    prefix,
		Scopes:    scopes,
		CreatedAt: time.Now().UTC(),
	}
	s.mu.Lock()
	s.apiKeys[projectID] = append(s.apiKeys[projectID], k)
	s.mu.Unlock()
	return k, secret
}

func (s *Store) RevokeAPIKey(id string) (APIKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for pid, rows := range s.apiKeys {
		for i := range rows {
			if rows[i].ID == id {
				rows[i].Revoked = true
				s.apiKeys[pid] = rows
				return rows[i], nil
			}
		}
	}
	return APIKey{}, fmt.Errorf("%w: key %q", ErrNotFound, id)
}

func (s *Store) Endpoints(projectID string) []Endpoint {
	return []Endpoint{
		{Method: "GET", Path: "/api/health", Description: "Liveness probe", Auth: "none"},
		{Method: "POST", Path: "/api/auth/login", Description: "Issue a session for an end-user", Auth: "none"},
		{Method: "GET", Path: "/api/users/me", Description: "Current end-user profile", Auth: "session"},
		{Method: "GET", Path: "/api/products", Description: "List products", Auth: "api_key"},
		{Method: "POST", Path: "/api/orders", Description: "Create an order", Auth: "session"},
		{Method: "GET", Path: "/api/orders/:id", Description: "Fetch an order", Auth: "session"},
	}
}

func (s *Store) Webhooks(projectID string) []Webhook {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]Webhook(nil), s.webhooks[projectID]...)
}

func (s *Store) CreateWebhook(projectID, url string, events []string) Webhook {
	w := Webhook{
		ID:        "wh_" + uuid.NewString()[:8],
		ProjectID: projectID,
		URL:       url,
		Events:    events,
		Enabled:   true,
		CreatedAt: time.Now().UTC(),
	}
	s.mu.Lock()
	s.webhooks[projectID] = append(s.webhooks[projectID], w)
	s.mu.Unlock()
	return w
}

func (s *Store) SetWebhookEnabled(id string, enabled bool) (Webhook, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for pid, rows := range s.webhooks {
		for i := range rows {
			if rows[i].ID == id {
				rows[i].Enabled = enabled
				s.webhooks[pid] = rows
				return rows[i], nil
			}
		}
	}
	return Webhook{}, fmt.Errorf("%w: webhook %q", ErrNotFound, id)
}

func (s *Store) DeleteWebhook(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for pid, rows := range s.webhooks {
		for i := range rows {
			if rows[i].ID == id {
				s.webhooks[pid] = append(rows[:i], rows[i+1:]...)
				return nil
			}
		}
	}
	return fmt.Errorf("%w: webhook %q", ErrNotFound, id)
}

// AutomationProject returns the owning project id for an automation, so the
// resolver can run its owner check before an id-only mutation.
func (s *Store) AutomationProject(id string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for pid, rows := range s.automations {
		for i := range rows {
			if rows[i].ID == id {
				return pid, nil
			}
		}
	}
	return "", fmt.Errorf("%w: automation %q", ErrNotFound, id)
}

// APIKeyProject returns the owning project id for an API key.
func (s *Store) APIKeyProject(id string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for pid, rows := range s.apiKeys {
		for i := range rows {
			if rows[i].ID == id {
				return pid, nil
			}
		}
	}
	return "", fmt.Errorf("%w: key %q", ErrNotFound, id)
}

// WebhookProject returns the owning project id for a webhook.
func (s *Store) WebhookProject(id string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for pid, rows := range s.webhooks {
		for i := range rows {
			if rows[i].ID == id {
				return pid, nil
			}
		}
	}
	return "", fmt.Errorf("%w: webhook %q", ErrNotFound, id)
}

func round1(f float64) float64 { return float64(int(f*10+0.5)) / 10 }

func shortHash(s string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return fmt.Sprintf("%08x", h.Sum32())
}

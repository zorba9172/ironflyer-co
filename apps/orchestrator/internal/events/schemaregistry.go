package events

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// Subject is "<topic>-<event_type>" per ARCHITECTURE_EVENTS.md.
type Subject string

// RegisteredSchema is one immutable version of a JSON Schema document
// for a subject. The registry returns these by lookup (Get/Latest).
type RegisteredSchema struct {
	Subject Subject
	Version int
	Schema  string // raw JSON Schema document
}

// Registry is the producer-side schema registry contract. Two concrete
// implementations ship: NewMemoryRegistry for tests/dev, and
// NewHTTPRegistry as a skeleton for Karapace/Confluent-compatible HTTP.
type Registry interface {
	Register(ctx context.Context, subject Subject, jsonSchema string) (RegisteredSchema, error)
	Get(ctx context.Context, subject Subject, version int) (RegisteredSchema, error)
	Latest(ctx context.Context, subject Subject) (RegisteredSchema, error)
	Validate(ctx context.Context, subject Subject, payload []byte) error
	// Check runs the compatibility rule (backward by default) for the
	// candidate schema against the latest registered version.
	Check(ctx context.Context, subject Subject, jsonSchema string) error
}

var (
	// ErrSchemaValidation is returned by Registry.Validate when the
	// payload does not satisfy the registered JSON Schema. Producers
	// MUST treat this as a permanent failure and reject the write.
	ErrSchemaValidation = errors.New("events: payload failed schema validation")
	// ErrSchemaNotFound means the subject has no registered version.
	// Producers SHOULD log a warning and proceed (so flows that predate
	// schema registration don't break).
	ErrSchemaNotFound = errors.New("events: schema subject not registered")
)

// SubjectFor stamps the canonical subject name.
func SubjectFor(topic, eventType string) Subject {
	return Subject(topic + "-" + eventType)
}

// ---- Memory registry ------------------------------------------------------

// MemoryRegistry is an in-process Registry suitable for dev and tests.
// Schemas are stored by subject + ascending version. Validation uses a
// small hand-rolled JSON Schema subset that covers the envelope rules
// the V22 outbox actually relies on: top-level object, required keys,
// and per-field type checks. Anything beyond that is no-op.
type MemoryRegistry struct {
	mu       sync.RWMutex
	versions map[Subject][]RegisteredSchema
}

// NewMemoryRegistry constructs an empty in-process registry.
func NewMemoryRegistry() *MemoryRegistry {
	return &MemoryRegistry{versions: map[Subject][]RegisteredSchema{}}
}

// Register appends a new version after a backward-compat Check. The
// new RegisteredSchema is returned with its assigned version.
func (m *MemoryRegistry) Register(ctx context.Context, subject Subject, jsonSchema string) (RegisteredSchema, error) {
	if subject == "" {
		return RegisteredSchema{}, errors.New("events: subject required")
	}
	if strings.TrimSpace(jsonSchema) == "" {
		return RegisteredSchema{}, errors.New("events: schema required")
	}
	if err := m.Check(ctx, subject, jsonSchema); err != nil {
		return RegisteredSchema{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	v := len(m.versions[subject]) + 1
	rs := RegisteredSchema{Subject: subject, Version: v, Schema: jsonSchema}
	m.versions[subject] = append(m.versions[subject], rs)
	return rs, nil
}

// Get returns the exact (subject, version) pair.
func (m *MemoryRegistry) Get(ctx context.Context, subject Subject, version int) (RegisteredSchema, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	vs := m.versions[subject]
	if version < 1 || version > len(vs) {
		return RegisteredSchema{}, ErrSchemaNotFound
	}
	return vs[version-1], nil
}

// Latest returns the most recent version, or ErrSchemaNotFound.
func (m *MemoryRegistry) Latest(ctx context.Context, subject Subject) (RegisteredSchema, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	vs := m.versions[subject]
	if len(vs) == 0 {
		return RegisteredSchema{}, ErrSchemaNotFound
	}
	return vs[len(vs)-1], nil
}

// Validate runs the payload through the latest schema. Subjects without
// a registered schema return ErrSchemaNotFound so the caller can decide
// (the outbox hook logs Warn + proceeds, per ARCHITECTURE_EVENTS.md).
func (m *MemoryRegistry) Validate(ctx context.Context, subject Subject, payload []byte) error {
	rs, err := m.Latest(ctx, subject)
	if err != nil {
		return err
	}
	return validateJSONSchemaMinimal(rs.Schema, payload)
}

// Check enforces backward compatibility for the candidate schema. The
// in-process rule is intentionally narrow: removing a required field is
// forbidden; adding optional fields is fine. CI/Karapace owns the full
// matrix.
func (m *MemoryRegistry) Check(ctx context.Context, subject Subject, jsonSchema string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	vs := m.versions[subject]
	if len(vs) == 0 {
		// First version is always compatible with "nothing".
		return nil
	}
	prev := vs[len(vs)-1].Schema
	prevReq, err := parseRequiredFields(prev)
	if err != nil {
		return fmt.Errorf("events: parse prior schema: %w", err)
	}
	nextReq, err := parseRequiredFields(jsonSchema)
	if err != nil {
		return fmt.Errorf("events: parse candidate schema: %w", err)
	}
	// Backward compatibility: every previously-required field must
	// still be required in the new version. Adding new required fields
	// would break old producers — but producers ARE the source-of-truth
	// here, so we only block field REMOVAL.
	nextSet := map[string]struct{}{}
	for _, f := range nextReq {
		nextSet[f] = struct{}{}
	}
	for _, f := range prevReq {
		if _, ok := nextSet[f]; !ok {
			return fmt.Errorf("events: backward-incompatible: required field %q removed", f)
		}
	}
	return nil
}

// ---- HTTP registry --------------------------------------------------------

// HTTPRegistry calls a Karapace/Confluent Schema Registry-compatible
// HTTP endpoint. The four operations Ironflyer needs are wired:
//
//   - POST /subjects/<subject>/versions                       (Register)
//   - GET  /subjects/<subject>/versions/<version>             (Get)
//   - GET  /subjects/<subject>/versions/latest                (Latest)
//   - POST /compatibility/subjects/<subject>/versions/latest  (Check)
//
// Validate fetches Latest from the upstream and runs the same minimal
// JSON Schema validator MemoryRegistry uses. We deliberately do NOT
// pull in the official Confluent SDK: the surface is small and the
// dependency cost is not justified.
type HTTPRegistry struct {
	baseURL string
	http    *http.Client
	auth    string // basic auth header value; empty = no auth
	log     zerolog.Logger
}

// HTTPRegistryOption tunes an HTTPRegistry constructor.
type HTTPRegistryOption func(*HTTPRegistry)

// WithHTTPClient overrides the default *http.Client. The default is a
// fresh client with a 10s timeout.
func WithHTTPClient(c *http.Client) HTTPRegistryOption {
	return func(r *HTTPRegistry) {
		if c != nil {
			r.http = c
		}
	}
}

// WithBasicAuth stamps a basic-auth header on every outbound request.
// Either argument empty disables auth.
func WithBasicAuth(user, pass string) HTTPRegistryOption {
	return func(r *HTTPRegistry) {
		if user == "" && pass == "" {
			r.auth = ""
			return
		}
		token := base64.StdEncoding.EncodeToString([]byte(user + ":" + pass))
		r.auth = "Basic " + token
	}
}

// WithLogger installs the zerolog logger used for warnings (e.g. read-back
// failures after a successful register).
func WithLogger(l zerolog.Logger) HTTPRegistryOption {
	return func(r *HTTPRegistry) { r.log = l }
}

// NewHTTPRegistry binds to a registry URL. baseURL example:
// "http://schema-registry:8081". Returns an error when baseURL is empty.
func NewHTTPRegistry(baseURL string, opts ...HTTPRegistryOption) (*HTTPRegistry, error) {
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if trimmed == "" {
		return nil, errors.New("events: schema registry base URL is required")
	}
	r := &HTTPRegistry{
		baseURL: trimmed,
		http:    &http.Client{Timeout: 10 * time.Second},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(r)
		}
	}
	return r, nil
}

// registryError mirrors the Confluent/Karapace error envelope.
type registryError struct {
	ErrorCode int    `json:"error_code"`
	Message   string `json:"message"`
}

func (e registryError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("schema registry error code %d", e.ErrorCode)
	}
	return fmt.Sprintf("schema registry %d: %s", e.ErrorCode, e.Message)
}

// do executes an HTTP request and classifies the response according to
// the Confluent error code table. Returns (body, nil) on 2xx, and a
// well-known sentinel error otherwise.
func (h *HTTPRegistry) do(ctx context.Context, method, path string, body any) ([]byte, error) {
	if h == nil || h.baseURL == "" {
		return nil, errors.New("events: http registry not configured")
	}
	var reqBody io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("events: marshal request: %w", err)
		}
		reqBody = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, h.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("events: build request: %w", err)
	}
	// Confluent uses a custom content type; Karapace accepts both. We
	// send the application/json fallback so any HTTP-Schema-Registry
	// implementation behind a reverse proxy doesn't choke.
	req.Header.Set("Accept", "application/vnd.schemaregistry.v1+json, application/json")
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/vnd.schemaregistry.v1+json")
	}
	if h.auth != "" {
		req.Header.Set("Authorization", h.auth)
	}
	resp, err := h.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("events: registry call: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return respBody, nil
	}
	// Best-effort decode of the registry error envelope.
	var rerr registryError
	_ = json.Unmarshal(respBody, &rerr)
	switch resp.StatusCode {
	case http.StatusNotFound:
		return nil, ErrSchemaNotFound
	case http.StatusConflict:
		// 409 -> schema is incompatible with prior version(s).
		return nil, fmt.Errorf("events: schema incompatible: %s", rerr.Error())
	case http.StatusUnprocessableEntity:
		// 422 -> invalid schema / parse error / unsupported type.
		return nil, fmt.Errorf("%w: %s", ErrSchemaValidation, rerr.Error())
	default:
		if rerr.ErrorCode == 0 && rerr.Message == "" {
			return nil, fmt.Errorf("events: registry http %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
		}
		return nil, fmt.Errorf("events: registry http %d: %s", resp.StatusCode, rerr.Error())
	}
}

// registerRequest is the body POSTed to /subjects/<s>/versions.
type registerRequest struct {
	SchemaType string `json:"schemaType"`
	Schema     string `json:"schema"`
}

type registerResponse struct {
	ID int `json:"id"`
}

// versionResponse is returned by GET /subjects/<s>/versions/<v>.
type versionResponse struct {
	Subject    string `json:"subject"`
	Version    int    `json:"version"`
	ID         int    `json:"id"`
	Schema     string `json:"schema"`
	SchemaType string `json:"schemaType"`
}

type compatibilityResponse struct {
	IsCompatible bool `json:"is_compatible"`
}

// Register POSTs the schema and then reads back the registered version
// (the create endpoint only returns the global schema id). On a 200
// response the assigned Version is returned. On read-back failure we
// return the schema with Version=0 plus a warning log so the caller can
// proceed without losing the registration.
func (h *HTTPRegistry) Register(ctx context.Context, subject Subject, jsonSchema string) (RegisteredSchema, error) {
	if subject == "" {
		return RegisteredSchema{}, errors.New("events: subject required")
	}
	if strings.TrimSpace(jsonSchema) == "" {
		return RegisteredSchema{}, errors.New("events: schema required")
	}
	body := registerRequest{SchemaType: "JSON", Schema: jsonSchema}
	raw, err := h.do(ctx, http.MethodPost, "/subjects/"+escapeSubject(subject)+"/versions", body)
	if err != nil {
		return RegisteredSchema{}, err
	}
	var resp registerResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return RegisteredSchema{}, fmt.Errorf("events: decode register response: %w", err)
	}
	// Read back the version metadata; Confluent's POST only returns id.
	latest, lerr := h.Latest(ctx, subject)
	if lerr != nil {
		h.log.Warn().
			Err(lerr).
			Str("subject", string(subject)).
			Int("id", resp.ID).
			Msg("events: registered schema but read-back failed")
		return RegisteredSchema{Subject: subject, Version: 0, Schema: jsonSchema}, nil
	}
	return latest, nil
}

// Get fetches a specific (subject, version).
func (h *HTTPRegistry) Get(ctx context.Context, subject Subject, version int) (RegisteredSchema, error) {
	if version < 1 {
		return RegisteredSchema{}, fmt.Errorf("events: version must be >= 1, got %d", version)
	}
	return h.fetchVersion(ctx, subject, fmt.Sprintf("%d", version))
}

// Latest fetches the most recent version of subject.
func (h *HTTPRegistry) Latest(ctx context.Context, subject Subject) (RegisteredSchema, error) {
	return h.fetchVersion(ctx, subject, "latest")
}

func (h *HTTPRegistry) fetchVersion(ctx context.Context, subject Subject, version string) (RegisteredSchema, error) {
	raw, err := h.do(ctx, http.MethodGet, "/subjects/"+escapeSubject(subject)+"/versions/"+version, nil)
	if err != nil {
		return RegisteredSchema{}, err
	}
	var resp versionResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return RegisteredSchema{}, fmt.Errorf("events: decode version response: %w", err)
	}
	return RegisteredSchema{
		Subject: subject,
		Version: resp.Version,
		Schema:  resp.Schema,
	}, nil
}

// Validate fetches the latest schema for subject and runs the local
// minimal validator (the same one MemoryRegistry uses). Subjects that
// have no registered version return ErrSchemaNotFound so outboxhooks
// can log + proceed.
func (h *HTTPRegistry) Validate(ctx context.Context, subject Subject, payload []byte) error {
	rs, err := h.Latest(ctx, subject)
	if err != nil {
		return err
	}
	return validateJSONSchemaMinimal(rs.Schema, payload)
}

// Check runs the upstream compatibility check against the latest
// version. Returns nil when compatible, an error when not.
func (h *HTTPRegistry) Check(ctx context.Context, subject Subject, jsonSchema string) error {
	if strings.TrimSpace(jsonSchema) == "" {
		return errors.New("events: schema required")
	}
	body := registerRequest{SchemaType: "JSON", Schema: jsonSchema}
	raw, err := h.do(ctx, http.MethodPost, "/compatibility/subjects/"+escapeSubject(subject)+"/versions/latest", body)
	if err != nil {
		// 404 from the compatibility endpoint means "no prior version",
		// which Confluent treats as trivially compatible.
		if errors.Is(err, ErrSchemaNotFound) {
			return nil
		}
		return err
	}
	var resp compatibilityResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Errorf("events: decode compatibility response: %w", err)
	}
	if !resp.IsCompatible {
		return fmt.Errorf("events: schema incompatible with latest version of %q", subject)
	}
	return nil
}

// escapeSubject percent-escapes the few characters Karapace cares
// about in a subject. We intentionally avoid net/url.PathEscape because
// it also escapes `.` which Karapace treats literally.
func escapeSubject(s Subject) string {
	r := strings.NewReplacer(
		" ", "%20",
		"/", "%2F",
		"?", "%3F",
		"#", "%23",
	)
	return r.Replace(string(s))
}

// ---- Minimal JSON Schema validator ---------------------------------------
//
// This is NOT a full JSON Schema implementation. It validates the
// subset every Ironflyer envelope actually uses:
//
//   - top-level "type": "object"
//   - "required": []string   (presence of each key, non-null)
//   - "properties": {field: {"type": "<json type>"}}
//
// Anything else is ignored (so optional fields and unknown keywords
// don't false-positive). This is enough to enforce the envelope
// invariants (event_id, tenant_id, occurred_at) until a real validator
// is adopted.

type minimalSchema struct {
	Type       string                              `json:"type"`
	Required   []string                            `json:"required"`
	Properties map[string]minimalPropertyConstrain `json:"properties"`
}

// minimalPropertyConstrain captures the per-property `type` clause. JSON
// Schema draft-6+ allows `type` to be a single string OR an array of
// strings (e.g. `["string","null"]` for nullable fields). We accept both
// shapes — the validator below treats any of the listed types as
// satisfactory.
type minimalPropertyConstrain struct {
	Types []string
}

func (p *minimalPropertyConstrain) UnmarshalJSON(b []byte) error {
	if len(b) == 0 {
		return nil
	}
	// Try string first (the common case).
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		if s != "" {
			p.Types = []string{s}
		}
		return nil
	}
	// Fall back to array of strings.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err == nil {
		if tRaw, ok := raw["type"]; ok {
			b = tRaw
		}
	}
	var arr []string
	if err := json.Unmarshal(b, &arr); err == nil {
		p.Types = arr
		return nil
	}
	// Object form: {"type": ...} — extract the inner type and recurse.
	var obj struct {
		Type json.RawMessage `json:"type"`
	}
	if err := json.Unmarshal(b, &obj); err == nil && len(obj.Type) > 0 {
		return p.UnmarshalJSON(obj.Type)
	}
	// Unknown shape — leave Types empty so the validator skips this field.
	return nil
}

func validateJSONSchemaMinimal(schemaJSON string, payload []byte) error {
	if strings.TrimSpace(schemaJSON) == "" {
		return nil
	}
	var s minimalSchema
	if err := json.Unmarshal([]byte(schemaJSON), &s); err != nil {
		return fmt.Errorf("%w: parse schema: %v", ErrSchemaValidation, err)
	}
	var doc map[string]any
	if err := json.Unmarshal(payload, &doc); err != nil {
		return fmt.Errorf("%w: payload not a JSON object: %v", ErrSchemaValidation, err)
	}
	if s.Type != "" && s.Type != "object" {
		// We only validate object envelopes — non-object schemas pass.
		return nil
	}
	for _, req := range s.Required {
		v, ok := doc[req]
		if !ok || v == nil {
			return fmt.Errorf("%w: missing required field %q", ErrSchemaValidation, req)
		}
	}
	for name, prop := range s.Properties {
		v, ok := doc[name]
		if !ok || v == nil {
			continue // unspecified optional field is fine
		}
		if len(prop.Types) == 0 {
			continue
		}
		// Any-of-types semantics per JSON Schema draft-6+.
		matched := false
		for _, t := range prop.Types {
			if jsonTypeMatches(t, v) {
				matched = true
				break
			}
		}
		if !matched {
			return fmt.Errorf("%w: field %q expected one of %v", ErrSchemaValidation, name, prop.Types)
		}
	}
	return nil
}

func jsonTypeMatches(want string, v any) bool {
	switch want {
	case "string":
		_, ok := v.(string)
		return ok
	case "number":
		switch v.(type) {
		case float64, float32, int, int32, int64:
			return true
		}
		return false
	case "integer":
		switch n := v.(type) {
		case int, int32, int64:
			return true
		case float64:
			return n == float64(int64(n))
		}
		return false
	case "boolean":
		_, ok := v.(bool)
		return ok
	case "object":
		_, ok := v.(map[string]any)
		return ok
	case "array":
		_, ok := v.([]any)
		return ok
	case "null":
		return v == nil
	}
	return true
}

func parseRequiredFields(schemaJSON string) ([]string, error) {
	var s minimalSchema
	if err := json.Unmarshal([]byte(schemaJSON), &s); err != nil {
		return nil, err
	}
	return s.Required, nil
}

// EnvelopeSchema is the default JSON Schema applied to outbox payloads
// when the producer hasn't registered something more specific. It
// codifies the three header-mirrored envelope keys
// ARCHITECTURE_EVENTS.md calls out: event_id, tenant_id, occurred_at.
const EnvelopeSchema = `{
  "type": "object",
  "required": ["event_id", "tenant_id", "occurred_at"],
  "properties": {
    "event_id":    {"type": "string"},
    "tenant_id":   {"type": "string"},
    "occurred_at": {"type": "string"}
  }
}`

// RegisterDefaultSubjects pre-loads a permissive envelope schema for
// every known V22 topic. This is best-effort — callers that already
// register richer per-event subjects keep theirs (Register is append-
// only and the validator picks Latest). On any Register error we
// return so the operator can see misconfiguration at boot.
//
// The 9 topics tracked by ARCHITECTURE_EVENTS.md are seeded under a
// catch-all subject "<topic>-default" so producers that haven't
// stamped a per-event subject still get baseline envelope validation.
// Producers that DO stamp <topic>-<event_type> remain unblocked
// because their subject is treated separately.
func RegisterDefaultSubjects(reg Registry) error {
	if reg == nil {
		return nil
	}
	topics := []string{
		TopicExecutionLifecycle,
		TopicExecutionSteps,
		TopicGatesResults,
		TopicPatchesLifecycle,
		TopicBillingLedger,
		TopicProfitGuardDecisions,
		TopicDeployLifecycle,
		TopicMemoryIndexing,
		TopicAuditSecurity,
	}
	ctx := context.Background()
	for _, t := range topics {
		subj := Subject(t + "-default")
		if _, err := reg.Register(ctx, subj, EnvelopeSchema); err != nil {
			return fmt.Errorf("events: register default subject %s: %w", subj, err)
		}
	}
	return nil
}

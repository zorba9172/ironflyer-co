package providers

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// MockProvider streams a deterministic response word-by-word so the streaming
// UI and Workflow can be exercised without API keys.
type MockProvider struct{ name string }

func NewMockProvider(name string) *MockProvider { return &MockProvider{name: name} }

func (m *MockProvider) Name() string { return m.name }

func (m *MockProvider) Capabilities() []Capability {
	return []Capability{CapReasoning, CapCode, CapJSON, CapCheap, CapFast, CapPrivate}
}

func (m *MockProvider) CompleteStream(ctx context.Context, req Request) (<-chan Delta, error) {
	out := make(chan Delta, 16)
	go func() {
		defer close(out)
		out <- Delta{Type: DeltaStart, Provider: m.name, Model: m.name + "-mock-1"}

		var script strings.Builder
		script.WriteString("[mock] ")
		if req.System != "" {
			script.WriteString("(system: " + summary(req.System) + ") ")
		}
		script.WriteString(req.Prompt)

		words := strings.Fields(script.String())
		for _, w := range words {
			select {
			case <-ctx.Done():
				out <- Delta{Type: DeltaError, Err: ctx.Err()}
				return
			case <-time.After(15 * time.Millisecond):
			}
			out <- Delta{Type: DeltaText, Text: w + " "}
		}
		out <- Delta{
			Type: DeltaDone, Provider: m.name, Model: m.name + "-mock-1",
			Usage: &Usage{InputTokens: len(req.Prompt) / 4, OutputTokens: len(words)},
		}
	}()
	return out, nil
}

func summary(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 60 {
		return s[:60] + "…"
	}
	return s
}

// compile-time check.
var _ Provider = (*MockProvider)(nil)
var _ = fmt.Sprintf

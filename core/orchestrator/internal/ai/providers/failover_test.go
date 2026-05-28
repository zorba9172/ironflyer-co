package providers

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type failoverTestProvider struct {
	name   string
	err    error
	deltas []Delta
}

func (p failoverTestProvider) Name() string { return p.name }

func (p failoverTestProvider) Capabilities() []Capability {
	return []Capability{CapCode, CapFast}
}

func (p failoverTestProvider) CompleteStream(context.Context, Request) (<-chan Delta, error) {
	if p.err != nil {
		return nil, p.err
	}
	deltas := p.deltas
	if len(deltas) == 0 {
		deltas = []Delta{
			{Type: DeltaText, Text: "fallback ok"},
			{Type: DeltaDone, Provider: p.name, Model: p.name + "-model"},
		}
	}
	ch := make(chan Delta, len(deltas))
	for _, d := range deltas {
		ch <- d
	}
	close(ch)
	return ch, nil
}

func TestRouterFailoverTriesNextProviderOnStartError(t *testing.T) {
	router := NewRouter()
	router.Register(failoverTestProvider{
		name: "gemini",
		err:  errors.New(`gemini http status 400: API key expired`),
	})
	router.Register(failoverTestProvider{name: "anthropic"})

	ch, err := router.CompleteStreamWithFailover(context.Background(), Request{
		Capabilities: []Capability{CapCode, CapFast},
	})
	if err != nil {
		t.Fatalf("CompleteStreamWithFailover() error = %v", err)
	}

	var text, provider string
	for d := range ch {
		switch d.Type {
		case DeltaText:
			text += d.Text
		case DeltaDone:
			provider = d.Provider
		case DeltaError:
			t.Fatalf("unexpected DeltaError: %v", d.Err)
		}
	}

	if !strings.Contains(text, "fallback ok") {
		t.Fatalf("stream text = %q, want fallback response", text)
	}
	if provider != "anthropic" {
		t.Fatalf("provider = %q, want anthropic", provider)
	}
}

func TestRouterFailoverDoesNotCommitOnStartOnly(t *testing.T) {
	router := NewRouter()
	router.Register(failoverTestProvider{
		name: "gemini",
		deltas: []Delta{
			{Type: DeltaStart, Provider: "gemini", Model: "gemini-model"},
			{Type: DeltaError, Err: errors.New(`gemini stream: API key expired`)},
		},
	})
	router.Register(failoverTestProvider{name: "anthropic"})

	ch, err := router.CompleteStreamWithFailover(context.Background(), Request{
		Capabilities: []Capability{CapCode, CapFast},
	})
	if err != nil {
		t.Fatalf("CompleteStreamWithFailover() error = %v", err)
	}

	var provider string
	for d := range ch {
		switch d.Type {
		case DeltaDone:
			provider = d.Provider
		case DeltaError:
			t.Fatalf("unexpected DeltaError: %v", d.Err)
		}
	}
	if provider != "anthropic" {
		t.Fatalf("provider = %q, want anthropic", provider)
	}
}

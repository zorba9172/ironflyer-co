package leads

import (
	"context"
	"strings"
	"testing"
)

func TestMemoryStore_CreatesIDAndTimestamp(t *testing.T) {
	s := NewMemoryStore()
	got, err := s.Create(context.Background(), Lead{Email: "a@b.com", Company: "Acme"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !strings.HasPrefix(got.ID, "lead_") {
		t.Errorf("expected lead_-prefixed ID, got %q", got.ID)
	}
	if got.CreatedAt.IsZero() {
		t.Errorf("CreatedAt was not stamped")
	}
}

func TestMemoryStore_ListNewestFirst(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()
	for _, email := range []string{"first@x", "second@x", "third@x"} {
		if _, err := s.Create(ctx, Lead{Email: email, Company: "Acme"}); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}
	got, err := s.List(ctx, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 3 || got[0].Email != "third@x" || got[2].Email != "first@x" {
		t.Errorf("unexpected order: %+v", got)
	}
}

func TestMemoryStore_LimitTrims(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		_, _ = s.Create(ctx, Lead{Email: "x@x", Company: "Acme"})
	}
	got, err := s.List(ctx, 2)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 items, got %d", len(got))
	}
}

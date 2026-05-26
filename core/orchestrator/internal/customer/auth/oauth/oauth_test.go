package oauth

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRedirectTargetPreservesCallbackQuery(t *testing.T) {
	h := &Handler{defaultPostLoginURL: "http://localhost:3000/app"}
	req := httptest.NewRequest("GET", "http://api.test/auth/github/start?redirect=%2Fauth%2Fcallback%3Fnext%3D%252Fdashboard", nil)

	got := h.redirectTarget(req)
	want := "http://localhost:3000/auth/callback?next=%2Fdashboard"
	if got != want {
		t.Fatalf("redirectTarget() = %q, want %q", got, want)
	}
}

func TestRedirectTargetRejectsAbsoluteRedirect(t *testing.T) {
	h := &Handler{defaultPostLoginURL: "http://localhost:3000/app"}
	req := httptest.NewRequest("GET", "http://api.test/auth/github/start?redirect=https%3A%2F%2Fevil.test%2Fcallback", nil)

	got := h.redirectTarget(req)
	want := "http://localhost:3000/app"
	if got != want {
		t.Fatalf("redirectTarget() = %q, want %q", got, want)
	}
}

func TestBuildRedirectKeepsCallbackQueryBeforeFragment(t *testing.T) {
	got := buildRedirect("http://localhost:3000/auth/callback?next=%2Fdashboard", "jwt.token", time.Unix(0, 0).UTC())

	if !strings.HasPrefix(got, "http://localhost:3000/auth/callback?next=%2Fdashboard#") {
		t.Fatalf("buildRedirect() = %q, want callback query before fragment", got)
	}
	if !strings.Contains(got, "token=jwt.token") {
		t.Fatalf("buildRedirect() = %q, want token fragment", got)
	}
}

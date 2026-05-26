// {{PROJECT_NAME}} — Ironflyer scaffold {{TODAY}}
//
// Minimal chi server with /health and / endpoints. The Ironflyer Coder
// agent will iterate on this file (and add packages) as gates progress.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(15 * time.Second))

	r.Get("/", index)
	r.Get("/health", health)

	addr := ":" + port
	log.Printf("{{PROJECT_NAME}} listening on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("server: %v", err)
	}
}

func index(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!doctype html>
<html><head><meta charset="utf-8"><title>{{PROJECT_NAME}}</title>
<style>body{margin:0;min-height:100vh;background:#0b0b0c;color:#f5f5f4;font-family:Inter,system-ui,sans-serif;display:grid;place-items:center;padding:24px}
.card{max-width:560px;width:100%%;background:#161617;border-radius:12px;padding:28px}
.chip{display:inline-block;padding:4px 10px;border-radius:999px;background:#c6ff3a;color:#0b0b0c;font-weight:600;font-size:12px}
h1{margin:12px 0 8px;font-size:28px}p{opacity:.8;line-height:1.5}
code{background:#0b0b0c;padding:2px 6px;border-radius:4px}</style></head>
<body><div class="card">
<span class="chip">Ironflyer scaffold {{TODAY}}</span>
<h1>Welcome to {{PROJECT_NAME}}</h1>
<p>This Go + chi server is live. Try <code>GET /health</code> — the Ironflyer
finisher will keep editing this app through enforced gates.</p>
</div></body></html>`)
}

func health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":  "ok",
		"service": "{{PROJECT_NAME}}",
		"at":      "{{TODAY}}",
	})
}

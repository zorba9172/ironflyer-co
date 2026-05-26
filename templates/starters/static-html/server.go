// {{PROJECT_NAME}} static server — Ironflyer scaffold {{TODAY}}
//
// Tiny stdlib server that serves the current directory as static files.
// Kept dependency-free so the runtime can `go run ./server.go` without
// fetching modules.
package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	dir, err := os.Getwd()
	if err != nil {
		log.Fatalf("getwd: %v", err)
	}
	log.Printf("{{PROJECT_NAME}} serving %s on :%s", dir, port)
	if err := http.ListenAndServe(":"+port, http.FileServer(http.Dir(dir))); err != nil {
		log.Fatalf("server: %v", err)
	}
}

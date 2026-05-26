//go:build tools
// +build tools

// Package gql pins the genqlient binary as a tool dependency so the
// codegen step (`go run github.com/Khan/genqlient`) finds its full
// transitive module set without requiring `go get` after every
// `go mod tidy`.
//
// Build tag `tools` keeps these imports out of the runtime binary —
// nothing here is reachable from a normal build.
package gql

import (
	_ "github.com/Khan/genqlient"
	_ "github.com/Khan/genqlient/generate"
)

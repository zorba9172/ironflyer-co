// Command ironflyer is the Ironflyer CLI entry point. The binary is a
// thin wrapper around commands.Execute — every interesting line lives
// under apps/cli/internal/.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"ironflyer/apps/cli/internal/commands"
)

func main() {
	// Honour SIGINT/SIGTERM at the outermost level so long-running
	// streaming commands can clean up. Individual commands also install
	// their own handlers because some of them (eg `run`) treat Ctrl+C
	// as "detach, don't cancel server-side work".
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	code := commands.Execute(ctx, os.Args[1:])
	os.Exit(code)
}

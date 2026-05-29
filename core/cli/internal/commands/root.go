// Package commands wires every CLI subcommand. The framework is hand-
// rolled rather than imported (cobra, urfave/cli) to keep deps zero —
// the dispatch surface is small enough that stdlib + a tiny Command
// struct is clearer than a 3rd-party DSL.
package commands

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"ironflyer/core/cli/internal/client"
	"ironflyer/core/cli/internal/config"
	"ironflyer/core/cli/internal/ui"
)

// Version is the CLI's reported version. The Makefile -ldflags override
// this at release time; the default matches version.txt.
var Version = "v0.1.0"

// Command is one node in the dispatch tree. A command can be a leaf
// (Run != nil) or a parent (Subs non-empty). Each command registers its
// own flag.FlagSet on demand so `--help` knows the exact flags.
type Command struct {
	Name     string
	Short    string
	Long     string
	Usage    string
	Examples []string
	Aliases  []string
	Subs     []*Command
	RegFlags func(fs *flag.FlagSet)
	Run      func(ctx context.Context, env *Env, fs *flag.FlagSet, args []string) error
}

// Env carries the resolved global flags + a constructed API client. Every
// Run function receives one — there is no global state.
type Env struct {
	Host   string
	Token  string
	JSON   bool
	Config config.File
	Client *client.Client
	Out    io.Writer
	Err    io.Writer
}

// Root builds the top-level Command. main() calls Execute on this.
func Root() *Command {
	root := &Command{
		Name:  "ironflyer",
		Short: "Ironflyer CLI — the AI Product Finisher from your terminal",
		Long: "ironflyer is a power-user CLI for the Ironflyer orchestrator.\n" +
			"It mirrors the dashboard: list/create projects, run the\n" +
			"finisher gates, stream events, apply patches, deploy, export.",
		Examples: []string{
			"ironflyer login",
			"ironflyer projects",
			"ironflyer projects create --name todo --prompt 'a todo app'",
			"ironflyer run my-project",
			"ironflyer deploy my-project --provider fly --region iad",
		},
	}
	root.Subs = []*Command{
		loginCmd(),
		logoutCmd(),
		whoamiCmd(),
		projectsCmd(),
		runCmd(),
		logsCmd(),
		patchesCmd(),
		deployCmd(),
		exportCmd(),
		statusCmd(),
		configCmd(),
		memoryCmd(),
		auditCmd(),
		telemetryCmd(),
		graphCmd(),
		versionCmd(),
	}
	return root
}

// Execute parses argv and dispatches. Returns an exit code.
func Execute(ctx context.Context, argv []string) int {
	root := Root()
	if len(argv) == 0 {
		root.PrintHelp(os.Stdout)
		return 0
	}
	// Global flags are parsed against the root flag set first. Any flag
	// before a known subcommand name is global; after the subcommand
	// name, args belong to that subcommand.
	host, token, jsonOut, rest, err := splitGlobals(argv)
	if err != nil {
		ui.Errorf("%v", err)
		return 1
	}
	cfg, err := config.Load()
	if err != nil {
		ui.Errorf("load config: %v", err)
		return 1
	}
	if host == "" {
		host = cfg.HostOrDefault()
	}
	if token == "" {
		token = cfg.Token
	}
	env := &Env{
		Host:   host,
		Token:  token,
		JSON:   jsonOut,
		Config: cfg,
		Client: client.New(host, token),
		Out:    os.Stdout,
		Err:    os.Stderr,
	}
	return root.dispatch(ctx, env, rest)
}

// splitGlobals walks argv extracting --host, --json, --token until it
// hits the first non-flag token (the subcommand). This lets us support
// either `ironflyer --json projects` or `ironflyer projects --json` —
// subcommand flag sets accept the same flag names.
func splitGlobals(argv []string) (host, token string, jsonOut bool, rest []string, err error) {
	i := 0
	for i < len(argv) {
		a := argv[i]
		switch {
		case a == "--":
			return host, token, jsonOut, argv[i+1:], nil
		case a == "--host":
			if i+1 >= len(argv) {
				return "", "", false, nil, fmt.Errorf("--host requires a value")
			}
			host = argv[i+1]
			i += 2
		case strings.HasPrefix(a, "--host="):
			host = strings.TrimPrefix(a, "--host=")
			i++
		case a == "--token":
			if i+1 >= len(argv) {
				return "", "", false, nil, fmt.Errorf("--token requires a value")
			}
			token = argv[i+1]
			i += 2
		case strings.HasPrefix(a, "--token="):
			token = strings.TrimPrefix(a, "--token=")
			i++
		case a == "--json":
			jsonOut = true
			i++
		case a == "-h", a == "--help":
			rest = append(rest, a)
			i++
		default:
			// First non-global token: the subcommand name. Everything
			// after belongs to the subcommand.
			rest = append(rest, argv[i:]...)
			return host, token, jsonOut, rest, nil
		}
	}
	return host, token, jsonOut, rest, nil
}

// dispatch finds the matching subcommand, parses its flags, and runs it.
func (c *Command) dispatch(ctx context.Context, env *Env, args []string) int {
	// If we're a parent (Run nil, Subs non-empty) and got no args, print
	// help. Leaf commands still need to run with empty arg lists.
	if len(args) == 0 {
		if c.Run != nil {
			return c.runLeaf(ctx, env, args)
		}
		c.PrintHelp(env.Out)
		return 0
	}
	first := args[0]
	if first == "-h" || first == "--help" {
		c.PrintHelp(env.Out)
		return 0
	}
	// Try to resolve a subcommand by name or alias.
	for _, sub := range c.Subs {
		if matchesName(sub, first) {
			return sub.dispatch(ctx, env, args[1:])
		}
	}
	// Leaf command — parse its flags then run.
	if c.Run == nil {
		ui.Errorf("unknown subcommand: %s", first)
		c.PrintHelp(env.Err)
		return 1
	}
	return c.runLeaf(ctx, env, args)
}

// runLeaf parses flags and executes c.Run. Pulled out so a parent command
// that also has a Run (like `projects` defaulting to `list`) can run
// with zero args.
func (c *Command) runLeaf(ctx context.Context, env *Env, args []string) int {
	fs := flag.NewFlagSet(c.Name, flag.ContinueOnError)
	fs.SetOutput(io.Discard) // we render usage ourselves
	// Always allow the global flags on the subcommand as well so users
	// can put them anywhere.
	var localHost, localToken string
	var localJSON bool
	if c.RegFlags != nil {
		c.RegFlags(fs)
	}
	if fs.Lookup("host") == nil {
		fs.StringVar(&localHost, "host", "", "orchestrator host (overrides config)")
	}
	if fs.Lookup("token") == nil {
		fs.StringVar(&localToken, "token", "", "bearer token (overrides config)")
	}
	if fs.Lookup("json") == nil {
		fs.BoolVar(&localJSON, "json", false, "emit machine-readable JSON")
	}
	if err := fs.Parse(args); err != nil {
		ui.Errorf("%v", err)
		c.PrintHelp(env.Err)
		return 1
	}
	// Merge per-command overrides into env.
	if localHost != "" {
		env.Host = localHost
		env.Client = client.New(localHost, env.Token)
	}
	if localToken != "" {
		env.Token = localToken
		env.Client = client.New(env.Host, localToken)
	}
	if localJSON {
		env.JSON = true
	}
	if err := c.Run(ctx, env, fs, fs.Args()); err != nil {
		ui.Errorf("%v", err)
		return 1
	}
	return 0
}

func matchesName(c *Command, name string) bool {
	if c.Name == name {
		return true
	}
	for _, a := range c.Aliases {
		if a == name {
			return true
		}
	}
	return false
}

// PrintHelp renders a uniform help screen.
func (c *Command) PrintHelp(w io.Writer) {
	fmt.Fprintln(w, ui.Bold(c.Name)+" — "+c.Short)
	if c.Long != "" {
		fmt.Fprintln(w)
		fmt.Fprintln(w, c.Long)
	}
	if c.Usage != "" {
		fmt.Fprintln(w)
		fmt.Fprintln(w, ui.Bold("USAGE"))
		fmt.Fprintln(w, "  "+c.Usage)
	}
	if len(c.Subs) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, ui.Bold("COMMANDS"))
		// Stable order by name for the help output.
		subs := append([]*Command(nil), c.Subs...)
		sort.Slice(subs, func(i, j int) bool { return subs[i].Name < subs[j].Name })
		for _, s := range subs {
			fmt.Fprintf(w, "  %-12s  %s\n", s.Name, s.Short)
		}
	}
	if c.RegFlags != nil {
		fs := flag.NewFlagSet(c.Name, flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		c.RegFlags(fs)
		var lines []string
		fs.VisitAll(func(f *flag.Flag) {
			lines = append(lines, fmt.Sprintf("  --%-14s %s", f.Name, f.Usage))
		})
		if len(lines) > 0 {
			fmt.Fprintln(w)
			fmt.Fprintln(w, ui.Bold("FLAGS"))
			for _, l := range lines {
				fmt.Fprintln(w, l)
			}
		}
	}
	if c.Subs == nil {
		fmt.Fprintln(w)
		fmt.Fprintln(w, ui.Bold("GLOBAL FLAGS"))
		fmt.Fprintln(w, "  --host VALUE      orchestrator host (default $IRONFLYER_HOST or http://localhost:8080)")
		fmt.Fprintln(w, "  --token VALUE     bearer token (default from ~/.ironflyer/config.json)")
		fmt.Fprintln(w, "  --json            emit machine-readable JSON")
	}
	if len(c.Examples) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, ui.Bold("EXAMPLES"))
		for _, e := range c.Examples {
			fmt.Fprintln(w, "  "+e)
		}
	}
}

// requireAuth aborts the command if no bearer token is configured.
func requireAuth(env *Env) error {
	if env.Token == "" {
		return fmt.Errorf("not logged in. run `ironflyer login` first")
	}
	return nil
}

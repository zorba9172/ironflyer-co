package commands

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"sort"
	"strings"

	"ironflyer/apps/cli/internal/client"
	"ironflyer/apps/cli/internal/ui"
)

// graphCmd prints the derived dependency graph for a project. Default
// output is a 3-section summary; --dot emits Graphviz DOT so the
// terminal user can pipe to `dot -Tpng > graph.png`.
func graphCmd() *Command {
	var dot bool
	return &Command{
		Name:  "graph",
		Short: "Show the derived dependency graph for a project",
		Usage: "ironflyer graph <project-id> [--dot]",
		Examples: []string{
			"ironflyer graph my-app",
			"ironflyer graph my-app --dot | dot -Tpng > graph.png",
		},
		RegFlags: func(fs *flag.FlagSet) {
			fs.BoolVar(&dot, "dot", false, "emit Graphviz DOT to stdout")
		},
		Run: func(ctx context.Context, env *Env, fs *flag.FlagSet, args []string) error {
			if err := requireAuth(env); err != nil {
				return err
			}
			id, err := requireProjectID(env, args)
			if err != nil {
				return err
			}
			g, err := env.Client.GetProjectGraph(ctx, id)
			if err != nil {
				return err
			}
			if env.JSON {
				b, _ := json.MarshalIndent(g, "", "  ")
				fmt.Fprintln(env.Out, string(b))
				return nil
			}
			if dot {
				renderDot(env, id, g)
				return nil
			}
			renderGraphSummary(env, g)
			return nil
		},
	}
}

// renderGraphSummary prints the three-section human view: totals,
// language histogram, and a small sample of edges so users can sanity
// check that their imports are resolving the way they expect.
func renderGraphSummary(env *Env, g *client.ProjectGraph) {
	fmt.Fprintln(env.Out, ui.Bold("NODES"))
	fmt.Fprintf(env.Out, "  total: %d\n", len(g.Nodes))

	fmt.Fprintln(env.Out)
	fmt.Fprintln(env.Out, ui.Bold("EDGES"))
	fmt.Fprintf(env.Out, "  total: %d\n", len(g.Edges))

	fmt.Fprintln(env.Out)
	fmt.Fprintln(env.Out, ui.Bold("LANGUAGES"))
	hist := map[string]int{}
	for _, n := range g.Nodes {
		lang := n.Language
		if lang == "" {
			lang = "unknown"
		}
		hist[lang]++
	}
	keys := make([]string, 0, len(hist))
	for k := range hist {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if hist[keys[i]] != hist[keys[j]] {
			return hist[keys[i]] > hist[keys[j]]
		}
		return keys[i] < keys[j]
	})
	if len(keys) == 0 {
		fmt.Fprintln(env.Out, ui.Dim("  (none)"))
		return
	}
	rows := make([][]string, 0, len(keys))
	for _, k := range keys {
		rows = append(rows, []string{k, fmt.Sprintf("%d", hist[k])})
	}
	ui.RenderTable(env.Out, []string{"Language", "Files"}, rows)
}

// renderDot writes Graphviz DOT to env.Out. Paths are quoted as-is.
// We use directed edges (digraph) since imports have a direction.
func renderDot(env *Env, projectID string, g *client.ProjectGraph) {
	fmt.Fprintf(env.Out, "digraph %q {\n", projectID)
	fmt.Fprintln(env.Out, "  rankdir=LR;")
	fmt.Fprintln(env.Out, "  node [shape=box, fontname=\"Helvetica\"];")
	for _, n := range g.Nodes {
		label := n.Path
		if n.Language != "" {
			label = fmt.Sprintf("%s\\n(%s)", n.Path, n.Language)
		}
		fmt.Fprintf(env.Out, "  %q [label=%q];\n", n.Path, label)
	}
	for _, e := range g.Edges {
		fmt.Fprintf(env.Out, "  %q -> %q", e.From, e.To)
		if e.Raw != "" && e.Raw != e.To {
			fmt.Fprintf(env.Out, " [label=%q]", trimRaw(e.Raw))
		}
		fmt.Fprintln(env.Out, ";")
	}
	fmt.Fprintln(env.Out, "}")
}

// trimRaw shortens a long raw import literal for DOT edge labels so the
// rendered graph stays legible.
func trimRaw(s string) string {
	const max = 40
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

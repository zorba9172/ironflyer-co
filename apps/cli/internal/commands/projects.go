package commands

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"ironflyer/apps/cli/internal/client"
	"ironflyer/apps/cli/internal/ui"
)

// projectsCmd is the umbrella for project CRUD. Subcommands: list (default),
// create, show, delete. Alias `ls` mirrors a common UX expectation.
func projectsCmd() *Command {
	return &Command{
		Name:    "projects",
		Aliases: []string{"ls", "project"},
		Short:   "List, create, inspect, or delete projects",
		Long:    "Without a subcommand, lists the caller's projects.",
		Usage:   "ironflyer projects [list|create|show|delete] [flags]",
		Subs: []*Command{
			projectsListCmd(),
			projectsCreateCmd(),
			projectsShowCmd(),
			projectsDeleteCmd(),
		},
		// Calling `ironflyer projects` directly behaves as `list`.
		Run: projectsListCmd().Run,
		RegFlags: func(fs *flag.FlagSet) {
			fs.Bool("all", false, "include archived projects (forwarded to list)")
		},
	}
}

func projectsListCmd() *Command {
	return &Command{
		Name:  "list",
		Short: "List projects you have access to",
		Usage: "ironflyer projects list [--json]",
		Examples: []string{
			"ironflyer projects",
			"ironflyer projects --json | jq '.[].id'",
		},
		Run: func(ctx context.Context, env *Env, fs *flag.FlagSet, args []string) error {
			if err := requireAuth(env); err != nil {
				return err
			}
			ps, err := env.Client.ListProjects(ctx)
			if err != nil {
				return err
			}
			if env.JSON {
				b, _ := json.MarshalIndent(ps, "", "  ")
				fmt.Fprintln(env.Out, string(b))
				return nil
			}
			if len(ps) == 0 {
				fmt.Fprintln(env.Out, ui.Dim("no projects yet — create one with `ironflyer projects create`"))
				return nil
			}
			rows := make([][]string, 0, len(ps))
			for _, p := range ps {
				rows = append(rows, []string{
					p.ID,
					trunc(p.Name, 32),
					colorStatus(p.Status),
					p.UpdatedAt,
				})
			}
			ui.RenderTable(env.Out, []string{"ID", "Name", "Status", "Updated"}, rows)
			return nil
		},
	}
}

func projectsCreateCmd() *Command {
	var name, idea, description string
	return &Command{
		Name:  "create",
		Short: "Create a new project with an initial idea",
		Usage: "ironflyer projects create --name NAME --prompt IDEA",
		Examples: []string{
			"ironflyer projects create --name todo-app --prompt 'a todo list with auth'",
		},
		RegFlags: func(fs *flag.FlagSet) {
			fs.StringVar(&name, "name", "", "human-readable project name (required)")
			fs.StringVar(&idea, "prompt", "", "the initial product idea")
			// Alias --idea so users who think in spec terms can use it too.
			fs.StringVar(&idea, "idea", "", "alias for --prompt")
			fs.StringVar(&description, "description", "", "optional one-line description")
		},
		Run: func(ctx context.Context, env *Env, fs *flag.FlagSet, args []string) error {
			if err := requireAuth(env); err != nil {
				return err
			}
			if name == "" {
				return fmt.Errorf("--name is required")
			}
			if idea == "" {
				return fmt.Errorf("--prompt (or --idea) is required")
			}
			p, err := env.Client.CreateProject(ctx, name, idea, description)
			if err != nil {
				return err
			}
			if env.JSON {
				b, _ := json.MarshalIndent(p, "", "  ")
				fmt.Fprintln(env.Out, string(b))
				return nil
			}
			fmt.Fprintln(env.Out, ui.Green("created project ")+ui.Bold(p.ID))
			fmt.Fprintln(env.Out, ui.Dim("→ ironflyer run "+p.ID))
			return nil
		},
	}
}

func projectsShowCmd() *Command {
	return &Command{
		Name:  "show",
		Short: "Print a detailed view of one project",
		Usage: "ironflyer projects show <id>",
		Run: func(ctx context.Context, env *Env, fs *flag.FlagSet, args []string) error {
			if err := requireAuth(env); err != nil {
				return err
			}
			id, err := requireProjectID(env, args)
			if err != nil {
				return err
			}
			p, err := env.Client.GetProject(ctx, id)
			if err != nil {
				return err
			}
			if env.JSON {
				b, _ := json.MarshalIndent(p, "", "  ")
				fmt.Fprintln(env.Out, string(b))
				return nil
			}
			renderProjectDetail(env, p)
			// Recent patches: best-effort, swallow auth errors.
			if patches, err := env.Client.ListPatches(ctx, id); err == nil && len(patches) > 0 {
				fmt.Fprintln(env.Out)
				fmt.Fprintln(env.Out, ui.Bold("RECENT PATCHES"))
				rows := make([][]string, 0, len(patches))
				cap := 8
				for i, pt := range patches {
					if i >= cap {
						break
					}
					rows = append(rows, []string{pt.ID, trunc(pt.Title, 40), pt.Status, pt.CreatedAt})
				}
				ui.RenderTable(env.Out, []string{"ID", "Title", "Status", "Created"}, rows)
			}
			return nil
		},
	}
}

func projectsDeleteCmd() *Command {
	var yes bool
	return &Command{
		Name:  "delete",
		Short: "Delete a project (irreversible)",
		Usage: "ironflyer projects delete <id> [--yes]",
		RegFlags: func(fs *flag.FlagSet) {
			fs.BoolVar(&yes, "yes", false, "skip the confirmation prompt")
		},
		Run: func(ctx context.Context, env *Env, fs *flag.FlagSet, args []string) error {
			if err := requireAuth(env); err != nil {
				return err
			}
			id, err := requireProjectID(env, args)
			if err != nil {
				return err
			}
			if !yes {
				fmt.Fprintf(env.Out, "delete project %s? [y/N] ", ui.Bold(id))
				if !confirm() {
					fmt.Fprintln(env.Out, ui.Dim("aborted"))
					return nil
				}
			}
			// We use /projects/bulk-delete with a single-id payload because
			// the orchestrator does not currently expose DELETE /projects/{id}.
			// See orchestrator internal/httpapi/api.go — there is only a
			// bulk endpoint at internal/httpapi/dashboard_handlers.go:18.
			if err := env.Client.BulkDeleteProjects(ctx, []string{id}); err != nil {
				return err
			}
			fmt.Fprintln(env.Out, ui.Green("deleted ")+id)
			return nil
		},
	}
}

func renderProjectDetail(env *Env, p *client.Project) {
	fmt.Fprintf(env.Out, "%s %s\n", ui.Bold("id:         "), p.ID)
	fmt.Fprintf(env.Out, "%s %s\n", ui.Bold("name:       "), p.Name)
	if p.Description != "" {
		fmt.Fprintf(env.Out, "%s %s\n", ui.Bold("description:"), p.Description)
	}
	fmt.Fprintf(env.Out, "%s %s\n", ui.Bold("status:     "), colorStatus(p.Status))
	if p.Spec.Idea != "" {
		fmt.Fprintf(env.Out, "%s %s\n", ui.Bold("idea:       "), trunc(p.Spec.Idea, 200))
	}
	if len(p.Gates) > 0 {
		fmt.Fprintln(env.Out)
		fmt.Fprintln(env.Out, ui.Bold("GATES"))
		rows := make([][]string, 0, len(p.Gates))
		// Iterate in a deterministic order — we know the canonical gate
		// order but the wire format is a map, so sort by name.
		names := make([]string, 0, len(p.Gates))
		for n := range p.Gates {
			names = append(names, n)
		}
		// simple sort
		for i := 1; i < len(names); i++ {
			for j := i; j > 0 && names[j] < names[j-1]; j-- {
				names[j], names[j-1] = names[j-1], names[j]
			}
		}
		for _, n := range names {
			g := p.Gates[n]
			rows = append(rows, []string{g.Name, colorStatus(g.Status), trunc(g.Detail, 60)})
		}
		ui.RenderTable(env.Out, []string{"Gate", "Status", "Detail"}, rows)
	}
}

// requireProjectID returns the first positional arg or the configured
// default project. Yields a friendly error if neither is set.
func requireProjectID(env *Env, args []string) (string, error) {
	if len(args) > 0 && args[0] != "" {
		return args[0], nil
	}
	if env.Config.DefaultProject != "" {
		return env.Config.DefaultProject, nil
	}
	return "", fmt.Errorf("project id required (positional arg or `ironflyer config set project <id>`)")
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
}

func colorStatus(s string) string {
	switch strings.ToLower(s) {
	case "passed", "pass", "ready", "ok", "live", "deployed":
		return ui.Green(s)
	case "failed", "fail", "error":
		return ui.Red(s)
	case "running", "pending", "queued":
		return ui.Yellow(s)
	case "draft":
		return ui.Dim(s)
	default:
		return s
	}
}

// confirm reads stdin for a y/N answer. Default no.
func confirm() bool {
	r := bufio.NewReader(os.Stdin)
	line, _ := r.ReadString('\n')
	line = strings.ToLower(strings.TrimSpace(line))
	return line == "y" || line == "yes"
}

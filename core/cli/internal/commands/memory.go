package commands

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"strconv"
	"strings"

	"ironflyer/core/cli/internal/client"
	"ironflyer/core/cli/internal/ui"
)

// memoryCmd is the umbrella for memory store operations. The orchestrator
// exposes /memory as the persistence layer for project / execution /
// user / business memories — this command lets power users curate it
// from the terminal without touching the dashboard.
func memoryCmd() *Command {
	return &Command{
		Name:  "memory",
		Short: "List, add, or delete memory records",
		Long: "ironflyer memory talks to the orchestrator's persistent\n" +
			"memory store. Records are scoped by kind:\n" +
			"  project   — facts pinned to one project\n" +
			"  execution — gate/run learnings\n" +
			"  user      — the calling user's notes (auto-pinned)\n" +
			"  business  — KPI / pricing / market signals",
		Usage: "ironflyer memory [list|add|delete] [flags]",
		Subs: []*Command{
			memoryListCmd(),
			memoryAddCmd(),
			memoryDeleteCmd(),
		},
		Run: memoryListCmd().Run,
	}
}

func memoryListCmd() *Command {
	var kind, project, tag, q string
	var limit int
	return &Command{
		Name:  "list",
		Short: "List memory records matching the filters",
		Usage: "ironflyer memory list [--kind=K] [--project=ID] [--tag=T] [--q=SUB] [--limit=N]",
		Examples: []string{
			"ironflyer memory list --kind=project --project=my-app",
			"ironflyer memory list --tag=decision --limit=50",
		},
		RegFlags: func(fs *flag.FlagSet) {
			fs.StringVar(&kind, "kind", "", "memory kind (project|execution|user|business)")
			fs.StringVar(&project, "project", "", "project id scope")
			fs.StringVar(&tag, "tag", "", "single-tag filter")
			fs.StringVar(&q, "q", "", "substring search across title+body")
			fs.IntVar(&limit, "limit", 0, "max rows (server default 20, cap 200)")
		},
		Run: func(ctx context.Context, env *Env, fs *flag.FlagSet, args []string) error {
			if err := requireAuth(env); err != nil {
				return err
			}
			params := map[string]string{
				"kind":      kind,
				"projectId": project,
				"tag":       tag,
				"q":         q,
			}
			if limit > 0 {
				params["limit"] = strconv.Itoa(limit)
			}
			resp, err := env.Client.ListMemory(ctx, params)
			if err != nil {
				return err
			}
			if env.JSON {
				b, _ := json.MarshalIndent(resp, "", "  ")
				fmt.Fprintln(env.Out, string(b))
				return nil
			}
			if len(resp.Records) == 0 {
				fmt.Fprintln(env.Out, ui.Dim("no memory records match the filter"))
				return nil
			}
			// Two-line render per row: title (with id + kind prefix) then
			// dim tags + scope on the second line. This keeps records
			// readable when bodies are long without truncating to noise.
			for i, r := range resp.Records {
				if i > 0 {
					fmt.Fprintln(env.Out)
				}
				header := fmt.Sprintf("%s  %s  %s",
					ui.Bold(r.ID),
					ui.Cyan(r.Kind),
					r.Title,
				)
				fmt.Fprintln(env.Out, header)
				meta := []string{}
				if r.ProjectID != "" {
					meta = append(meta, "project="+r.ProjectID)
				}
				if r.UserID != "" {
					meta = append(meta, "user="+r.UserID)
				}
				if r.GateName != "" {
					meta = append(meta, "gate="+r.GateName)
				}
				if len(r.Tags) > 0 {
					meta = append(meta, "tags=["+strings.Join(r.Tags, ",")+"]")
				}
				if r.CreatedAt != "" {
					meta = append(meta, r.CreatedAt)
				}
				fmt.Fprintln(env.Out, "  "+ui.Dim(strings.Join(meta, " · ")))
			}
			fmt.Fprintln(env.Out)
			fmt.Fprintln(env.Out, ui.Dim(fmt.Sprintf("%d record(s)", resp.Count)))
			return nil
		},
	}
}

func memoryAddCmd() *Command {
	var kind, project, title, body, tagsRaw string
	return &Command{
		Name:  "add",
		Short: "Persist a new memory record",
		Usage: "ironflyer memory add --kind=K --project=ID --title=T --body=B [--tags=a,b]",
		Examples: []string{
			"ironflyer memory add --kind=project --project=my-app --title='auth choice' --body='picked JWT over sessions' --tags=decision,auth",
		},
		RegFlags: func(fs *flag.FlagSet) {
			fs.StringVar(&kind, "kind", "", "memory kind (project|execution|user|business) — required")
			fs.StringVar(&project, "project", "", "project id (required for project/execution/business)")
			fs.StringVar(&title, "title", "", "short headline — required")
			fs.StringVar(&body, "body", "", "longer body / detail")
			fs.StringVar(&tagsRaw, "tags", "", "comma-separated tags")
		},
		Run: func(ctx context.Context, env *Env, fs *flag.FlagSet, args []string) error {
			if err := requireAuth(env); err != nil {
				return err
			}
			if kind == "" {
				return fmt.Errorf("--kind is required")
			}
			if title == "" {
				return fmt.Errorf("--title is required")
			}
			rec := client.MemoryRecord{
				Kind:      strings.ToLower(kind),
				ProjectID: project,
				Title:     title,
				Body:      body,
			}
			if tagsRaw != "" {
				for _, t := range strings.Split(tagsRaw, ",") {
					if t = strings.TrimSpace(t); t != "" {
						rec.Tags = append(rec.Tags, t)
					}
				}
			}
			stored, err := env.Client.AddMemory(ctx, rec)
			if err != nil {
				return err
			}
			if env.JSON {
				b, _ := json.MarshalIndent(stored, "", "  ")
				fmt.Fprintln(env.Out, string(b))
				return nil
			}
			fmt.Fprintln(env.Out, ui.Green("recorded ")+ui.Bold(stored.ID))
			return nil
		},
	}
}

func memoryDeleteCmd() *Command {
	return &Command{
		Name:  "delete",
		Short: "Delete a memory record by id (idempotent)",
		Usage: "ironflyer memory delete <id>",
		Run: func(ctx context.Context, env *Env, fs *flag.FlagSet, args []string) error {
			if err := requireAuth(env); err != nil {
				return err
			}
			if len(args) == 0 || args[0] == "" {
				return fmt.Errorf("memory id required")
			}
			if err := env.Client.DeleteMemory(ctx, args[0]); err != nil {
				return err
			}
			fmt.Fprintln(env.Out, "ok")
			return nil
		},
	}
}

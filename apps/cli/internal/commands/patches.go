package commands

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"

	"ironflyer/apps/cli/internal/ui"
)

// patchesCmd lists patches for a project and, with --apply, applies a
// specific patch by id.
func patchesCmd() *Command {
	var applyID string
	return &Command{
		Name:  "patches",
		Short: "List or apply patches for a project",
		Usage: "ironflyer patches <id> [--apply PATCH_ID]",
		Examples: []string{
			"ironflyer patches my-project",
			"ironflyer patches my-project --apply patch_abc123",
		},
		RegFlags: func(fs *flag.FlagSet) {
			fs.StringVar(&applyID, "apply", "", "apply this patch id and exit")
		},
		Run: func(ctx context.Context, env *Env, fs *flag.FlagSet, args []string) error {
			if err := requireAuth(env); err != nil {
				return err
			}
			id, err := requireProjectID(env, args)
			if err != nil {
				return err
			}
			if applyID != "" {
				if err := env.Client.ApplyPatch(ctx, applyID); err != nil {
					return err
				}
				fmt.Fprintln(env.Out, ui.Green("applied ")+applyID)
				return nil
			}
			ps, err := env.Client.ListPatches(ctx, id)
			if err != nil {
				return err
			}
			if env.JSON {
				b, _ := json.MarshalIndent(ps, "", "  ")
				fmt.Fprintln(env.Out, string(b))
				return nil
			}
			if len(ps) == 0 {
				fmt.Fprintln(env.Out, ui.Dim("no patches for "+id))
				return nil
			}
			rows := make([][]string, 0, len(ps))
			for _, p := range ps {
				rows = append(rows, []string{
					p.ID,
					trunc(p.Title, 40),
					colorStatus(p.Status),
					p.Author,
					p.CreatedAt,
				})
			}
			ui.RenderTable(env.Out, []string{"ID", "Title", "Status", "Author", "Created"}, rows)
			return nil
		},
	}
}

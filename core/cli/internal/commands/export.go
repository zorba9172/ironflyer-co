package commands

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"ironflyer/core/cli/internal/ui"
)

// exportCmd implements `ironflyer export <id> --format zip|github`. Zip
// downloads the archive to ./<id>.zip; github creates a repo via the
// orchestrator's /export/github endpoint and prints the URL.
func exportCmd() *Command {
	var format, repoName, description, outPath string
	var private bool
	return &Command{
		Name:  "export",
		Short: "Export a project as a zip or push it to a new GitHub repo",
		Usage: "ironflyer export <id> --format zip|github [--repo-name NAME] [--out PATH]",
		Examples: []string{
			"ironflyer export my-project --format zip",
			"ironflyer export my-project --format github --repo-name my-app --private",
		},
		RegFlags: func(fs *flag.FlagSet) {
			fs.StringVar(&format, "format", "zip", "zip | github")
			fs.StringVar(&repoName, "repo-name", "", "(github) name for the new repo")
			fs.StringVar(&description, "description", "", "(github) repo description")
			fs.BoolVar(&private, "private", false, "(github) create as private")
			fs.StringVar(&outPath, "out", "", "(zip) output path (default ./<id>.zip)")
		},
		Run: func(ctx context.Context, env *Env, fs *flag.FlagSet, args []string) error {
			if err := requireAuth(env); err != nil {
				return err
			}
			id, err := requireProjectID(env, args)
			if err != nil {
				return err
			}
			switch format {
			case "zip":
				path := outPath
				if path == "" {
					path = filepath.Join(".", id+".zip")
				}
				f, err := os.Create(path)
				if err != nil {
					return err
				}
				defer f.Close()
				n, err := env.Client.ExportZip(ctx, id, f)
				if err != nil {
					return err
				}
				fmt.Fprintf(env.Out, "%s wrote %d bytes to %s\n", ui.Green("✓"), n, path)
				return nil
			case "github":
				res, err := env.Client.ExportGitHub(ctx, id, repoName, description, private)
				if err != nil {
					return err
				}
				if env.JSON {
					b, _ := json.MarshalIndent(res, "", "  ")
					fmt.Fprintln(env.Out, string(b))
					return nil
				}
				url := res.HTMLURL
				if url == "" {
					url = res.RepoURL
				}
				fmt.Fprintln(env.Out, ui.Green("✓ exported to ")+url)
				return nil
			default:
				return fmt.Errorf("--format must be zip or github")
			}
		},
	}
}

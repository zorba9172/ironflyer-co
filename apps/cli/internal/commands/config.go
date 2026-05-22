package commands

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"ironflyer/apps/cli/internal/config"
	"ironflyer/apps/cli/internal/ui"
)

// configCmd reads + writes ~/.ironflyer/config.json keys. Supports the
// subset {host, token, project, userEmail}.
func configCmd() *Command {
	return &Command{
		Name:  "config",
		Short: "Read or write CLI config values",
		Usage: "ironflyer config get <key> | ironflyer config set <key> <value>",
		Subs: []*Command{
			{
				Name:  "get",
				Short: "Print a config value",
				Usage: "ironflyer config get <key>",
				Run: func(ctx context.Context, env *Env, fs *flag.FlagSet, args []string) error {
					if len(args) < 1 {
						return fmt.Errorf("usage: ironflyer config get <key>")
					}
					f, err := config.Load()
					if err != nil {
						return err
					}
					val, ok := readConfigKey(f, args[0])
					if !ok {
						return fmt.Errorf("unknown key: %s (try: host, token, project, userEmail)", args[0])
					}
					fmt.Fprintln(env.Out, val)
					return nil
				},
			},
			{
				Name:  "set",
				Short: "Write a config value",
				Usage: "ironflyer config set <key> <value>",
				Run: func(ctx context.Context, env *Env, fs *flag.FlagSet, args []string) error {
					if len(args) < 2 {
						return fmt.Errorf("usage: ironflyer config set <key> <value>")
					}
					f, err := config.Load()
					if err != nil {
						return err
					}
					if err := writeConfigKey(&f, args[0], args[1]); err != nil {
						return err
					}
					if err := config.Save(f); err != nil {
						return err
					}
					fmt.Fprintln(env.Out, ui.Green("saved ")+args[0])
					return nil
				},
			},
			{
				Name:  "path",
				Short: "Print the absolute path to the config file",
				Run: func(ctx context.Context, env *Env, fs *flag.FlagSet, args []string) error {
					p, err := config.Path()
					if err != nil {
						return err
					}
					fmt.Fprintln(env.Out, p)
					return nil
				},
			},
			{
				Name:  "show",
				Short: "Print the full config (token masked)",
				Run: func(ctx context.Context, env *Env, fs *flag.FlagSet, args []string) error {
					f, err := config.Load()
					if err != nil {
						return err
					}
					fmt.Fprintf(env.Out, "host:           %s\n", f.HostOrDefault())
					fmt.Fprintf(env.Out, "token:          %s\n", maskToken(f.Token))
					fmt.Fprintf(env.Out, "defaultProject: %s\n", f.DefaultProject)
					fmt.Fprintf(env.Out, "userEmail:      %s\n", f.UserEmail)
					return nil
				},
			},
		},
	}
}

func readConfigKey(f config.File, key string) (string, bool) {
	switch strings.ToLower(key) {
	case "host":
		return f.Host, true
	case "token":
		return f.Token, true
	case "project", "defaultproject":
		return f.DefaultProject, true
	case "useremail", "email":
		return f.UserEmail, true
	}
	return "", false
}

func writeConfigKey(f *config.File, key, val string) error {
	switch strings.ToLower(key) {
	case "host":
		f.Host = val
	case "token":
		f.Token = val
	case "project", "defaultproject":
		f.DefaultProject = val
	case "useremail", "email":
		f.UserEmail = val
	default:
		return fmt.Errorf("unknown key: %s (try: host, token, project, userEmail)", key)
	}
	return nil
}

// maskToken keeps the last 4 chars visible so users can tell which token
// is configured without revealing the full secret.
func maskToken(t string) string {
	if t == "" {
		return "<unset>"
	}
	if len(t) <= 4 {
		return "****"
	}
	return strings.Repeat("*", len(t)-4) + t[len(t)-4:]
}

func versionCmd() *Command {
	return &Command{
		Name:  "version",
		Short: "Print the CLI version",
		Run: func(ctx context.Context, env *Env, fs *flag.FlagSet, args []string) error {
			fmt.Fprintln(env.Out, Version)
			return nil
		},
	}
}

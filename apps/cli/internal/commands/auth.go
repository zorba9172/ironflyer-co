package commands

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"ironflyer/apps/cli/internal/config"
	"ironflyer/apps/cli/internal/ui"
)

// loginCmd implements `ironflyer login`. The flow:
//   1. Pick a free localhost port.
//   2. Open ${HOST}/login?cli=1&port=<port>&state=<random> in the browser.
//   3. Wait for the web app to POST/GET back the token to that port.
//   4. Persist the token to ~/.ironflyer/config.json.
//
// We support a `--token` flag for headless CI environments where the
// browser flow is impossible.
func loginCmd() *Command {
	var tokenFlag string
	var noBrowser bool
	return &Command{
		Name:  "login",
		Short: "Authenticate the CLI with the orchestrator",
		Long:  "Opens a browser to ${HOST}/login?cli=1 and listens on localhost for the token callback.\nUse --token to skip the browser flow (handy in CI).",
		Usage: "ironflyer login [--token TOKEN] [--no-browser]",
		Examples: []string{
			"ironflyer login",
			"ironflyer login --token $IRONFLYER_TOKEN",
		},
		RegFlags: func(fs *flag.FlagSet) {
			fs.StringVar(&tokenFlag, "token", "", "pre-issued bearer token (skips the browser flow)")
			fs.BoolVar(&noBrowser, "no-browser", false, "print the URL instead of opening it")
		},
		Run: func(ctx context.Context, env *Env, fs *flag.FlagSet, args []string) error {
			cfg := env.Config
			if cfg.Host == "" {
				cfg.Host = env.Host
			}
			if tokenFlag != "" {
				cfg.Token = tokenFlag
				if err := config.Save(cfg); err != nil {
					return err
				}
				fmt.Fprintln(env.Out, ui.Green("logged in (token saved to ~/.ironflyer/config.json)"))
				return nil
			}
			// Spin up the local callback server.
			ln, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				return fmt.Errorf("listen: %w", err)
			}
			port := ln.Addr().(*net.TCPAddr).Port
			state := randomState()
			tokenCh := make(chan string, 1)
			errCh := make(chan error, 1)
			mux := http.NewServeMux()
			mux.HandleFunc("/cli/callback", func(w http.ResponseWriter, r *http.Request) {
				q := r.URL.Query()
				if q.Get("state") != state {
					http.Error(w, "state mismatch", http.StatusBadRequest)
					errCh <- fmt.Errorf("state mismatch (CSRF check)")
					return
				}
				tok := q.Get("token")
				if tok == "" {
					http.Error(w, "missing token", http.StatusBadRequest)
					errCh <- fmt.Errorf("callback missing token")
					return
				}
				// Plain HTML response so the browser shows something friendly.
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				fmt.Fprint(w, `<!doctype html><html><body style="font-family:system-ui;padding:2rem"><h2>You can close this tab.</h2><p>Ironflyer CLI is signed in.</p></body></html>`)
				tokenCh <- tok
			})
			server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
			go func() {
				if err := server.Serve(ln); err != nil && err != http.ErrServerClosed {
					errCh <- err
				}
			}()
			defer server.Shutdown(context.Background())

			callback := fmt.Sprintf("http://127.0.0.1:%d/cli/callback", port)
			authURL := strings.TrimRight(env.Host, "/") + "/login?cli=1&" + url.Values{
				"redirect": {callback},
				"state":    {state},
			}.Encode()

			fmt.Fprintln(env.Out, "Open this URL in your browser to finish login:")
			fmt.Fprintln(env.Out, "  "+ui.Cyan(authURL))
			if !noBrowser {
				_ = openBrowser(authURL)
			}
			spinner := ui.NewSpinner("waiting for browser callback…")
			spinner.Start()
			defer spinner.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case err := <-errCh:
				return err
			case tok := <-tokenCh:
				cfg.Token = tok
				cfg.Host = env.Host
				if err := config.Save(cfg); err != nil {
					return err
				}
				spinner.Stop()
				fmt.Fprintln(env.Out, ui.Green("logged in (token saved to ~/.ironflyer/config.json)"))
				return nil
			case <-time.After(5 * time.Minute):
				return fmt.Errorf("timed out waiting for browser callback")
			}
		},
	}
}

// logoutCmd implements `ironflyer logout`. We delete the config rather
// than just clearing the token so a stale host doesn't trip up next
// login.
func logoutCmd() *Command {
	return &Command{
		Name:  "logout",
		Short: "Clear the saved bearer token",
		Usage: "ironflyer logout",
		Run: func(ctx context.Context, env *Env, fs *flag.FlagSet, args []string) error {
			if err := config.Clear(); err != nil {
				return err
			}
			fmt.Fprintln(env.Out, ui.Green("logged out (config cleared)"))
			return nil
		},
	}
}

// whoamiCmd implements `ironflyer whoami`. Hits /auth/me.
func whoamiCmd() *Command {
	return &Command{
		Name:  "whoami",
		Short: "Show the currently authenticated user",
		Usage: "ironflyer whoami",
		Run: func(ctx context.Context, env *Env, fs *flag.FlagSet, args []string) error {
			if err := requireAuth(env); err != nil {
				return err
			}
			u, err := env.Client.Me(ctx)
			if err != nil {
				return err
			}
			if env.JSON {
				b, _ := json.MarshalIndent(u, "", "  ")
				fmt.Fprintln(env.Out, string(b))
				return nil
			}
			fmt.Fprintf(env.Out, "%s %s\n", ui.Bold("id:   "), u.ID)
			fmt.Fprintf(env.Out, "%s %s\n", ui.Bold("email:"), u.Email)
			if u.Name != "" {
				fmt.Fprintf(env.Out, "%s %s\n", ui.Bold("name: "), u.Name)
			}
			plan := u.Plan
			if plan == "" {
				plan = "free"
			}
			fmt.Fprintf(env.Out, "%s %s\n", ui.Bold("plan: "), plan)
			return nil
		},
	}
}

// openBrowser tries to launch a browser. Failures are non-fatal: we
// already printed the URL.
func openBrowser(target string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", target)
	case "linux":
		cmd = exec.Command("xdg-open", target)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	default:
		return fmt.Errorf("unsupported OS")
	}
	return cmd.Start()
}

// randomState mints a 16-byte hex string for the OAuth-style state
// parameter that protects against CSRF on the callback.
func randomState() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

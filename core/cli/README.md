# ironflyer CLI

A power-user command-line companion to the Ironflyer orchestrator. Lets
you drive the AI Product Finisher (`Spec → UX → Architecture → Code →
Lint → Tests → Security → Deploy`) without leaving the terminal.

## Install

### macOS (Homebrew, planned)

```sh
brew tap ironflyer/tap
brew install ironflyer
```

### Linux (curl pipe, planned)

```sh
curl -fsSL https://ironflyer.dev/install.sh | sh
```

### Windows (Scoop, planned)

```powershell
scoop bucket add ironflyer https://github.com/ironflyer/scoop-bucket
scoop install ironflyer
```

### From source (works today)

```sh
go install ironflyer/core/cli/cmd/ironflyer@latest
```

Or from a local checkout:

```sh
cd core/cli && make build && ./dist/ironflyer --help
```

## Quick start

```sh
ironflyer login                              # opens a browser, captures the token
ironflyer projects create --name todo --prompt 'a todo app with auth'
ironflyer run todo                           # streams agent events live
ironflyer patches todo                       # list proposed patches
ironflyer deploy todo --provider fly --region iad
ironflyer export todo --format zip
```

## Config

The CLI stores its state in `~/.ironflyer/config.json`:

```json
{
  "host": "https://api.ironflyer.dev",
  "token": "ey...",
  "defaultProject": "todo",
  "userEmail": "you@example.com"
}
```

- `--host` overrides the configured host for one invocation (handy for
  pointing at `http://localhost:8080` during development).
- `--token` overrides the saved bearer token.
- `--json` makes every command emit machine-readable JSON. Combine with
  `jq` to script the orchestrator.

## Commands

| Command                     | Description |
| --------------------------- | ----------- |
| `ironflyer login`           | Browser-based bearer-token capture. |
| `ironflyer logout`          | Clear the saved token. |
| `ironflyer whoami`          | Show the current user + plan. |
| `ironflyer projects`        | List your projects (alias `ls`). |
| `ironflyer projects create` | Create a project from an idea prompt. |
| `ironflyer projects show`   | Show gates, recent patches, current spec. |
| `ironflyer projects delete` | Delete a project (irreversible). |
| `ironflyer run <id>`        | Trigger the finisher and stream events. |
| `ironflyer logs <id>`       | Subscribe to the event stream without re-running. |
| `ironflyer patches <id>`    | List patches; `--apply <id>` applies one. |
| `ironflyer deploy <id>`     | Deploy via fly or railway, stream the build log. |
| `ironflyer export <id>`     | Export as `--format zip` or `--format github`. |
| `ironflyer status`          | Ping orchestrator + runtime healthz. |
| `ironflyer config get/set`  | Read/write a config key. |
| `ironflyer version`         | Print the CLI version. |

## Building releases

```sh
make release    # cross-compiles darwin/linux/windows × amd64/arm64
```

Output lands in `dist/` along with a `SHA256SUMS` file.

## License

Same as the Ironflyer monorepo.

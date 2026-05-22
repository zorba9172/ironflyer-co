# Ironflyer for VSCode

Finish products end-to-end with [Ironflyer](https://ironflyer.dev) — without leaving your editor.

## What you get

- **Projects sidebar** — every Ironflyer project you own, one click away.
- **Chat panel** — streams from the orchestrator's `chatStream` endpoint with
  the same Lite/Economy/Power dial as the web app, and the same agent roles
  (planner / architect / coder / reviewer / tester / security).
- **Patches view** — every patch the orchestrator has proposed, grouped by
  project. Click any file change to open a VSCode side-by-side diff between
  the current file and the proposed content. Inline ✔ applies the patch
  (with a confirm prompt) through the orchestrator's lifecycle.
- **Finisher Gates view** — Spec / UX / Arch / Code / Lint / Test / Security
  / Deploy for every project, with status colors, an issues drill-down, and
  an inline ▶ to run the Finisher pass.
- **Live updates** — while a chat panel is open, the extension subscribes
  to the project's `/stream` and refreshes the Patches and Gates trees
  automatically when an agent updates state. Lifecycle events also surface
  in the chat log so progress is visible without watching the trees.
- **Ask Ironflyer to fix** — every editor diagnostic (TypeScript, ESLint,
  Go, anything) lights up a Quick Fix that bundles the message, code, and
  the surrounding snippet, opens the chat for your pinned project, and
  routes the request to the `coder` agent. Set the pinned project once
  via `Ironflyer: Set Active Project`.
- **Run Finisher** — kick a full Spec → UX → Arch → Code → Test → Security → Deploy
  pass from the command palette.
- **Budget status** — current plan and month-to-date spend in the status bar;
  click for a full snapshot.

The extension is a **thin client** — auth, AI provider calls, budget, and
patch state all live in the orchestrator. The JWT is held in VSCode
`SecretStorage`; it never enters settings.json.

## Sign-in flow

`Ironflyer: Sign In` opens the web app's login page with a callback URL
pointing back at this extension. After you sign in (email / password,
Google, or GitHub), the web app redirects to
`vscode://ironflyer.ironflyer/auth?token=…` and the extension stores the
token. There is no API key to paste anywhere.

## Settings

| Setting | Default | Purpose |
| --- | --- | --- |
| `ironflyer.orchestratorUrl` | `http://localhost:8080` | Where the orchestrator lives. Set to `https://api.ironflyer.dev` for production. |
| `ironflyer.webUrl` | `http://localhost:3000` | Used for the sign-in handshake. |

## Building from source

```bash
cd apps/vscode-extension
npm install
npm run build          # produces dist/extension.js
npm run package        # produces ironflyer-0.1.0.vsix
```

To debug, open this folder in VSCode and press `F5` — a new Extension
Development Host window launches with the extension loaded.

## Publishing

Two registries: the Microsoft VS Marketplace (`vsce`) and Open VSX
(`ovsx`, the registry Cursor / VSCodium / Theia use). Local dry-run:

```bash
npm run package
code --install-extension ironflyer-0.1.0.vsix
```

Real publish requires:
- A `publisher` on https://marketplace.visualstudio.com matching the
  `publisher` field in package.json (currently `ironflyer`).
- A Personal Access Token from https://dev.azure.com — store as
  `VSCE_PAT` in the repo's secrets.
- An Open VSX namespace (https://open-vsx.org) and a `OVSX_PAT` secret.

Then push a tag of the form `vscode-v<version>` — the
[`release-vscode`](../../.github/workflows/release-vscode.yml) workflow
runs the full verify pipeline, packages the `.vsix`, publishes to both
registries when the matching secret is set, and attaches the artifact to
a GitHub Release. `workflow_dispatch` exposes a `dry_run` toggle that
runs the verify + package steps without publishing.

## Roadmap

- LSP-proxy fan-out so the workspace runtime's LSPs surface in VSCode.
- "Apply patch" mode that streams the agent's proposed FileChange events
  straight into the Patches tree (today the user still hits refresh once).
- Workspace-folder ↔ remote-workspace sync for users who edit locally.

See `apps/vscode-extension/src/extension.ts` for the entrypoint.

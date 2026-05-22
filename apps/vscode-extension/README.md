# Ironflyer — AI Product Finisher for VSCode

> Ship products end-to-end without leaving your editor. Ironflyer turns
> your idea into a working, gated, deployable app — and the VSCode
> extension is your cockpit.

![Ironflyer hero](media/hero.png)

---

## What it is

**Ironflyer is an AI Product Finisher.** It takes an idea and shepherds
it through enforced gates — **Spec → UX → Architecture → Code → Lint →
Tests → Security → Deploy** — that block until they pass. No demoware,
no "almost done"; either the gates are green or you know exactly which
one is red.

The VSCode extension is the **delightful thin client** for the platform:

- **Projects sidebar** — every Ironflyer project you own, one click away.
- **Chat panel** — streams from the orchestrator with the same Lite /
  Economy / Power dial and agent roles (planner / architect / coder /
  reviewer / tester / security) as the web app.
- **Live Preview** — a sandboxed iframe rendering your project's
  workspace in real time, with mobile / tablet / desktop viewport
  presets and a one-click refresh.
- **Finisher Gates view** — Spec / UX / Arch / Code / Lint / Test /
  Security / Deploy for every project, with status colors, a "last
  updated" timestamp, an issues drill-down, and right-click to **re-run
  a single gate**.
- **Patches view** — every patch the orchestrator has proposed, grouped
  by project. Click any file change to open a VSCode side-by-side diff;
  hit the checkmark to apply through the orchestrator's lifecycle.
- **Run output channel + toast actions** — every Finisher event tails
  into a dedicated `Ironflyer Run` channel, and big moments
  (`gate_failed`, `run_complete`, `patch_proposed`) surface as
  notifications with **Review / Apply / Dismiss** quick actions.
- **Ask Ironflyer to fix** — every editor diagnostic (TypeScript, ESLint,
  Go, anything) lights up a Quick Fix that bundles the message, code,
  and surrounding snippet, opens the chat for your pinned project, and
  routes the request to the `coder` agent.
- **Status bar cockpit** — the pinned project, last gate status, budget
  remaining, and a one-tap **Run** button.
- **Live updates** — while a project is pinned the extension subscribes
  to `/projects/{id}/stream` and refreshes the trees and preview as the
  agent makes progress.

The extension is a **thin client**. Auth, AI provider calls, budget, and
patch state all live in the orchestrator. Your JWT is held in VSCode
`SecretStorage`; it never enters `settings.json`.

---

## Install

From the marketplace inside VSCode:

```
ext install ironflyer.ironflyer
```

Or download the latest `.vsix` from
[releases](https://github.com/zorba9172/ironflyer/releases) and run:

```
code --install-extension ironflyer-0.3.0.vsix
```

> Cursor, VSCodium, Theia and other Open VSX-based editors are
> supported via [Open VSX](https://open-vsx.org/extension/ironflyer/ironflyer).

---

## Sign in

`Ironflyer: Sign In` opens the web app's login page with a callback URL
pointing back at this extension. After you sign in (email / password,
Google, or GitHub), the web app redirects to
`vscode://ironflyer.ironflyer/auth?token=…` and the extension stores
the JWT in `SecretStorage`. **There is no API key to paste anywhere.**

![Sign-in flow](media/sign-in.png)

---

## Commands

| Command | Default keybinding | What it does |
| --- | --- | --- |
| `Ironflyer: Sign In` |  | Browser handshake → JWT in `SecretStorage`. |
| `Ironflyer: Sign Out` |  | Drops the JWT. |
| `Ironflyer: New Project` | `Ctrl/Cmd+Alt+N` | Create a project, auto-pin it, open chat. |
| `Ironflyer: Open Chat` | `Ctrl/Cmd+Alt+I` | Streaming chat for the active project. |
| `Ironflyer: Open Live Preview` | `Ctrl/Cmd+Alt+P` | Reveal the sandboxed iframe in the Ironflyer sidebar. |
| `Ironflyer: Run Finisher` | `Ctrl/Cmd+Alt+R` | Kick a full Spec→Deploy pass; tails events to the Run channel. |
| `Ironflyer: Re-run this Gate` |  | Right-click a gate in the Finisher Gates view. |
| `Ironflyer: Apply Patch` |  | Confirm + `POST /patches/{id}/apply`. |
| `Ironflyer: Reject Patch` |  | Mark a proposed patch as rejected. |
| `Ironflyer: Set Active Project` |  | Pin the project for this VSCode window. |
| `Ironflyer: Show Budget` |  | Plan tier + month-to-date spend. |
| `Ironflyer: Quick Actions` |  | Palette listing every Ironflyer surface. |
| `Ironflyer: Open Project in Browser` |  | Open the web dashboard for this project. |
| `Ironflyer: Open Preview in Browser` |  | Open the runtime preview URL outside VSCode. |
| `Ironflyer: Refresh` / `Refresh Preview` |  | Force-refresh the trees / preview iframe. |
| `Ironflyer: Show Logs` / `Show Run Output` |  | Surface the extension's two output channels. |

---

## Settings

| Setting | Default | Purpose |
| --- | --- | --- |
| `ironflyer.orchestratorUrl` | `http://localhost:8080` | Where the orchestrator lives. Production: `https://api.ironflyer.dev`. |
| `ironflyer.apiUrl` | _empty_ | Friendly alias for `orchestratorUrl`. When set it wins. |
| `ironflyer.runtimeUrl` | `http://localhost:8090` | Where the per-user workspace runtime lives. Used by Live Preview. |
| `ironflyer.webUrl` | `http://localhost:3000` | Web app URL — used for the sign-in handshake. |
| `ironflyer.defaultProject` | _empty_ | Project ID auto-pinned on activation when no pin exists. |
| `ironflyer.preview.defaultViewport` | `responsive` | Initial viewport: `responsive` / `mobile` / `tablet` / `desktop`. |

Settings are read every API call, so toggling between local and
production is a one-line change — no restart required.

---

## The Live Preview

![Live Preview](media/preview.png)

The preview pane is a sandboxed `<iframe>` pointing at the runtime's
per-workspace preview URL. The toolbar lets you:

- **Refresh** the iframe with a cache-busting timestamp.
- **Open in browser** for full-page testing.
- **Switch viewport** between Responsive, Mobile (390×844), Tablet
  (820), and Desktop (1280) — handy for catching responsive bugs while
  the agent codes.

If no workspace exists for the pinned project yet, the pane shows an
empty state with **Run Finisher** and **Provision Workspace** buttons.

---

## Troubleshooting

**"Sign in" opens the browser but VSCode never gets the token.**
The callback URL is `vscode://ironflyer.ironflyer/auth?token=…`. If
you're in an OS / browser that strips custom URI schemes (some corporate
environments do), paste the token via `Developer: Reload Window` after
copying it from the URL bar.

**"Could not read budget" in the status bar.**
The status bar polls `/budget/users/me` every 60s. A red pill means the
orchestrator is unreachable or the JWT has expired. Click it to retry;
the extension will prompt you to re-sign-in on 401.

**The Live Preview shows "No live preview yet".**
The runtime hasn't provisioned a workspace for that project, or the
workspace is still booting. Click **Run Finisher** (or **Provision
Workspace**), then wait for the orchestrator to emit `workspace_ready`
in the Run output channel.

**Patches don't apply / get rejected by the orchestrator.**
Every patch goes through the Finisher lifecycle gates — a `lint` or
`test` failure on the proposed change will block the apply. Open the
**Finisher Gates** view to see which gate is red; the gate row expands
to show the failure reason.

**I want to point at a self-hosted Ironflyer.**
Set `ironflyer.orchestratorUrl`, `ironflyer.runtimeUrl`, and
`ironflyer.webUrl` to your install. The JWT issuer must match the
orchestrator's signing secret.

**Where do the extension logs go?**
`View → Output → Ironflyer` for the extension log,
`View → Output → Ironflyer Run` for Finisher event tails.

---

## Building from source

```bash
cd apps/vscode-extension
npm install
npm run build          # produces dist/extension.js
npm run package        # produces ironflyer-0.3.0.vsix
```

To debug, open this folder in VSCode and press `F5` — a new Extension
Development Host window launches with the extension loaded.

---

## Publishing

Two registries: the Microsoft VS Marketplace (`vsce`) and Open VSX
(`ovsx`, the registry Cursor / VSCodium / Theia use). Local dry-run:

```bash
npm run package
code --install-extension ironflyer-0.3.0.vsix
```

Real publish requires:

- A `publisher` on https://marketplace.visualstudio.com matching the
  `publisher` field in `package.json` (`ironflyer`).
- A Personal Access Token from https://dev.azure.com — store as
  `VSCE_PAT` in repo secrets.
- An Open VSX namespace (https://open-vsx.org) and an `OVSX_PAT` secret.

Then push a tag of the form `vscode-v<version>` — the
[`release-vscode`](../../.github/workflows/release-vscode.yml) workflow
runs the full verify pipeline, packages the `.vsix`, publishes to both
registries when the matching secret is set, and attaches the artifact
to a GitHub Release.

---

## Roadmap

- LSP-proxy fan-out so the workspace runtime's LSPs surface in VSCode.
- Streaming `FileChange` events into the Patches tree (today the user
  still hits refresh once after the Finisher emits `patch_proposed`).
- Workspace-folder ↔ remote-workspace sync for users who edit locally.

---

## Media assets the extension expects

The screenshots referenced above are not yet checked into the repo. To
ship marketing-grade visuals, drop the following files into
`apps/vscode-extension/media/`:

| Path | Purpose |
| --- | --- |
| `media/icon.png` | 128×128 PNG marketplace icon (matches `icon` in `package.json`). |
| `media/hero.png` | ~1600×900 hero banner showcasing the activity-bar + preview + chat. |
| `media/sign-in.png` | Screenshot of the sign-in handshake (browser → VSCode). |
| `media/preview.png` | Screenshot of the Live Preview pane with the viewport toggle visible. |
| `media/patches.png` | Screenshot of a proposed patch in the Patches view + diff editor. |
| `media/gates.png` | Screenshot of the Finisher Gates view with a failed gate expanded. |
| `media/status-bar.png` | Close-up of the status bar (project pill + gate badge + budget + Run). |

`media/icon.svg` and the walkthrough markdown under `media/walkthrough/`
already exist and are used by the Getting Started walkthrough.

See `apps/vscode-extension/src/extension.ts` for the entrypoint.

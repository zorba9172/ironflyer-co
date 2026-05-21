# Ironflyer for VSCode

Finish products end-to-end with [Ironflyer](https://ironflyer.dev) — without leaving your editor.

## What you get

- **Projects sidebar** — every Ironflyer project you own, one click away.
- **Chat panel** — streams from the orchestrator's `chatStream` endpoint with
  the same Lite/Economy/Power dial as the web app, and the same agent roles
  (planner / architect / coder / reviewer / tester / security).
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

## Roadmap (not in 0.1)

- Inline patch review (diff explorer rendering orchestrator patch lifecycle).
- Quick-fix code actions wired to repair agents.
- LSP-proxy fan-out so the workspace runtime's LSPs surface in VSCode.

See `apps/vscode-extension/src/extension.ts` for the entrypoint.

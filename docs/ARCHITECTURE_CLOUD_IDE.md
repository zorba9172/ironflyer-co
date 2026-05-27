# Cloud IDE Architecture (historical)

Status: **superseded 2026-05-28**. The embedded openvscode-server / code-server
iframe was removed from Studio. The in-Studio code surface is now Monaco
only; the professional IDE path is the standalone Ironflyer VS Code
Extension at `clients/vscode-extension/`, which brings gates, patches,
and the wallet into the operator's local VS Code rather than running a
per-user IDE container.

This document is kept as the rationale trail for the decision and the
infra contract that lived in the codebase before removal. Do not
re-introduce the iframe path without first revisiting that decision —
the trade-off was: kill per-user container cost, keep Ironflyer chrome
front-and-center, route pro users to the extension.

---

IronFlyer Studio is a cloud product builder. The web Studio remains the product cockpit, while each real workspace gets a VS Code-compatible cloud IDE backed by the runtime sandbox.

## Open-Source Base

The product direction is VS Code cloud, built on an open project:

- Primary base: OpenVSCode Server for the Studio iframe, because it stays closest to upstream VS Code behavior and accepts the slim IronFlyer profile cleanly.
- Legacy fallback: `code-server`, because the runtime already builds `infra/docker/ironflyer-code.Dockerfile`, the Docker driver returns `ideUrl`, and local compose can expose the same branded settings profile.
- Not the current base: Eclipse Theia. Theia is powerful for custom IDE products, but IronFlyer already has a code-server runtime path and needs a VS Code-compatible workspace quickly.

Reference notes:

- code-server runs VS Code in the browser and is designed for remote/self-hosted development.
- OpenVSCode Server provides upstream VS Code on a remote machine through a browser.
- Theia is an extensible cloud/desktop IDE framework, useful for a deeper future custom IDE but not the current fastest path.

## Product Contract

The Studio surface must match `design-reference/2026-05-25-private-ironflyer/`:

- `/studio` is a full IDE-style builder, not a marketing page.
- `/p/[projectID]` is the live execution workspace.
- The cloud IDE appears as an IDE action or tab in the Studio shell, using the execution/workspace identity.
- The embedded or launched IDE uses the same IronFlyer dark-violet/coral design language. No lime-first legacy theme.
- The IDE image ships a slim profile: hidden activity bar, compact tabs/status bar, no welcome walkthroughs, no extension recommendations, telemetry and updates off, and watcher/search excludes for heavy generated folders.
- The IDE must open the same project snapshot as Monaco. In local dev, `/api/ide/sync` mirrors `projectFiles` into `.ironflyer/ide-sync/projects/<projectID>` and OpenVSCode mounts that folder at `/home/workspace/projects/<projectID>`. The sync route keeps a hash manifest so later GraphQL refreshes do not overwrite files edited inside VS Code; the Code pane can pull the IDE snapshot back into Monaco for review. In production, this responsibility moves to the runtime workspace/signed IDE route.
- Preview, files, code, patches, deploy, wallet, ledger, ProfitGuard, and support bundle remain connected to the same execution.

## Runtime Contract

Runtime already owns the primitives:

- workspace lifecycle
- file operations
- PTY/terminal
- exec
- preview proxy
- screenshot
- patch apply
- archive/restore
- `ideUrl` from the Docker workspace driver

Cloud IDE production routing should formalize signed IDE URLs under a runtime or edge-owned route such as `/ide/{workspaceID}`. Direct host-mapped local URLs are acceptable only in development.

## Web Integration Contract

The web app should expose the cloud IDE without breaking the cockpit:

1. Resolve the active `workspaceID` from the execution.
2. Request or derive a signed IDE URL from runtime/orchestrator.
3. Open the IDE in a contained Studio tab or a deliberate new-window action.
4. Keep the Studio preview/code/files panels usable even when the full IDE is unavailable.
5. Guest `/studio` may show an interactive IDE-style demo, but real workspace creation requires an authenticated paid/live execution.

## Security And Cost

- No public unauthenticated IDE URLs. Local dev removes the nested IDE login/token so the embedded editor opens cleanly after the IronFlyer app session; production must enforce access at the signed runtime/edge IDE route.
- IDE URLs must be scoped to tenant, workspace, and expiration.
- Workspace credentials must not be persisted into snapshots.
- Idle IDE sessions are archived and destroyed under the runtime scale policy.
- ProfitGuard and wallet rules still govern sandbox allocation for real workspaces.

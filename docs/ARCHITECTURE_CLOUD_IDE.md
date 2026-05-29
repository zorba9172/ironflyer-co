# Cloud IDE Architecture

Status: **active 2026-05-29**. The canonical embedded code IDE is a
custom-branded **Eclipse Theia** browser app, built at `clients/ide/`,
shipped as the image `ironflyer/theia-ide:latest` (listens on `:3030`,
opens `/home/coder`). The runtime provisions one per workspace and the
studio embeds it as an iframe in the `CodePane` via the runtime
`GET /workspaces/{id}/ide` endpoint. The standalone Ironflyer VS Code
Extension at `clients/vscode-extension/` remains the local-editor path
for teams that prefer desktop VS Code.

IronFlyer Studio is a cloud product builder. The web Studio is the
product cockpit; each real workspace gets a branded cloud IDE backed by
the runtime sandbox. Ironflyer stays visualization-first, so the IDE is
the opt-in "open the hood" surface for professionals — reachable in one
click but never the default landing pane.

---

## IDE base — custom-branded Eclipse Theia

The embedded IDE is a custom Theia distribution under `clients/ide/`,
not an off-the-shelf VS Code server. We assemble exactly the editor we
want and theme it to the Ironflyer dark system end-to-end:

- **Full control over branding.** The chrome, splash, accent, and
  product name are ours, not upstream VS Code's. No lime-first legacy
  theme.
- **Lean extension set.** Theia lets us bundle only the extensions a
  generated workspace needs, instead of inheriting the full VS Code
  surface.
- **No marketplace / telemetry baggage.** We do not depend on the
  Microsoft VS Code marketplace, and there is no upstream telemetry to
  switch off — the distro never ships it.

Image: `ironflyer/theia-ide:latest`, built from `clients/ide/`
(`docker build -t ironflyer/theia-ide:latest clients/ide`). The
container listens on `:3030` and opens `/home/coder` — the same
host-path mount the runtime sandbox uses for files, terminal, exec, and
preview.

### History — why we moved off code-server / OpenVSCode

Earlier iterations targeted a VS Code-compatible browser editor:
`code-server` (the runtime once built `infra/docker/ironflyer-code.Dockerfile`)
and then OpenVSCode Server, chosen for staying close to upstream VS Code
behavior and accepting the slim profile quickly. That path got us a
working IDE fast, but it tied the product to upstream VS Code branding,
the VS Code marketplace, and upstream telemetry/update plumbing we had
to keep suppressing. We moved to a custom Theia distro for the control
above. The runtime still keeps a registry-pullable code-server fallback
(see "Runtime selection" below) so an unconfigured deployment boots a
working IDE, but Theia is the canonical, intended surface.

## Slim profile requirements

The branded IDE image ships a slim profile (satisfied by `clients/ide/`):

- hidden menubar / compact activity bar, compact tabs and status bar
- no welcome page, no walkthroughs, no extension recommendations
- telemetry and updates off
- watcher/search excludes for heavy generated folders

## Runtime selection

The runtime Docker driver provisions the IDE container as part of
workspace `Create`. Image and in-container port are env-selected:

- `IRONFLYER_IDE_IMAGE=ironflyer/theia-ide:latest` — the canonical
  branded Theia IDE.
- `IRONFLYER_IDE_CONTAINER_PORT=3030` — Theia's in-container port.

The compiled-in default stays `codercom/code-server:latest` on `8080`,
because — unlike the locally-built Theia image — it is registry-pullable,
so a deployment that relies on Docker pulling the default still boots a
working IDE without any extra setup. The driver keys per-image run args
off the image name: code-server receives `--auth none --disable-telemetry`,
while Theia is launched bare (its entrypoint takes no such flags). Setting
the two env vars above is the documented, non-breaking way to make Theia
the active IDE once the image is built locally or pushed to your registry.

## Product Contract

The Studio surface must match `design-reference/2026-05-25-private-ironflyer/`:

- `/studio` is a full IDE-style builder, not a marketing page.
- `/p/[projectID]` is the live execution workspace.
- The cloud IDE appears as a code tab in the Studio shell (the studio
  `CodePane`), using the execution/workspace identity. The studio's
  `IdeTopBar` owns the surrounding chrome (Ironflyer wordmark, project
  name, Pro·code marker) so the embedded Theia frame stays bare.
- The embedded IDE uses the same Ironflyer dark design language. No
  lime-first legacy theme.
- The IDE opens the same workspace files as Studio: it mounts the
  runtime sandbox's `/home/coder`, so files, terminal, exec, and
  preview all operate on one workspace — no separate file-sync layer.
- Preview, files, code, patches, deploy, wallet, ledger, ProfitGuard,
  and support bundle remain connected to the same execution.

## Runtime Contract

Runtime owns the primitives:

- workspace lifecycle
- file operations
- PTY/terminal
- exec
- preview proxy
- screenshot
- patch apply
- archive/restore
- `ideUrl` from the Docker workspace driver

The studio reaches the IDE through `GET /workspaces/{id}/ide`, which
returns `{"url": <loopback host:port of the IDE container>, "ready":
true}` (200) once the backend is up, or `{"url": "", "ready": false}`
(202) while it boots, prompting the client to poll. In local dev,
`IRONFLYER_IDE_URL` overrides the lookup so a developer can run the
Theia app on `:3030` without Docker (e.g. `yarn start` in `clients/ide/`).
This runtime IDE route is REST by design (the GraphQL-only rule governs
the orchestrator, not the runtime). Production routing should keep
signed, expiring IDE URLs scoped to tenant + workspace.

## Web Integration Contract

The studio exposes the cloud IDE without breaking the cockpit:

1. Resolve the active `workspaceID` from the execution / project.
2. Request the IDE URL from the runtime (`GET /workspaces/{id}/ide`),
   polling while `ready` is false.
3. Embed the IDE in the contained Studio `CodePane` tab (iframe), with a
   branded top accent and loading/offline lifecycle states.
4. Keep the Studio preview/code/files panels usable even when the full
   IDE is unavailable.
5. Guest `/studio` may show an interactive IDE-style demo, but real
   workspace creation requires an authenticated paid/live execution.

## Security And Cost

- No public unauthenticated IDE URLs. The IDE container binds to
  loopback only; the runtime reverse-proxy dials it. Local dev removes
  the nested IDE login/token so the embedded editor opens cleanly after
  the Ironflyer app session; production must enforce access at the
  signed runtime/edge IDE route.
- IDE URLs must be scoped to tenant, workspace, and expiration.
- Workspace credentials must not be persisted into snapshots.
- Idle IDE sessions are archived and destroyed under the runtime scale
  policy.
- ProfitGuard and wallet rules still govern sandbox allocation for real
  workspaces.

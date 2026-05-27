# Changelog

## 0.3.1

- Fix Marketplace listing metadata: homepage points at the live brand domain (`ironflyer.ai`), repository and bugs URLs point at the actual GitHub repo (`zorba9172/ironflyer-co`), and the production orchestrator URL in the configuration description is corrected.
- README cleanup: remove the broken `media/hero.png` include so the Marketplace listing renders cleanly until the screenshot capture pass lands.
- Bump install snippet to `ironflyer-0.3.1.vsix`.

## 0.3.0

- Apollo Client wiring: GraphQL operations now share a single Apollo Client with persisted-query support, matching the web cockpit's transport.
- URI handler (`onUri`): the extension activates on the auth callback from the web sign-in flow and stores the JWT in `SecretStorage`.
- Sentry integration: opt-in error reporting with safe defaults; user opt-out via `ironflyer.sentry.enabled = false`.
- Apollo + SSE alignment with the orchestrator's new `executionFeed` subscription.
- Audit / Memory / Telemetry tree views alongside the existing Projects / Gates / Patches sidebars.

## 0.2.0

- **Quick Fix code actions** — every editor diagnostic offers "Ask Ironflyer to fix", which bundles the snippet + message + range and routes it to the coder agent in the pinned project.
- **Active project pin** — `Ironflyer: Set Active Project` persists one project per VSCode window in `workspaceState`.
- **Finisher Gates view** — third sidebar TreeView: project → gate (Spec/UX/Arch/Code/Lint/Test/Security/Deploy) → issue, with status colors and an inline ▶ Run Finisher.
- **Patches view** — second sidebar TreeView with native side-by-side diff via an `ironflyer://` content provider, plus inline ✔ Apply.
- **Live updates via SSE** — while a chat panel is open the extension subscribes to `/projects/{id}/stream` and refreshes Patches + Gates in place. Lifecycle events appear inline in the chat log.
- **Output channel logger** — `Ironflyer: Show Logs` reveals the channel; warn/error events route there instead of the dev console.
- **Onboarding walkthrough** — Get-started tour available from `Help → Welcome → Get started with Ironflyer`.
- **Keybindings** — `Cmd+Alt+I` opens chat, `Cmd+Alt+N` creates a new project (when signed in).
- **Marketplace pipeline** — `release-vscode` GitHub workflow publishes to VS Marketplace + Open VSX on `vscode-v*` tags.

Tests: 35 unit cases across SSE parser, patch URIs, gate icons, throttler, and fix-prompt formatter.

## 0.1.0

- Initial release. Projects sidebar, streaming chat webview, sign-in via web → SecretStorage, Run Finisher, budget status bar, New Project command.

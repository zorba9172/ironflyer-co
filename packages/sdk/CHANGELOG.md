# Changelog

All notable changes to `@ironflyer/sdk` are documented here. Format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project
adheres to [Semantic Versioning](https://semver.org/).

## [0.1.0] - 2026-05-24

### Added

- Initial public release of the Ironflyer TypeScript SDK.
- `Ironflyer` class with typed methods for ~50 queries and mutations:
  - Auth: `signIn`, `signUp`, `signOut`, `me`.
  - Projects: `projects`, `project`, `projectFiles`, `projectGraph`,
    `projectSnapshot`, `searchProjectCode`, `createProject`,
    `updateProject`, `deleteProject`, `bulkDeleteProjects`.
  - Finisher gates: `gates`, `rerunGate`, `runFinisher`.
  - Patches: `patches`, `proposePatch`, `applyPatch`, `rollbackPatch`.
  - Budget + billing: `myBudget`, `plans`, `rates`, `vault`,
    `mySubscription`, `startCheckout`, `cancelSubscription`.
  - Memory: `memory`, `addMemory`, `deleteMemory`.
  - Audit: `audit`, `verifyAudit`.
  - Webhooks: `webhooks`, `createWebhook`, `deleteWebhook`,
    `testWebhook`.
  - Deploys: `deploys`, `startDeploy`.
  - Inline completions: `acceptInlineCompletion`.
  - Chats: `chats`, `chatMessages`, `createChat`, `forkChat`.
  - Workspaces: `workspaces`, `workspace`, `workspaceFiles`,
    `workspaceFile`, `createWorkspace`, `startWorkspace`,
    `stopWorkspace`, `writeWorkspaceFile`, `execInWorkspace`.
- Real-time subscriptions exposed as `AsyncIterable`:
  - `runProject`, `chatStream`, `inlineCompletion`, `deployStream`,
    `workspacePty`, `costStream`.
- Bearer token auth with hot-swappable `setToken(...)`.
- ESM + CJS dual build via `tsup`; declaration files with sourcemaps.
- Node 18+ engine guard. Browser, Bun, Deno compatible.

[0.1.0]: https://github.com/ironflyer/ironflyer/releases/tag/sdk-v0.1.0

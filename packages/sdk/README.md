# @ironflyer/sdk

TypeScript client for the Ironflyer **AI Product Finisher** — orchestrator
(`/projects`, `/budget`, `/auth`, GitHub integration, streaming chat) and
workspace runtime (`/workspaces`, file I/O, exec, PTY URL).

Zero runtime dependencies; works in browsers, Node 20+, Bun, and Deno.

## Install

The SDK is an internal monorepo package today — depend on it from another
workspace via your package manager (`npm`, `pnpm`, `yarn`). It is **not**
published to npm.

## Usage

```ts
import { ironflyer } from '@ironflyer/sdk';

const ifc = ironflyer({
  orchestratorUrl: 'http://localhost:8080',
  runtimeUrl: 'http://localhost:8090',
  getToken: () => localStorage.getItem('ironflyer.token'),
});

await ifc.orchestrator.signup({ email: 'me@example.com', password: 'hunter22' });
const projects = await ifc.orchestrator.listProjects();
const ws = await ifc.runtime!.create({ projectId: projects[0]?.id });
const built = await ifc.runtime!.exec(ws.id, { shell: 'go build ./...' });
if (built.exitCode !== 0) console.error(built.stderr);
```

## Layout

| File | Contents |
| --- | --- |
| `types.ts` | Shared interfaces — `Project`, `GateState`, `Plan`, `Workspace`, `ExecResult`, etc. |
| `http.ts` | `Transport` + `IronflyerError`, used by both clients. |
| `orchestrator.ts` | `OrchestratorClient` — projects, gates, budget, brainstorm, streaming chat. |
| `runtime.ts` | `RuntimeClient` — workspaces, files, exec, PTY URL. |
| `index.ts` | `ironflyer({...})` factory + re-exports. |

Streaming chat (`OrchestratorClient.streamChat`) parses Server-Sent Events
from the orchestrator's `POST /projects/{id}/chat` endpoint into typed
`ChatDelta`s — call it with an `AbortSignal` to cancel mid-turn.

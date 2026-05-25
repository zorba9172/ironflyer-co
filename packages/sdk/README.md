# @ironflyer/sdk

Official TypeScript SDK for the **Ironflyer AI Product Finisher**.

Ironflyer takes an idea and ships it through enforced gates — **Spec
→ UX → Architecture → Code → Lint → Tests → Security → Deploy** —
that block until they pass. This SDK is the typed client for the
orchestrator's GraphQL API, including real-time subscriptions over
`graphql-ws`.

```
npm install @ironflyer/sdk graphql graphql-request graphql-ws
```

On Node < 22, also install `ws`:

```
npm install ws
```

## Quick start

```ts
import { Ironflyer } from '@ironflyer/sdk';

const ifr = new Ironflyer({
  endpoint: 'https://api.ironflyer.dev',
});

// 1. Sign in. The returned token is auto-attached to subsequent calls.
const { token, user } = await ifr.signIn({
  email: 'you@example.com',
  password: 'hunter22',
});
console.log('signed in as', user.email);

// 2. List projects.
const projects = await ifr.projects();
console.log(`${projects.length} projects`);

// 3. Propose a patch.
const patch = await ifr.proposePatch({
  projectId: projects[0].id,
  title: 'README polish',
  author: 'demo',
  changes: [
    {
      op: 'CREATE',
      path: 'README.md',
      content: '# Hello from the SDK\n',
    },
  ],
});
console.log('proposed patch', patch.id);

// 4. Stream a finisher run. `for await` is the standard pattern.
for await (const evt of ifr.runProject(projects[0].id)) {
  if (evt.__typename === 'RunDoneEvent') {
    console.log('run finished, ok =', evt.ok);
    break;
  }
  if (evt.__typename === 'RunGateEvent') {
    console.log(`gate ${evt.gate} → ${evt.status}`);
  }
}

// 5. Always dispose to close the WS connection.
ifr.dispose();
```

## Browser vs Node

| Runtime           | Setup                                                                                                        |
| ----------------- | ------------------------------------------------------------------------------------------------------------ |
| Browser           | Nothing extra. Native `WebSocket` + `fetch` are used.                                                        |
| Node 22+          | Nothing extra. Node ships a stable native `WebSocket`.                                                       |
| Node 18 – 21      | Install `ws` and pass it: `new Ironflyer({ endpoint, webSocketImpl: (await import('ws')).WebSocket })`.      |
| Bun / Deno        | Nothing extra.                                                                                               |

## Auth

The SDK is bearer-token authenticated. After `signIn` (or `signUp`),
the returned token is attached to the client automatically. To swap
tokens (refresh flow, impersonation, multi-tenant), call `setToken`:

```ts
ifr.setToken(newToken);
```

Token updates also take effect on the **next reconnect** of the WS
client, because `connectionParams` is a function, not a static object.

To clear the session, call `signOut()` — it clears the token client-side
even if the network call fails.

## Subscriptions

Subscription methods return an `AsyncIterable<T>` typed against the
union returned by the GraphQL schema. You can:

- consume with `for await (const event of ifr.runProject(id)) { … }`,
  and `break` to unsubscribe (the iterator's `return()` cancels the WS
  subscription cleanly),
- or wire it into an `Observable` / `EventEmitter` if your app already
  uses one.

The lifecycle is:

1. The first `next()` call lazily opens the WebSocket (`graphql-ws`
   `lazy: true`).
2. Each `next` GraphQL frame is buffered until the consumer pulls.
3. A `complete` GraphQL message resolves the iterator with `done: true`.
4. A transport error or GraphQL error rejects the next pull with an
   `IronflyerError`.
5. Calling `ifr.dispose()` tears down the underlying WS client — all
   open subscriptions terminate.

```ts
const stream = ifr.runProject(projectId);

// Cancellation via AbortController.
const ac = new AbortController();
ac.signal.addEventListener('abort', () => stream[Symbol.asyncIterator]().return?.());

for await (const evt of stream) {
  // …
}
```

Available subscriptions:

| Method                          | Returns                            |
| ------------------------------- | ---------------------------------- |
| `runProject(id)`                | `RunEvent` union                   |
| `chatStream(projectId, input)`  | `ChatDelta` union                  |
| `inlineCompletion(input)`       | `InlineDelta` union                |
| `deployStream(deployId)`        | `DeployEvent` union                |
| `workspacePty(workspaceId)`     | `PtyEvent` union                   |
| `costStream()`                  | `CostDelta` per call               |

## Error handling

Every method throws `IronflyerError` on failure with the original
error attached as `cause`:

```ts
import { IronflyerError } from '@ironflyer/sdk';

try {
  await ifr.signIn({ email, password });
} catch (err) {
  if (err instanceof IronflyerError) {
    console.error('Ironflyer call failed:', err.message, err.cause);
  } else {
    throw err;
  }
}
```

## Full API surface

The class wraps the public, third-party-safe slice of the orchestrator
schema — auth, projects, gates, patches, budget, memory, audit,
webhooks, deploys, inline completion, chats, workspaces — plus six
streaming subscriptions. See [`CHANGELOG.md`](./CHANGELOG.md) for the
full list as of the current release.

The complete GraphQL schema lives at
[`apps/orchestrator/internal/graph/schema/*.graphql`](https://github.com/ironflyer/ironflyer/tree/main/apps/orchestrator/internal/graph/schema)
and is browsable live at `${endpoint}/graphql/sandbox` (Apollo Sandbox).

## TypeScript

The SDK is ESM-first with a CJS fallback shipped alongside (`exports`
map). All inputs and result shapes are codegen'd from the schema and
re-exported from `@ironflyer/sdk` so you don't need a second package
for types.

```ts
import type { Project, PatchChangeInput, GateStatus } from '@ironflyer/sdk';
```

## License

MIT — see [`LICENSE`](./LICENSE).

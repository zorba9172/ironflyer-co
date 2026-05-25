# Runbook — GraphQL incident

The orchestrator's API of record is GraphQL. When `/graphql` is broken,
every product surface is broken. This runbook is the triage path.

## Surfaces

```
POST /graphql            Queries + mutations (JSON)
GET  /graphql            Persisted-query GETs (APQ)
WS   /graphql            Subscriptions (graphql-transport-ws subprotocol)
GET  /graphql/sandbox    Apollo Sandbox embed (live docs)
```

REST is reserved for k8s probes, Prometheus, the Stripe webhook, the
`/admin/logs/tail` operator stream, and the SSE chat surface. There is
no OpenAPI document.

## 1. Confirm the orchestrator is up

```bash
curl -s -o /dev/null -w '%{http_code}\n' http://localhost:8080/livez
curl -s -o /dev/null -w '%{http_code}\n' http://localhost:8080/readyz
curl -s http://localhost:8080/ | jq .
```

If any of these are non-200, the issue is the orchestrator itself —
not GraphQL. Jump to [`cold-start.md`](cold-start.md) §6.

## 2. GraphQL handshake

```bash
curl -s -X POST http://localhost:8080/graphql \
  -H 'content-type: application/json' \
  -d '{"query":"{ __typename }"}'
```

Expected: `{"data":{"__typename":"Query"}}`.

If the response is `422`, the request body did not parse — usually a
content-type mismatch.

If the response is `400` with `persisted query`-shaped errors, APQ is
locked and your hash isn't registered. Set
`GRAPHQL_APQ_LOCKED=false` (dev) or register the hash (prod).

If the response is `401`, an auth-required middleware was mounted in
front of `/graphql`. Check the boot logs for `auth.Middleware` vs
`auth.Optional` — only `Optional` is correct on the GraphQL chain so
that anonymous `signIn` / `signUp` resolvers work.

## 3. Live sandbox

```bash
curl -s -o /dev/null -w '%{http_code}\n' http://localhost:8080/graphql/sandbox
# expect 200
```

If the sandbox 404s, the orchestrator was built with an older binary
that did not mount the sandbox handler — rebuild from current main.

## 4. Subscriptions

```bash
# Quick handshake using the python ws path the smoke script uses.
python3 -c "
import asyncio, json, websockets
async def main():
    async with websockets.connect('ws://localhost:8080/graphql',
                                  subprotocols=['graphql-transport-ws']) as ws:
        await ws.send(json.dumps({'type':'connection_init','payload':{}}))
        print(await ws.recv())
asyncio.run(main())
"
```

Expected first frame: `{"type":"connection_ack"}`.

If the connection is dropped immediately, an HTTP→WS upgrader is
missing — usually the ingress is the culprit, not the orchestrator.

## 5. Common modes

- **Every mutation returns 422** — APQ middleware running in
  prod-locked mode against dev queries. Toggle
  `GRAPHQL_APQ_LOCKED=false` (dev) or `GRAPHQL_INTROSPECTION=on` while
  introspecting.
- **CSRF rejecting cross-origin POSTs** — only enabled in prod. If
  you're hitting that in dev, the orchestrator was started with
  `IRONFLYER_PROD=true`; unset it.
- **A specific resolver always errors with `internal: ...`** — the
  resolver swallowed a typed error and the masking layer hid the
  detail. Re-run with `GRAPHQL_TRACING=on` and inspect the
  `extensions.traces` field.

## 6. Verification

```bash
bash scripts/v22_smoke.sh   # exercises the GraphQL contract end-to-end.
```

If `v22_smoke.sh` exits 0 (or `PASS-WITH-WARN`), the GraphQL surface
is healthy enough for paid execution.

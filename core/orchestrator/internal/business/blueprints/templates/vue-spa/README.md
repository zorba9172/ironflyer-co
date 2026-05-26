# Vue SPA Blueprint

Vue 3 + Vite + TypeScript + Vue Router + Pinia. A snappy SPA scaffold
for interactive UIs that don't need a Next.js server runtime.

## Quick start

```bash
cp .env.example .env
npm install
npm run dev
```

Open http://localhost:5173 — Home and About routes are wired, and a
Pinia counter store is mounted to demonstrate reactive state.

## Scripts

| Command            | Purpose                              |
|--------------------|--------------------------------------|
| `npm run dev`      | Vite dev server with HMR             |
| `npm run build`    | Typecheck (`vue-tsc`) + production build |
| `npm run preview`  | Preview the built bundle             |
| `npm run typecheck`| Run `vue-tsc --noEmit` only          |

## Adding routes

Create a component under `src/views/` and register it in
`src/router.ts`. Store modules live under `src/stores/` — follow the
`counter.ts` pattern (composition API + `defineStore`).

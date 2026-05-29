# Ironflyer IDE (`@ironflyer/ide`)

A custom-branded [Eclipse Theia](https://theia-ide.org/) **browser**
application. It is served per-workspace and embedded as an `<iframe>` inside
the Ironflyer studio. Theia's own branding and clutter are stripped; the shell
is fully Ironflyer-branded ("international" cobalt -> cyan identity).

- **Theia version:** `^1.60.0` (pinned across all `@theia/*` packages and
  `@theia/cli`).
- **Default port:** `3030`, bound to `0.0.0.0`, served at `/`.
- **Default workspace folder:** `/home/coder` (the per-workspace volume mount).
- **Product name shown in the IDE:** `Ironflyer IDE`.

## Layout

```
clients/ide/
├── package.json              Theia browser app (lean extension set + theia config)
├── Dockerfile                multi-stage build -> ironflyer/theia-ide:latest
├── .dockerignore
├── README.md                 (this file)
└── ironflyer-branding/       local extension: @ironflyer/theia-branding
    ├── package.json          theiaExtensions + contributes.themes
    ├── tsconfig.json
    ├── src/browser/
    │   ├── ironflyer-frontend-module.ts      Inversify module (default export)
    │   └── ironflyer-branding-contribution.ts FrontendApplicationContribution
    ├── style/branding.css    UI font (Inter) + chrome de-clutter + wordmark
    └── themes/
        ├── ironflyer-dark-color-theme.json
        └── ironflyer-light-color-theme.json
```

### Included Theia extensions (intentionally lean)

`@theia/core`, `editor`, `monaco`, `filesystem`, `workspace`, `navigator`,
`terminal`, `search-in-workspace`, `preferences`, `file-search`, `messages`,
`markers`, `outline-view` — plus the local `@ironflyer/theia-branding`.

Deliberately **excluded** (clutter / heavy): getting-started, git, scm, debug,
plugin-ext / vsx-registry, keymaps editor, mini-browser, console, task.

## Branding

- **Single source of truth:** the brand CSS variables
  (`ironflyer-branding/style/brand-tokens.generated.css`) and **both color
  themes** (`themes/*.json`) are **code-generated** from
  `packages/design-tokens/brand.ts` — the exact same tokens the studio's MUI
  theme consumes — by `ironflyer-branding/scripts/generate-brand-theme.mjs`.
  Change a color/font in `brand.ts`, rebuild, and the whole IDE (chrome +
  editor + terminal + both themes) retints automatically. Never hand-edit the
  generated files. The generator runs as part of the package `build` (or
  `npm run gen`); in the standalone Docker build `brand.ts` is out of context,
  so it no-ops and the committed generated files are used as-is.
- **Color themes:** contributed via the standard VS Code / Theia
  `contributes.themes` mechanism in `ironflyer-branding/package.json` pointing
  at the JSON theme files in `themes/` (the cleanest idiomatic approach for
  Theia 1.60 — no programmatic `ColorContribution` needed for full themes).
  Two full, brand-faithful themes ship at parity — **Ironflyer Dark** and
  **Ironflyer Light** — both mapped to the cobalt -> cyan brand tokens
  (surfaces, borders, text, semantic accents). The default theme is
  **Ironflyer Dark** (set via the `workbench.colorTheme` preference in
  `package.json` -> `theia.frontend`).
- **Fonts:** UI = Inter (loaded via CSS), code/mono = Geist Mono (set via the
  `editor.fontFamily` preference). See the `TODO(fonts)` note in
  `ironflyer-branding/style/branding.css` for self-hosting the font files
  for offline/air-gapped workspaces.
- **Shell cleanup:** the `FrontendApplicationContribution` forces
  `document.title = "Ironflyer IDE"`, scrubs Theia attribution from the About
  dialog, closes any Getting Started tab, and injects a lightweight Ironflyer
  wordmark. The default menu bar is hidden via CSS for the embedded look.

## Local development (host monorepo)

From the monorepo root. Do **not** run installs in constrained CI sandboxes;
the Theia build is heavy.

```bash
pnpm install
pnpm --filter @ironflyer/theia-branding build   # compile the branding extension
pnpm --filter @ironflyer/ide build              # theia build --mode production
pnpm --filter @ironflyer/ide start              # theia start on :3030
```

Then open <http://localhost:3030>. The branding extension is wired as a
`workspace:*` dependency, so pnpm links it automatically.

## Docker

The Docker build context is this directory (`clients/ide`); inside the image
the branding extension is linked via `file:` instead of pnpm workspaces.

```bash
# from clients/ide/
docker build -t ironflyer/theia-ide:latest .

docker run --rm -p 3030:3030 \
  -v "$PWD/some-project:/home/coder" \
  ironflyer/theia-ide:latest
```

Open <http://localhost:3030>. The container opens `/home/coder` as the
workspace folder; mount the per-workspace volume there.

## Security / auth

Theia's browser app has **no built-in authentication** (`--auth none`
semantics). Access to this container MUST be gated by the runtime service
(network policy, reverse-proxy auth, or a per-workspace token) before it is
exposed. The studio iframes whatever URL points at this container.

## Integration contract

- Listens on port **3030**, serves the Theia browser app at `/`.
- Opens the folder **`/home/coder`** (the per-workspace volume mount).
- The studio embeds the container URL in an `<iframe>`.

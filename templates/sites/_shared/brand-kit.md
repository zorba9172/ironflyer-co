# Ironflyer brand kit (injected into every template)

Every template in `templates/sites/<cat>/<slug>/index.html` gets two
identical, idempotent blocks injected by
`scripts/templates/upgrade-with-brand.mjs`:

1. **Scroll-reveal CSS** — placed just before `</head>`. Hides
   common section / card-like elements (`section`, `.feature`, `.plan`,
   `.card`, `.product`, `.listing`, `.item`, `.testimonial`, `.case-study`,
   `.post`) at opacity 0; a CSS animation reveals them as they scroll
   into view. Respects `prefers-reduced-motion`. Has a `<noscript>`
   escape hatch so JS-disabled users always see content.

2. **"Built with Ironflyer" signature + IntersectionObserver bootstrap**
   — placed just before `</body>`. Floating pill in the bottom-right that
   links to ironflyer.dev; tiny JS that wires up the scroll-reveal
   observer.

Both blocks are wrapped in `data-if-brand` attributes so the upgrade
script can find and replace them without duplicating. The injection is
**idempotent**: running the upgrade twice produces the same file.

## Why inject instead of `<link>`?

Templates stay self-contained — copying the file or hotlinking it from
`clients/web/public/template-previews/` doesn't require any sibling assets.
The kit is small (~2 KB inline) and the cost of duplication across 150
files is negligible compared to the gain of fully portable templates.

## Customising the kit

Edit `scripts/templates/upgrade-with-brand.mjs` (the `BRAND_HEAD` and
`BRAND_FOOT` constants), then run `node
scripts/templates/upgrade-with-brand.mjs` to re-stamp every template.
Re-run `screenshot.mjs` or `publish-previews.mjs` to publish the
updated HTML.

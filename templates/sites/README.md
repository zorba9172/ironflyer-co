# Ironflyer Site Templates

A curated collection of 100 original, world-class site templates the
Ironflyer template gallery offers to users. These are first-class assets:
they ship with the product, are referenced from
`clients/web/app/app/resources/page.tsx`, and are pulled into a project as
visual + structural references when a user picks one.

These are **original** templates created for Ironflyer. They are not
purchased themes. Imagery is loaded from Unsplash (free, royalty-free) by
direct URL so the repository stays free of third-party binary assets.

## Layout

```
templates/sites/
├── README.md            ← this file
├── manifest.json        ← generated index — every template listed once
├── saas/                ← developer + business app landings (10)
├── agency/              ← agency & studio sites (10)
├── portfolio/           ← personal portfolios (10)
├── ecommerce/           ← product stores (10)
├── restaurant/          ← food & beverage (10)
├── realestate/          ← real estate, architecture, spaces (10)
├── fitness/             ← health, fitness, wellness (10)
├── education/           ← courses, schools, edtech (10)
├── blog/                ← blogs & magazines (10)
└── mobile-app/          ← mobile / PWA app landings (10)
```

Each template lives in its own directory under its category:

```
templates/sites/<category>/<NN-slug>/
├── index.html           ← single self-contained file (HTML + inline <style>)
└── template.json        ← metadata read by the gallery + manifest builder
```

## template.json schema

```json
{
  "slug": "saas-01-codeforge",
  "title": "CodeForge",
  "subtitle": "The AI dev platform that ships production code.",
  "category": "SaaS",
  "type": "Apps",
  "tags": ["dark", "developer", "minimal"],
  "previewImage": "https://images.unsplash.com/photo-...",
  "palette": { "bg": "#0a0a0a", "fg": "#f7f3ea", "accent": "#e5ff00" },
  "sections": ["nav", "hero", "logos", "features", "code", "pricing", "cta", "footer"],
  "stack": "HTML + inline CSS",
  "prompt": "Use the CodeForge template as the foundation for a developer-focused SaaS landing with hero, logo strip, feature grid, code demo, pricing, and a tight footer."
}
```

| field          | rule                                                                                  |
|----------------|---------------------------------------------------------------------------------------|
| `slug`         | `<category>-<NN>-<short-name>` — globally unique, kebab-case                          |
| `title`        | Brand name (1–3 words)                                                                |
| `subtitle`     | One sentence positioning, sentence case                                               |
| `category`     | Human label: SaaS / Agency / Portfolio / Commerce / Restaurant / Real Estate / Fitness / Education / Blog / Mobile App |
| `type`         | One of the gallery filter chips: `Apps` / `Websites` / `Commerce` / `Mobile/PWA`      |
| `tags`         | 2–5 lowercase descriptors (style, audience, vibe)                                     |
| `previewImage` | Absolute Unsplash URL, `?w=1200&q=70&auto=format` recommended                         |
| `palette`      | `bg`, `fg`, `accent` hex triplet that matches the template's CSS                      |
| `sections`     | Ordered list of section ids that appear on the page                                   |
| `stack`        | Always `HTML + inline CSS` today                                                      |
| `prompt`       | One-paragraph user prompt seeded into the workspace when "Use template" is clicked    |

## index.html quality bar

- **Self-contained.** One file, with `<style>` block inline. No external
  CSS frameworks, no build step. Optional `<script>` only for tiny
  enhancements (mobile nav, tab switch); never required for the page to
  read.
- **Real copy.** Sentence-case, English, product-specific, no
  "Lorem ipsum" and no fake testimonials with quoted CEOs of made-up
  companies. Generic but believable customer names ("Acme", "Northbeam",
  "Field & Format") are fine.
- **Responsive.** Two breakpoints: ≤720px (mobile) and ≤1080px (tablet).
  Use CSS grid / flexbox; avoid pixel-perfect breakage.
- **Accessible.** Real heading order (`h1` once, descending), `alt` text
  on images, sufficient color contrast (4.5:1 for body text).
- **Image hygiene.** Hero + section images load from `images.unsplash.com`
  with explicit `width`, `height`, `loading="lazy"`, and meaningful alt
  text. No third-party scripts, trackers, or fonts beyond Google Fonts
  (preferred: Inter, JetBrains Mono, or a category-appropriate display
  face).
- **Section count.** Minimum 6 sections, typical 7–9: nav, hero, value
  props / features, social proof or stats, secondary content
  (testimonials / portfolio / menu), pricing / CTA, footer.
- **Length.** Typically 300–700 lines of HTML+CSS. Beautiful and tight
  beats sprawling.

## How the gallery consumes these

`clients/web/app/app/resources/page.tsx` reads `manifest.json` (built by
`scripts/templates/build-manifest.mjs`) to populate the template gallery.
The user picks a card, the page seeds a prompt into the workspace
referencing `templates/sites/<category>/<NN-slug>/`, and the orchestrator
uses the template as the visual + structural foundation for the build.

## Adding or editing a template

1. Create / edit `templates/sites/<category>/<NN-slug>/index.html`.
2. Create / edit the matching `template.json`.
3. Run `node scripts/templates/build-manifest.mjs` to regenerate the
   manifest. CI does this in `npm run build`.

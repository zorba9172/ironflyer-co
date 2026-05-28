# Brand & Design System — International Brand (2026-05-28)

Grounding research + the locked brand for the new `clients/` surfaces. This is
a **deliberate break** from the retiring `clients/web` identity (deep-space
violet + coral→magenta CTA + banned-lime). The new brand is built for a serious
international product, not a one-off SaaS skin.

> Token discipline is still constitutional: nothing below is hardcoded in app
> code. Every value lives in `@ironflyer/design-tokens` (the new `brand` export)
> and reaches components only through the MUI theme (web) or RN theme (native).

---

## 1. Reference research (what we learned, what we take)

| Reference | What it does well | What we take | What we deliberately do NOT copy |
| --- | --- | --- | --- |
| **sonichaos.io** | Section flow: bold problem-headline hero → 3 CTA cards → metric-anchored proof → product tiers → genre/collection grids → pricing → vault. Industry vernacular, specificity over hype ("-14 LUFS", "Four rules. No fine print"). Loose vertical rhythm. | The **marketing page architecture** (hero → 3 entry cards → proof-by-numbers → capability blocks → pricing → final bundle CTA) and the **specific-over-hyped copy voice**. | Their music-catalog content; their exact card layouts. |
| **artlist.io** | Cinematic premium creative-tool feel; oversized type; confident minimal nav; asset-grid density. | Editorial confidence, oversized display type, generous whitespace, motion on scroll. | Video-wall hero (we're a builder, not a stock library). |
| **lovable.dev** | AI app-builder: prompt-box-as-hero, project card grid, friendly-but-pro tone, clean theming with dark toggle. | **Prompt-first dashboard pattern** for studio; multi-format logo system; light+dark parity. | Playful gradients / "friendly dev" softness — we go more instrument-grade. |
| **app.base44.com** | Global Theme panel (palette/typography/spacing update the whole build); split-layout auth. | Token-driven theming as a first-class idea; split auth shell (already locked in our login spec). | — |
| **output.com** | Bold creative confidence, strong type, restrained dark palette with one hot accent. | One confident accent on a disciplined neutral base; bold display type. | Music-product chrome. |

Common thread across all five: **disciplined neutral base + one confident
accent + oversized grotesk display + metric-anchored, human copy.** That is the
spine of our system.

---

## 2. Differentiation thesis

The old identity reads "violet AI SaaS template." Everyone in this space is
violet/purple-gradient right now (Lovable, base44, countless others). To look
**international and serious**, we move to:

- **A true-neutral ink base** (not blue-black "space"), so the product feels
  like an instrument, not a theme.
- **A single signature accent: electric cobalt → cyan**, cool and precise —
  reads "engineering / signal", and is unmistakably *not* the purple crowd.
- **One warm signal (amber)** reserved for emphasis and value moments.
- **A grotesk display face** (Bricolage Grotesque) for editorial weight —
  distinct from the Inter-everywhere look.

---

## 3. Color (locked)

Signature gradient — **Cobalt → Cyan** — is the brand's single recognizable mark.

| Role | Dark | Light |
| --- | --- | --- |
| Base bg | `#0A0B0D` ink | `#FAF9F6` paper |
| Surface | `#111317` | `#FFFFFF` |
| Surface raised | `#171A1F` | `#F3F1EC` |
| Border subtle | `rgba(255,255,255,0.08)` | `rgba(10,11,13,0.08)` |
| Text primary | `#F4F5F7` | `#0A0B0D` |
| Text secondary | `#A4A9B3` | `#4A4F58` |
| Text muted | `#6B7280` | `#787E88` |

| Accent | Hex | Use |
| --- | --- | --- |
| Cobalt (primary) | `#2F6BFF` | primary actions, links, focus |
| Cyan (secondary) | `#18C8E6` | gradient end, live/active |
| Amber (signal) | `#FFB020` | value moments, highlights |
| Emerald (success) | `#16B981` | success/live-ok |
| Rose (danger) | `#F43F5E` | errors/destructive |

Primary CTA = `linear-gradient(100deg, #2F6BFF, #18C8E6)`. No purple, no coral.

---

## 4. Typography (locked)

| Role | Family | Notes |
| --- | --- | --- |
| Display / headings | **Bricolage Grotesque** | editorial grotesk; oversized hero type, tight tracking |
| Body / UI | **Inter** | already in the stack; weights 400–600 |
| Mono / metrics / code | **Geist Mono** | already in the stack; numbers, specs, code |

Type scale (rem): display 4.5 / 3.375 / 2.5 · h1 2 · h2 1.5 · h3 1.25 · body 1 ·
small 0.875 · mono-label 0.75. Line-height tight on display (1.05), 1.5 on body.

---

## 5. Dark / Light (locked behavior + timing)

Both themes are first-class (not dark-only). Resolution order:

1. Explicit user choice in `localStorage` (`if-theme`), else
2. `prefers-color-scheme`, with `<meta name="color-scheme">` set so the browser
   paints the right canvas before hydration (no white flash).

**Timing:** theme transitions animate `background-color`, `color`, and
`border-color` over **180ms `ease-out`** (the `--if-theme-transition` token).
The `<html>` element gets a `.theme-animating` class only during the swap so
first paint and route changes are instant; the transition applies only to the
deliberate toggle. Native (Expo) follows the OS `Appearance` with the same token
values and a 180ms `Animated` timing on themed surfaces.

---

## 6. Copywriting voice (locked)

Write like a senior product person who respects the reader. Never like an AI.

- **Specific over hyped.** Numbers, named capabilities, concrete outcomes — the
  sonichaos "-14 LUFS / Four rules. No fine print" discipline.
- **Second person, present tense.** "You ship the part that's actually done."
- **Short declaratives.** No "unlock", "elevate", "seamless", "revolutionize",
  "in today's fast-paced world", em-dash padding, or tricolon filler.
- **Confidence without superlatives.** State the mechanism, not the adjective.
- **Same register across surfaces** — marketing, product empty-states, and docs
  share one voice.

A copy lint list of banned AI-tells lives in `packages/core` (`bannedPhrases`)
so empty-states and marketing can be checked against it.

---

## 7. How this maps to code

- New brand → `@ironflyer/design-tokens` `./brand` export (legacy `.` export
  stays for the retiring web app until it's deleted).
- Web consumes via a generated MUI theme (`@ironflyer/ui-web` `makeTheme`).
- Native consumes via an RN theme provider (`@ironflyer/ui-native`).
- Marketing (Astro) consumes tokens as CSS custom properties emitted at build.

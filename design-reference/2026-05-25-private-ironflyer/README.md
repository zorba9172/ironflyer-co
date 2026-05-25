# 2026-05-25 Private IronFlyer Reference

Status: locked.

This folder records the private IronFlyer visual reference supplied on 2026-05-25. The supplied screenshots are private product design material and are treated as the source of truth for the web product, Studio, login/signup, cloud IDE styling, and every internal product page.

## Assets

- `references/home-desktop-reference.png` - current local dark Home capture aligned to the private landing direction.
- `references/home-mobile-current-gap.png` - current local mobile Home capture for overflow and density comparison.
- `references/STUDIO_VSCODE_CLOUD_TARGET.md` - locked Studio target extracted from the supplied private screenshot.
- `references/studio-vscode-cloud-target.html` - local static reference board for the supplied Studio design.
- `references/studio-entry-current-gap.png` - current local Studio implementation capture; this is not the target.
- `references/studio-mobile-current-gap.png` - current local mobile Studio implementation capture; this is not the target.
- `references/login-desktop-current-gap.png` - current local login capture for the Base44-style split-product auth direction.
- `derived-local-baseline/` - older local handoff captures kept for comparison only.

## Required Target

- Home follows the private dark landing reference: left hero copy, right interactive product builder, logo row, workflow, feature grid, templates, testimonial, pricing, FAQ, bottom CTA and footer.
- Studio follows `references/STUDIO_VSCODE_CLOUD_TARGET.md`: global nav, left rail, breadcrumb/action bar, mode/status row, prompt/code/preview columns, assistant strip, and VS Code cloud behavior.
- Product visuals are interactive UI, not flattened screenshots.
- No route may create page-level horizontal overflow.

The actual in-chat reference images remain the deciding source. This folder makes the local repo point to one stable reference location; it does not authorize drifting toward older light screenshots or generated approximations.

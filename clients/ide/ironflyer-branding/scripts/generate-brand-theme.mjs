// Generate the Theia IDE brand surfaces from the SINGLE source of truth:
// packages/design-tokens/brand.ts (the same tokens the MUI theme consumes).
//
// Emits three committed artifacts so the Theia app — which builds in a Docker
// context that does NOT contain the monorepo — can consume them directly:
//   - style/brand-tokens.generated.css                  (:root CSS variables)
//   - themes/ironflyer-dark-color-theme.json            (VS Code color theme)
//   - themes/ironflyer-light-color-theme.json           (VS Code color theme)
//
// Run automatically by the package `build` script (host monorepo). When
// brand.ts is not reachable (e.g. the standalone Docker build whose context is
// only clients/ide), this script no-ops and the committed artifacts are used
// as-is. Change a brand value in brand.ts, rebuild, and the whole IDE retints.

import { existsSync, mkdtempSync, writeFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
// scripts -> ironflyer-branding -> ide -> clients -> ironflyer (repo root)
const repoRoot = resolve(here, '../../../..');
const brandEntry = resolve(repoRoot, 'packages/design-tokens/brand.ts');
const brandingDir = resolve(here, '..');

if (!existsSync(brandEntry)) {
  console.warn(
    '[brand-theme] brand.ts not reachable (standalone/Docker build) — '
    + 'keeping committed generated artifacts.',
  );
  process.exit(0);
}

// esbuild is only needed on the host path (brand.ts present); import lazily so
// the Docker no-op above never requires it to be installed.
const { build } = await import('esbuild');

// ── load brand tokens ───────────────────────────────────────────────────────
const tmp = mkdtempSync(join(tmpdir(), 'if-brand-'));
const out = join(tmp, 'brand.mjs');
await build({
  entryPoints: [brandEntry],
  outfile: out,
  bundle: true,
  format: 'esm',
  platform: 'node',
  logLevel: 'silent',
});
const brand = await import(pathToFileURL(out).href);
rmSync(tmp, { recursive: true, force: true });

const { palette, modes, accent, gradient, typography } = brand;

// ── color helpers ───────────────────────────────────────────────────────────
const clamp = (n) => Math.max(0, Math.min(255, Math.round(n)));
const toByte = (n) => clamp(n).toString(16).padStart(2, '0').toUpperCase();

function parseHex(hex) {
  const h = hex.replace('#', '');
  return { r: parseInt(h.slice(0, 2), 16), g: parseInt(h.slice(2, 4), 16), b: parseInt(h.slice(4, 6), 16) };
}
function hex(r, g, b) { return `#${toByte(r)}${toByte(g)}${toByte(b)}`; }
// #RRGGBB + alpha (0..1) -> #RRGGBBAA
function alpha(c, a) { const { r, g, b } = parseHex(c); return `${hex(r, g, b)}${toByte(a * 255)}`; }
// blend t (0..1) of `b` into `a`
function mix(a, b, t) {
  const A = parseHex(a); const B = parseHex(b);
  return hex(A.r + (B.r - A.r) * t, A.g + (B.g - A.g) * t, A.b + (B.b - A.b) * t);
}
// brand modes carry borders/overlay as rgba() strings -> convert to #RRGGBBAA
function rgbaToHex8(rgba) {
  const m = rgba.match(/rgba?\(([^)]+)\)/i);
  if (!m) return rgba;
  const [r, g, b, a = '1'] = m[1].split(',').map((s) => s.trim());
  return `${hex(+r, +g, +b)}${toByte(parseFloat(a) * 255)}`;
}

const WHITE = '#FFFFFF';
const ink = palette.ink;
const tint = (c, t) => mix(c, WHITE, t);
const shade = (c, t) => mix(c, ink, t);

// ── per-mode semantic roles (everything derives from brand tokens) ──────────
function roles(mode) {
  const dark = mode === 'dark';
  const m = modes[mode];
  const { cobalt, cobaltDeep, cyan, amber, emerald, rose } = palette;
  return {
    dark,
    // surfaces
    bg: m.bg,
    bgSubtle: m.bgSubtle,
    surface: dark ? m.surface : m.bgSubtle,
    raised: dark ? m.surfaceRaised : m.surface,
    hover: dark ? m.surfaceRaised : shade(m.bgSubtle, 0.045),
    hoverStrong: dark ? m.surfaceHover : shade(m.bgSubtle, 0.08),
    editorBg: m.bg,
    inputBg: dark ? m.bg : m.surface,
    // borders (translucent, straight from brand modes)
    borderSubtle: rgbaToHex8(m.borderSubtle),
    borderStrong: rgbaToHex8(m.borderStrong),
    // text
    textPrimary: m.textPrimary,
    textSecondary: m.textSecondary,
    textMuted: m.textMuted,
    onAccent: dark ? m.textPrimary : WHITE,
    onBadge: ink,
    // brand accents
    cobalt,
    focus: cobalt,
    link: dark ? cyan : cobaltDeep,
    linkActive: dark ? tint(cyan, 0.25) : cobalt,
    cursor: dark ? cyan : cobaltDeep,
    badge: cyan,
    buttonBg: cobalt,
    buttonHover: dark ? tint(cobalt, 0.18) : cobaltDeep,
    progress: cobalt,
    // selection / highlight alphas
    sel: alpha(cobalt, dark ? 0.33 : 0.2),
    selHi: alpha(cobalt, dark ? 0.2 : 0.1),
    selInactive: alpha(cobalt, dark ? 0.13 : 0.08),
    listActive: alpha(cobalt, dark ? 0.2 : 0.15),
    bracket: alpha(cobalt, dark ? 0.2 : 0.1),
    findMatch: alpha(amber, dark ? 0.33 : 0.47),
    findMatchHi: alpha(amber, dark ? 0.2 : 0.27),
    lineHighlight: alpha(dark ? m.surface : ink, dark ? 0.5 : 0.04),
    // semantic signals (darkened on light for contrast on paper)
    warn: dark ? amber : shade(amber, 0.3),
    warnBright: dark ? tint(amber, 0.3) : amber,
    success: dark ? emerald : shade(emerald, 0.18),
    successBright: dark ? tint(emerald, 0.3) : emerald,
    error: dark ? rose : shade(rose, 0.2),
    errorBright: dark ? tint(rose, 0.3) : rose,
    // validation fills
    errBg: dark ? mix(rose, m.bg, 0.82) : mix(rose, WHITE, 0.9),
    warnBg: dark ? mix(amber, m.bg, 0.85) : mix(amber, WHITE, 0.88),
    infoBg: dark ? mix(cobalt, m.bg, 0.85) : mix(cobalt, WHITE, 0.9),
    // editor furniture
    lineNumber: alpha(m.textMuted, 0.7),
    lineNumberActive: m.textSecondary,
    whitespace: alpha(m.textMuted, 0.35),
    sliderBg: alpha(m.textMuted, dark ? 0.4 : 0.33),
    sliderHover: alpha(m.textSecondary, dark ? 0.53 : 0.47),
    sliderActive: alpha(m.textPrimary, dark ? 0.66 : 0.6),
    shadow: dark ? '#00000066' : '#00000022',
    transparent: '#00000000',
    // syntax
    synComment: m.textMuted,
    synString: dark ? tint(emerald, 0.3) : shade(emerald, 0.18),
    synNumber: dark ? amber : shade(amber, 0.3),
    synKeyword: dark ? tint(cobalt, 0.22) : cobaltDeep,
    synFunction: dark ? cyan : shade(cyan, 0.45),
    synType: dark ? tint(cyan, 0.3) : shade(cyan, 0.55),
    synVariable: m.textPrimary,
    synParam: m.textSecondary,
    synTag: cobalt,
    synAttr: dark ? amber : shade(amber, 0.3),
    synInvalid: dark ? rose : shade(rose, 0.2),
    // terminal ansi (magenta has no brand token; terminals require one)
    ansiBlack: dark ? m.surface : ink,
    ansiBrightBlack: m.textMuted,
    ansiBlue: dark ? cobalt : cobaltDeep,
    ansiBrightBlue: dark ? tint(cobalt, 0.2) : cobalt,
    ansiCyan: dark ? cyan : shade(cyan, 0.45),
    ansiBrightCyan: dark ? tint(cyan, 0.25) : cyan,
    ansiGreen: dark ? emerald : shade(emerald, 0.18),
    ansiBrightGreen: dark ? tint(emerald, 0.3) : emerald,
    ansiYellow: dark ? amber : shade(amber, 0.3),
    ansiBrightYellow: dark ? tint(amber, 0.3) : amber,
    ansiRed: dark ? rose : shade(rose, 0.2),
    ansiBrightRed: dark ? tint(rose, 0.3) : rose,
    ansiMagenta: '#A855F7',
    ansiBrightMagenta: '#C084FC',
    ansiWhite: m.textSecondary,
    ansiBrightWhite: m.textPrimary,
  };
}

// ── VS Code color map ───────────────────────────────────────────────────────
function colors(r) {
  return {
    focusBorder: r.focus,
    foreground: r.textPrimary,
    disabledForeground: r.textMuted,
    descriptionForeground: r.textSecondary,
    errorForeground: r.error,
    'icon.foreground': r.textSecondary,
    'selection.background': r.sel,

    'editor.background': r.editorBg,
    'editor.foreground': r.textPrimary,
    'editorLineNumber.foreground': r.lineNumber,
    'editorLineNumber.activeForeground': r.lineNumberActive,
    'editorCursor.foreground': r.cursor,
    'editor.selectionBackground': r.sel,
    'editor.selectionHighlightBackground': r.selHi,
    'editor.inactiveSelectionBackground': r.selInactive,
    'editor.wordHighlightBackground': alpha(r.link, 0.13),
    'editor.wordHighlightStrongBackground': alpha(r.link, 0.2),
    'editor.findMatchBackground': r.findMatch,
    'editor.findMatchHighlightBackground': r.findMatchHi,
    'editor.lineHighlightBackground': r.lineHighlight,
    'editor.lineHighlightBorder': r.transparent,
    'editorWhitespace.foreground': r.whitespace,
    'editorIndentGuide.background1': r.borderSubtle,
    'editorIndentGuide.activeBackground1': r.borderStrong,
    'editorRuler.foreground': r.borderSubtle,
    'editorBracketMatch.background': r.bracket,
    'editorBracketMatch.border': r.cobalt,
    'editorError.foreground': r.error,
    'editorWarning.foreground': r.warn,
    'editorInfo.foreground': r.link,
    'editorHint.foreground': r.success,
    'editorGutter.modifiedBackground': r.cobalt,
    'editorGutter.addedBackground': r.successBright,
    'editorGutter.deletedBackground': r.errorBright,
    'editorOverviewRuler.border': r.transparent,
    'editorOverviewRuler.findMatchForeground': alpha(r.warnBright, 0.6),
    'editorOverviewRuler.errorForeground': r.error,
    'editorOverviewRuler.warningForeground': r.warn,
    'editorOverviewRuler.infoForeground': r.link,

    'editorWidget.background': r.raised,
    'editorWidget.border': r.borderStrong,
    'editorSuggestWidget.background': r.raised,
    'editorSuggestWidget.border': r.borderStrong,
    'editorSuggestWidget.foreground': r.textPrimary,
    'editorSuggestWidget.selectedBackground': r.selHi,
    'editorSuggestWidget.highlightForeground': r.link,
    'editorHoverWidget.background': r.raised,
    'editorHoverWidget.border': r.borderStrong,

    'editorGroupHeader.tabsBackground': r.surface,
    'editorGroupHeader.noTabsBackground': r.surface,
    'editorGroup.border': r.borderSubtle,
    'tab.activeBackground': r.editorBg,
    'tab.inactiveBackground': r.surface,
    'tab.activeForeground': r.textPrimary,
    'tab.inactiveForeground': r.textSecondary,
    'tab.border': r.borderSubtle,
    'tab.activeBorderTop': r.cobalt,
    'tab.activeBorder': r.cobalt,
    'tab.hoverBackground': r.hover,
    'tab.unfocusedActiveBorderTop': r.borderStrong,

    'titleBar.activeBackground': r.surface,
    'titleBar.activeForeground': r.textPrimary,
    'titleBar.inactiveBackground': r.surface,
    'titleBar.inactiveForeground': r.textMuted,
    'titleBar.border': r.borderSubtle,

    'activityBar.background': r.surface,
    'activityBar.foreground': r.textPrimary,
    'activityBar.inactiveForeground': r.textMuted,
    'activityBar.border': r.borderSubtle,
    'activityBar.activeBorder': r.cobalt,
    'activityBar.activeBackground': r.hover,
    'activityBarBadge.background': r.badge,
    'activityBarBadge.foreground': r.onBadge,

    'sideBar.background': r.surface,
    'sideBar.foreground': r.textPrimary,
    'sideBar.border': r.borderSubtle,
    'sideBarTitle.foreground': r.textSecondary,
    'sideBarSectionHeader.background': r.hover,
    'sideBarSectionHeader.foreground': r.textPrimary,
    'sideBarSectionHeader.border': r.borderSubtle,

    'list.activeSelectionBackground': r.listActive,
    'list.activeSelectionForeground': r.textPrimary,
    'list.inactiveSelectionBackground': r.hover,
    'list.hoverBackground': r.hover,
    'list.hoverForeground': r.textPrimary,
    'list.focusBackground': r.listActive,
    'list.focusForeground': r.textPrimary,
    'list.highlightForeground': r.link,
    'list.errorForeground': r.error,
    'list.warningForeground': r.warn,
    'tree.indentGuidesStroke': r.borderStrong,

    'statusBar.background': r.bg,
    'statusBar.foreground': r.textSecondary,
    'statusBar.border': r.borderSubtle,
    'statusBar.noFolderBackground': r.bg,
    'statusBar.debuggingBackground': r.cobalt,
    'statusBar.debuggingForeground': r.onAccent,
    'statusBarItem.remoteBackground': r.cobalt,
    'statusBarItem.remoteForeground': r.onAccent,
    'statusBarItem.hoverBackground': r.hover,
    'statusBarItem.prominentBackground': r.hover,
    'statusBarItem.errorBackground': r.errorBright,
    'statusBarItem.errorForeground': r.onAccent,
    'statusBarItem.warningBackground': r.warnBright,
    'statusBarItem.warningForeground': r.onBadge,

    'panel.background': r.surface,
    'panel.border': r.borderSubtle,
    'panelTitle.activeForeground': r.textPrimary,
    'panelTitle.inactiveForeground': r.textMuted,
    'panelTitle.activeBorder': r.cobalt,
    'panelSection.border': r.borderSubtle,
    'panelSectionHeader.background': r.hover,

    'terminal.background': r.editorBg,
    'terminal.foreground': r.textPrimary,
    'terminal.border': r.borderSubtle,
    'terminalCursor.foreground': r.cursor,
    'terminal.selectionBackground': alpha(r.cobalt, 0.27),
    'terminal.ansiBlack': r.ansiBlack,
    'terminal.ansiBrightBlack': r.ansiBrightBlack,
    'terminal.ansiBlue': r.ansiBlue,
    'terminal.ansiBrightBlue': r.ansiBrightBlue,
    'terminal.ansiCyan': r.ansiCyan,
    'terminal.ansiBrightCyan': r.ansiBrightCyan,
    'terminal.ansiGreen': r.ansiGreen,
    'terminal.ansiBrightGreen': r.ansiBrightGreen,
    'terminal.ansiYellow': r.ansiYellow,
    'terminal.ansiBrightYellow': r.ansiBrightYellow,
    'terminal.ansiRed': r.ansiRed,
    'terminal.ansiBrightRed': r.ansiBrightRed,
    'terminal.ansiMagenta': r.ansiMagenta,
    'terminal.ansiBrightMagenta': r.ansiBrightMagenta,
    'terminal.ansiWhite': r.ansiWhite,
    'terminal.ansiBrightWhite': r.ansiBrightWhite,

    'input.background': r.inputBg,
    'input.foreground': r.textPrimary,
    'input.border': r.borderStrong,
    'input.placeholderForeground': r.textMuted,
    'inputOption.activeBorder': r.cobalt,
    'inputOption.activeBackground': r.selHi,
    'inputOption.activeForeground': r.textPrimary,
    'inputValidation.errorBackground': r.errBg,
    'inputValidation.errorBorder': r.errorBright,
    'inputValidation.warningBackground': r.warnBg,
    'inputValidation.warningBorder': r.warnBright,
    'inputValidation.infoBackground': r.infoBg,
    'inputValidation.infoBorder': r.cobalt,

    'dropdown.background': r.inputBg,
    'dropdown.foreground': r.textPrimary,
    'dropdown.border': r.borderStrong,
    'dropdown.listBackground': r.raised,

    'button.background': r.buttonBg,
    'button.foreground': r.onAccent,
    'button.hoverBackground': r.buttonHover,
    'button.secondaryBackground': r.hover,
    'button.secondaryForeground': r.textPrimary,
    'button.secondaryHoverBackground': r.hoverStrong,
    'checkbox.background': r.inputBg,
    'checkbox.foreground': r.textPrimary,
    'checkbox.border': r.borderStrong,

    'badge.background': r.badge,
    'badge.foreground': r.onBadge,

    'progressBar.background': r.progress,

    'scrollbar.shadow': r.transparent,
    'scrollbarSlider.background': r.sliderBg,
    'scrollbarSlider.hoverBackground': r.sliderHover,
    'scrollbarSlider.activeBackground': r.sliderActive,

    'textLink.foreground': r.link,
    'textLink.activeForeground': r.linkActive,
    'textBlockQuote.background': r.surface,
    'textBlockQuote.border': r.cobalt,
    'textCodeBlock.background': r.surface,
    'textPreformat.foreground': r.link,
    'textSeparator.foreground': r.borderStrong,

    'notifications.background': r.raised,
    'notifications.foreground': r.textPrimary,
    'notifications.border': r.borderStrong,
    'notificationCenterHeader.background': r.surface,
    'notificationCenterHeader.foreground': r.textPrimary,
    'notificationsErrorIcon.foreground': r.error,
    'notificationsWarningIcon.foreground': r.warn,
    'notificationsInfoIcon.foreground': r.cobalt,

    'menu.background': r.raised,
    'menu.foreground': r.textPrimary,
    'menu.selectionBackground': r.listActive,
    'menu.selectionForeground': r.textPrimary,
    'menu.border': r.borderStrong,
    'menu.separatorBackground': r.borderSubtle,
    'menubar.selectionBackground': r.hover,
    'menubar.selectionForeground': r.textPrimary,

    'peekView.border': r.cobalt,
    'peekViewEditor.background': r.editorBg,
    'peekViewEditor.matchHighlightBackground': alpha(r.warnBright, 0.4),
    'peekViewResult.background': r.surface,
    'peekViewResult.matchHighlightBackground': r.findMatchHi,
    'peekViewResult.selectionBackground': r.listActive,
    'peekViewTitle.background': r.surface,
    'peekViewTitleLabel.foreground': r.textPrimary,
    'peekViewTitleDescription.foreground': r.textSecondary,

    'gitDecoration.addedResourceForeground': r.success,
    'gitDecoration.modifiedResourceForeground': r.cobalt,
    'gitDecoration.deletedResourceForeground': r.error,
    'gitDecoration.untrackedResourceForeground': r.success,
    'gitDecoration.ignoredResourceForeground': r.textMuted,
    'gitDecoration.conflictingResourceForeground': r.warn,

    'minimap.background': r.editorBg,
    'minimap.findMatchHighlight': r.warn,
    'minimap.selectionHighlight': r.cobalt,
    'widget.shadow': r.shadow,
  };
}

function tokenColors(r) {
  return [
    { scope: ['comment', 'punctuation.definition.comment'], settings: { foreground: r.synComment, fontStyle: 'italic' } },
    { scope: ['string', 'string.quoted', 'constant.other.symbol'], settings: { foreground: r.synString } },
    { scope: ['constant.numeric', 'constant.language', 'constant.character'], settings: { foreground: r.synNumber } },
    { scope: ['keyword', 'storage.type', 'storage.modifier', 'keyword.control'], settings: { foreground: r.synKeyword } },
    { scope: ['entity.name.function', 'support.function', 'meta.function-call'], settings: { foreground: r.synFunction } },
    { scope: ['entity.name.type', 'entity.name.class', 'support.class', 'support.type'], settings: { foreground: r.synType } },
    { scope: ['variable', 'variable.other', 'meta.definition.variable'], settings: { foreground: r.synVariable } },
    { scope: ['variable.parameter', 'variable.language'], settings: { foreground: r.synParam } },
    { scope: ['entity.name.tag', 'punctuation.definition.tag'], settings: { foreground: r.synTag } },
    { scope: ['entity.other.attribute-name'], settings: { foreground: r.synAttr } },
    { scope: ['invalid', 'invalid.illegal'], settings: { foreground: r.synInvalid } },
  ];
}

const GENERATED = 'GENERATED from packages/design-tokens/brand.ts — do not edit by hand. Run `npm run gen` (or the package build) after changing brand.ts.';

function theme(mode, label) {
  const r = roles(mode);
  return JSON.stringify({
    name: label,
    type: mode,
    '//generated': GENERATED,
    colors: colors(r),
    tokenColors: tokenColors(r),
  }, null, 2) + '\n';
}

function css() {
  const d = modes.dark;
  return [
    `/* ${GENERATED} */`,
    ':root {',
    '    /* Fonts (brand.ts typography) */',
    `    --ironflyer-font-ui: ${typography.body};`,
    `    --ironflyer-font-mono: ${typography.mono};`,
    '',
    '    /* Signature gradient (cobalt -> cyan) */',
    `    --ironflyer-cobalt: ${palette.cobalt};`,
    `    --ironflyer-cyan: ${palette.cyan};`,
    `    --ironflyer-gradient: ${gradient.signature};`,
    '',
    '    /* Ink backgrounds (dark mode) */',
    `    --ironflyer-bg: ${d.bg};`,
    `    --ironflyer-surface: ${d.surface};`,
    `    --ironflyer-raised: ${d.surfaceRaised};`,
    '',
    '    /* Text */',
    `    --ironflyer-text-primary: ${d.textPrimary};`,
    `    --ironflyer-text-secondary: ${d.textSecondary};`,
    `    --ironflyer-text-muted: ${d.textMuted};`,
    '}',
    '',
  ].join('\n');
}

// ── write artifacts ─────────────────────────────────────────────────────────
writeFileSync(join(brandingDir, 'style', 'brand-tokens.generated.css'), css());
writeFileSync(join(brandingDir, 'themes', 'ironflyer-dark-color-theme.json'), theme('dark', 'Ironflyer Dark'));
writeFileSync(join(brandingDir, 'themes', 'ironflyer-light-color-theme.json'), theme('light', 'Ironflyer Light'));

console.log('[brand-theme] regenerated CSS vars + dark/light themes from brand.ts');

#!/usr/bin/env node
// Render every templates/sites/<cat>/<slug>/index.html to a JPEG preview
// at clients/web/public/templates/<slug>.jpg, then rewrite each template.json
// previewImage to point at /templates/<slug>.jpg (served by Next).
//
// Usage:
//   node scripts/templates/screenshot.mjs               # all templates
//   node scripts/templates/screenshot.mjs <slug-prefix> # subset
//
// Requires: playwright (clients/web devDep) + chromium installed via
// `npx playwright install chromium`.

import { readFileSync, writeFileSync, existsSync, mkdirSync, readdirSync, statSync } from 'node:fs';
import { join, resolve, dirname } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { createRequire } from 'node:module';

const REPO_ROOT = resolve(dirname(fileURLToPath(import.meta.url)), '..', '..');

// Resolve playwright from clients/web/node_modules — the only place it's
// installed in this monorepo. Use createRequire so this script can be
// invoked from any cwd.
const require = createRequire(join(REPO_ROOT, 'apps', 'web', 'package.json'));
const { chromium } = require('playwright');
const SITES_ROOT = join(REPO_ROOT, 'templates', 'sites');
const PREVIEW_OUT = join(REPO_ROOT, 'apps', 'web', 'public', 'templates');
const LIVE_OUT = join(REPO_ROOT, 'apps', 'web', 'public', 'template-previews');

const VIEWPORT = { width: 1280, height: 800 };
const SCREENSHOT_WIDTH = 1200;
const SCREENSHOT_HEIGHT = 750;
const FILTER = process.argv[2] ?? '';

if (!existsSync(PREVIEW_OUT)) mkdirSync(PREVIEW_OUT, { recursive: true });
if (!existsSync(LIVE_OUT)) mkdirSync(LIVE_OUT, { recursive: true });

function isDir(p) { try { return statSync(p).isDirectory(); } catch { return false; } }

function discoverTemplates() {
  const out = [];
  const cats = readdirSync(SITES_ROOT).filter((n) => isDir(join(SITES_ROOT, n))).sort();
  for (const cat of cats) {
    const catDir = join(SITES_ROOT, cat);
    const slugs = readdirSync(catDir).filter((n) => isDir(join(catDir, n))).sort();
    for (const slug of slugs) {
      const metaPath = join(catDir, slug, 'template.json');
      const htmlPath = join(catDir, slug, 'index.html');
      if (!existsSync(metaPath) || !existsSync(htmlPath)) continue;
      const meta = JSON.parse(readFileSync(metaPath, 'utf8'));
      if (FILTER && !meta.slug.includes(FILTER)) continue;
      out.push({ category: cat, slug, slugDir: join(catDir, slug), metaPath, htmlPath, meta });
    }
  }
  return out;
}

async function shoot(browser, t) {
  const ctx = await browser.newContext({
    viewport: VIEWPORT,
    deviceScaleFactor: 1.5,
    reducedMotion: 'reduce',
  });
  const page = await ctx.newPage();
  page.setDefaultTimeout(15000);
  const url = pathToFileURL(t.htmlPath).href;
  const errors = [];
  page.on('pageerror', (err) => errors.push(`pageerror: ${err.message}`));
  page.on('requestfailed', (req) => {
    // picsum.photos sometimes 504s under load — ignore image failures.
    if (req.resourceType() === 'image' || req.resourceType() === 'font') return;
    errors.push(`requestfailed: ${req.url()} (${req.failure()?.errorText})`);
  });
  try {
    await page.goto(url, { waitUntil: 'networkidle', timeout: 12000 });
  } catch (e) {
    // networkidle can time out if picsum.photos is slow — fall back to DOM
    try { await page.goto(url, { waitUntil: 'domcontentloaded', timeout: 8000 }); } catch {}
  }
  // Give web fonts + lazy images one more beat to settle.
  await page.waitForTimeout(900);

  const outPath = join(PREVIEW_OUT, `${t.meta.slug}.jpg`);
  await page.screenshot({
    path: outPath,
    type: 'jpeg',
    quality: 78,
    fullPage: false,
    clip: { x: 0, y: 0, width: VIEWPORT.width, height: VIEWPORT.height },
  });
  await ctx.close();

  // Resize via reading + re-saving via screenshot clip is unnecessary;
  // browsers emit the clip dimensions directly. previewImage path served
  // by Next from /public so it's `/templates/<slug>.jpg`.
  const localPreview = `/templates/${t.meta.slug}.jpg`;
  const livePreview = `/template-previews/${t.meta.slug}.html`;
  let metaDirty = false;
  if (t.meta.previewImage !== localPreview) { t.meta.previewImage = localPreview; metaDirty = true; }
  if (t.meta.livePreview !== livePreview)   { t.meta.livePreview   = livePreview;   metaDirty = true; }
  if (metaDirty) writeFileSync(t.metaPath, JSON.stringify(t.meta, null, 2) + '\n', 'utf8');

  // Publish the raw HTML to clients/web/public/template-previews/<slug>.html
  // so the gallery's "Live preview" CTA can open the real, fully-rendered
  // template in a new tab. Cheap to copy on every run; keeps source +
  // published copy in sync.
  const liveOutPath = join(LIVE_OUT, `${t.meta.slug}.html`);
  writeFileSync(liveOutPath, readFileSync(t.htmlPath));
  return { ok: errors.length === 0, errors };
}

async function main() {
  const templates = discoverTemplates();
  if (!templates.length) {
    console.error('no templates found' + (FILTER ? ` matching "${FILTER}"` : ''));
    process.exit(1);
  }
  console.log(`shooting ${templates.length} template${templates.length === 1 ? '' : 's'} → ${PREVIEW_OUT.replace(REPO_ROOT + '/', '')}/`);
  const browser = await chromium.launch({ headless: true });
  const issues = [];
  const POOL = 6; // parallelism
  let cursor = 0;
  let done = 0;
  async function worker(workerId) {
    while (true) {
      const i = cursor++;
      if (i >= templates.length) return;
      const t = templates[i];
      const t0 = Date.now();
      try {
        const res = await shoot(browser, t);
        const ms = Date.now() - t0;
        done++;
        const tag = res.ok ? 'ok ' : 'WARN';
        console.log(`[${String(done).padStart(3)}/${templates.length}] ${tag} ${t.meta.slug.padEnd(34)} ${String(ms).padStart(5)}ms`);
        if (!res.ok) issues.push({ slug: t.meta.slug, errors: res.errors });
      } catch (e) {
        done++;
        console.log(`[${String(done).padStart(3)}/${templates.length}] FAIL ${t.meta.slug} — ${e.message}`);
        issues.push({ slug: t.meta.slug, errors: [e.message] });
      }
    }
  }
  await Promise.all(Array.from({ length: POOL }, (_, i) => worker(i)));
  await browser.close();

  if (issues.length) {
    console.error(`\n${issues.length} template${issues.length === 1 ? '' : 's'} produced warnings or errors:`);
    for (const { slug, errors } of issues) {
      console.error(`  ${slug}`);
      for (const err of errors.slice(0, 3)) console.error(`    - ${err}`);
    }
  } else {
    console.log('\nAll templates rendered without errors.');
  }
}

main().catch((e) => { console.error(e); process.exit(1); });

#!/usr/bin/env node
// Copy every templates/sites/<cat>/<slug>/index.html into
// clients/web/public/template-previews/<slug>.html so the gallery can open
// each template as a live preview in a new tab. Also stamps each
// template.json with `livePreview` pointing at the published URL.
//
// Usage: node scripts/templates/publish-previews.mjs

import { readFileSync, writeFileSync, existsSync, mkdirSync, readdirSync, statSync } from 'node:fs';
import { join, resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const REPO_ROOT = resolve(dirname(fileURLToPath(import.meta.url)), '..', '..');
const SITES_ROOT = join(REPO_ROOT, 'templates', 'sites');
const LIVE_OUT  = join(REPO_ROOT, 'apps', 'web', 'public', 'template-previews');

if (!existsSync(LIVE_OUT)) mkdirSync(LIVE_OUT, { recursive: true });

function isDir(p) { try { return statSync(p).isDirectory(); } catch { return false; } }

let published = 0;
let filesCopied = 0;
const cats = readdirSync(SITES_ROOT).filter((n) => isDir(join(SITES_ROOT, n))).sort();
for (const cat of cats) {
  if (cat === '_shared') continue;
  const catDir = join(SITES_ROOT, cat);
  const slugs = readdirSync(catDir).filter((n) => isDir(join(catDir, n))).sort();
  for (const slug of slugs) {
    const slugDir = join(catDir, slug);
    const metaPath = join(slugDir, 'template.json');
    const htmlPath = join(slugDir, 'index.html');
    if (!existsSync(metaPath) || !existsSync(htmlPath)) continue;
    const meta = JSON.parse(readFileSync(metaPath, 'utf8'));

    // Directory mode: each template gets its own subdir so multi-page
    // templates (about.html / pricing.html / contact.html) resolve
    // their relative nav links correctly.
    const outDir = join(LIVE_OUT, meta.slug);
    if (!existsSync(outDir)) mkdirSync(outDir, { recursive: true });
    const htmlFiles = readdirSync(slugDir).filter((n) => n.endsWith('.html'));
    for (const f of htmlFiles) {
      writeFileSync(join(outDir, f), readFileSync(join(slugDir, f)));
      filesCopied++;
    }

    // Also drop a flat copy of index.html at /template-previews/<slug>.html
    // for backwards compatibility with any cached URLs pointing there.
    writeFileSync(join(LIVE_OUT, `${meta.slug}.html`), readFileSync(htmlPath));

    const livePreview = `/template-previews/${meta.slug}/`;
    if (meta.livePreview !== livePreview) {
      meta.livePreview = livePreview;
      writeFileSync(metaPath, JSON.stringify(meta, null, 2) + '\n', 'utf8');
    }
    published++;
  }
}
console.log(`published ${published} templates (${filesCopied} html files) → ${LIVE_OUT.replace(REPO_ROOT + '/', '')}/`);

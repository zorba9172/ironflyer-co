#!/usr/bin/env node
// Copy every templates/sites/<cat>/<slug>/index.html into
// apps/web/public/template-previews/<slug>.html so the gallery can open
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
const cats = readdirSync(SITES_ROOT).filter((n) => isDir(join(SITES_ROOT, n))).sort();
for (const cat of cats) {
  const catDir = join(SITES_ROOT, cat);
  const slugs = readdirSync(catDir).filter((n) => isDir(join(catDir, n))).sort();
  for (const slug of slugs) {
    const metaPath = join(catDir, slug, 'template.json');
    const htmlPath = join(catDir, slug, 'index.html');
    if (!existsSync(metaPath) || !existsSync(htmlPath)) continue;
    const meta = JSON.parse(readFileSync(metaPath, 'utf8'));
    const livePreview = `/template-previews/${meta.slug}.html`;
    if (meta.livePreview !== livePreview) {
      meta.livePreview = livePreview;
      writeFileSync(metaPath, JSON.stringify(meta, null, 2) + '\n', 'utf8');
    }
    writeFileSync(join(LIVE_OUT, `${meta.slug}.html`), readFileSync(htmlPath));
    published++;
  }
}
console.log(`published ${published} template previews → ${LIVE_OUT.replace(REPO_ROOT + '/', '')}/`);

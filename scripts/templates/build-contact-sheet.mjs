#!/usr/bin/env node
// Build a one-page contact sheet of all 150 templates at
// apps/web/public/template-previews/index.html. Useful for QA, internal
// reviews, and showing the full gallery at a glance.
//
// Usage: node scripts/templates/build-contact-sheet.mjs

import { readFileSync, writeFileSync, readdirSync, statSync, existsSync } from 'node:fs';
import { join, resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const REPO_ROOT = resolve(dirname(fileURLToPath(import.meta.url)), '..', '..');
const SITES_ROOT = join(REPO_ROOT, 'templates', 'sites');
const OUT = join(REPO_ROOT, 'apps', 'web', 'public', 'template-previews', 'index.html');

function isDir(p) { try { return statSync(p).isDirectory(); } catch { return false; } }

const byCategory = new Map();
const cats = readdirSync(SITES_ROOT).filter((n) => isDir(join(SITES_ROOT, n))).sort();
let total = 0;
for (const cat of cats) {
  const catDir = join(SITES_ROOT, cat);
  const slugs = readdirSync(catDir).filter((n) => isDir(join(catDir, n))).sort();
  const items = [];
  for (const slug of slugs) {
    const metaPath = join(catDir, slug, 'template.json');
    if (!existsSync(metaPath)) continue;
    items.push(JSON.parse(readFileSync(metaPath, 'utf8')));
    total++;
  }
  if (items.length) byCategory.set(cat, items);
}

const escape = (s = '') => String(s).replace(/[&<>"']/g, (c) =>
  ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]),
);

const sections = [...byCategory.entries()].map(([cat, items]) => {
  const cards = items.map((t) => `
    <a class="card" href="./${escape(t.slug)}.html" target="_blank" rel="noopener noreferrer">
      <div class="thumb">
        <img src="../templates/${escape(t.slug)}.jpg" alt="${escape(t.title)} preview" loading="lazy" />
      </div>
      <div class="meta">
        <span class="title">${escape(t.title)}</span>
        <span class="slug">${escape(t.slug)}</span>
        <span class="sub">${escape(t.subtitle)}</span>
        <div class="palette" aria-hidden="true">
          <span style="background:${escape(t.palette?.bg ?? '#000')}"></span>
          <span style="background:${escape(t.palette?.fg ?? '#fff')}"></span>
          <span style="background:${escape(t.palette?.accent ?? '#ccc')}"></span>
        </div>
      </div>
    </a>`).join('');
  return `
    <section id="${escape(cat)}">
      <header>
        <h2>${escape(items[0]?.category ?? cat)}</h2>
        <span class="count">${items.length} templates</span>
      </header>
      <div class="grid">${cards}</div>
    </section>`;
}).join('');

const navLinks = [...byCategory.entries()].map(([cat, items]) =>
  `<a href="#${escape(cat)}">${escape(items[0]?.category ?? cat)} <span>${items.length}</span></a>`,
).join('');

const html = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>Ironflyer template gallery — contact sheet (${total})</title>
  <link rel="preconnect" href="https://fonts.googleapis.com" />
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin />
  <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;700;900&family=JetBrains+Mono:wght@400;500&display=swap" rel="stylesheet" />
  <style>
    :root {
      --bg: #0d0e0f;
      --surface: #151513;
      --surface-2: #1d1d1a;
      --border: rgba(244, 240, 232, 0.12);
      --border-strong: rgba(244, 240, 232, 0.24);
      --fg: #f7f3ea;
      --fg-2: #b9b3a8;
      --fg-3: #77736b;
      --accent: #e5ff00;
    }
    *, *::before, *::after { box-sizing: border-box; }
    html, body { margin: 0; padding: 0; }
    body {
      background: var(--bg);
      color: var(--fg);
      font-family: 'Inter', system-ui, sans-serif;
      -webkit-font-smoothing: antialiased;
    }
    a { color: inherit; text-decoration: none; }
    img { display: block; max-width: 100%; height: auto; }
    .mono { font-family: 'JetBrains Mono', ui-monospace, monospace; }

    .top {
      position: sticky; top: 0; z-index: 10;
      background: rgba(13, 14, 15, 0.86);
      backdrop-filter: blur(14px);
      border-bottom: 1px solid var(--border);
    }
    .top-inner {
      max-width: 1400px; margin: 0 auto; padding: 18px 28px;
      display: flex; align-items: center; gap: 24px;
    }
    .brand {
      font-weight: 900; letter-spacing: -0.01em; font-size: 18px;
      display: flex; align-items: center; gap: 10px;
    }
    .brand .mark {
      width: 24px; height: 24px; border-radius: 6px;
      background: var(--accent); color: #111;
      display: grid; place-items: center; font-weight: 900; font-family: 'JetBrains Mono', monospace;
    }
    .summary { color: var(--fg-2); font-size: 14px; font-family: 'JetBrains Mono', monospace; }
    nav.cats {
      margin-left: auto; display: flex; gap: 10px; flex-wrap: wrap; font-size: 12px;
      max-width: 70%; justify-content: flex-end;
    }
    nav.cats a {
      padding: 5px 10px; border-radius: 6px;
      border: 1px solid var(--border);
      color: var(--fg-2); font-family: 'JetBrains Mono', monospace;
    }
    nav.cats a:hover { color: var(--fg); border-color: var(--border-strong); }
    nav.cats a span { color: var(--fg-3); margin-left: 4px; }

    main { max-width: 1400px; margin: 0 auto; padding: 36px 28px 80px; }
    section { margin-top: 64px; }
    section:first-of-type { margin-top: 16px; }
    section > header {
      display: flex; align-items: baseline; justify-content: space-between; gap: 16px;
      padding-bottom: 16px; border-bottom: 1px solid var(--border);
      margin-bottom: 24px;
    }
    section h2 {
      font-size: 36px; font-weight: 900; letter-spacing: -0.02em;
      margin: 0;
    }
    section .count {
      color: var(--fg-2); font-family: 'JetBrains Mono', monospace; font-size: 12px;
      text-transform: uppercase; letter-spacing: 0.1em;
    }

    .grid {
      display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: 18px;
    }
    .card {
      background: var(--surface);
      border: 1px solid var(--border);
      border-radius: 12px;
      overflow: hidden;
      display: flex; flex-direction: column;
      transition: transform 220ms cubic-bezier(0.22, 1, 0.36, 1), border-color 220ms;
    }
    .card:hover {
      transform: translateY(-3px);
      border-color: var(--border-strong);
    }
    .thumb {
      aspect-ratio: 16 / 10;
      background: var(--surface-2);
      overflow: hidden;
    }
    .thumb img { width: 100%; height: 100%; object-fit: cover; }
    .meta {
      padding: 16px 16px 18px;
      display: flex; flex-direction: column; gap: 6px;
    }
    .title { font-weight: 700; letter-spacing: -0.01em; font-size: 15px; }
    .slug { font-family: 'JetBrains Mono', monospace; font-size: 11px; color: var(--fg-3); }
    .sub { color: var(--fg-2); font-size: 13px; line-height: 1.45; margin-top: 4px; }
    .palette {
      display: flex; gap: 6px; margin-top: 10px;
    }
    .palette span {
      width: 18px; height: 18px; border-radius: 4px;
      border: 1px solid var(--border);
    }

    footer.bottom {
      max-width: 1400px; margin: 80px auto 0; padding: 32px 28px 48px;
      border-top: 1px solid var(--border);
      color: var(--fg-3); font-family: 'JetBrains Mono', monospace; font-size: 12px;
      display: flex; justify-content: space-between; align-items: center; gap: 16px;
    }
    @media (max-width: 720px) {
      .grid { grid-template-columns: repeat(auto-fill, minmax(220px, 1fr)); }
      nav.cats { display: none; }
      .top-inner { padding: 14px 18px; }
      main { padding: 24px 18px 56px; }
      section h2 { font-size: 28px; }
    }
  </style>
</head>
<body>
  <div class="top">
    <div class="top-inner">
      <div class="brand"><span class="mark">▮</span> Ironflyer · Template gallery</div>
      <span class="summary">${total} templates · ${byCategory.size} categories</span>
      <nav class="cats" aria-label="Category jump">${navLinks}</nav>
    </div>
  </div>
  <main>
    ${sections}
  </main>
  <footer class="bottom">
    <span>Generated by scripts/templates/build-contact-sheet.mjs</span>
    <span>${new Date().toISOString().slice(0, 10)}</span>
  </footer>
</body>
</html>
`;

writeFileSync(OUT, html, 'utf8');
console.log(`wrote ${OUT.replace(REPO_ROOT + '/', '')} — ${total} templates`);

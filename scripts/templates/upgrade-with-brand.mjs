#!/usr/bin/env node
// Inject the Ironflyer brand kit into every template's index.html:
//   - scroll-reveal CSS (just before </head>)
//   - "Built with Ironflyer" signature + IO bootstrap (just before </body>)
//
// Both blocks are bounded by data-if-brand markers so the script is
// idempotent — running it twice replaces in place instead of stacking.
//
// Usage:
//   node scripts/templates/upgrade-with-brand.mjs              # all
//   node scripts/templates/upgrade-with-brand.mjs <slug-prefix> # subset

import { readFileSync, writeFileSync, readdirSync, statSync, existsSync } from 'node:fs';
import { join, resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const REPO_ROOT = resolve(dirname(fileURLToPath(import.meta.url)), '..', '..');
const SITES_ROOT = join(REPO_ROOT, 'templates', 'sites');
const FILTER = process.argv[2] ?? '';

function isDir(p) { try { return statSync(p).isDirectory(); } catch { return false; } }

// ---------- the brand kit ----------

const HEAD_OPEN  = '<!-- ironflyer-brand:head:start -->';
const HEAD_CLOSE = '<!-- ironflyer-brand:head:end -->';
const FOOT_OPEN  = '<!-- ironflyer-brand:foot:start -->';
const FOOT_CLOSE = '<!-- ironflyer-brand:foot:end -->';

const BRAND_HEAD = `${HEAD_OPEN}
<style data-if-brand>
/* Ironflyer brand kit — subtle scroll-reveal */
section,.feature,.plan,.card,.product,.listing,.item,.testimonial,.case-study,.post,.work,.member,.instructor,.speaker,.dish,.menu-item{opacity:0}
@keyframes if-reveal-up{from{opacity:0;transform:translateY(14px)}to{opacity:1;transform:translateY(0)}}
.if-r-in{animation:if-reveal-up 720ms cubic-bezier(0.16,1,0.3,1) both;opacity:1!important}
@media (prefers-reduced-motion:reduce){section,.feature,.plan,.card,.product,.listing,.item,.testimonial,.case-study,.post,.work,.member,.instructor,.speaker,.dish,.menu-item{opacity:1!important}.if-r-in{animation:none!important}}
/* Ironflyer signature */
.if-sig{position:fixed;bottom:24px;left:24px;z-index:9999;font-family:'Inter',-apple-system,BlinkMacSystemFont,system-ui,sans-serif;font-size:12px;line-height:1;transition:opacity 240ms ease}
.if-sig:hover{transform:none}
.if-sig a{display:inline-flex;align-items:center;gap:8px;padding:8px 14px 8px 8px;background:#0d0e0f;border:1px solid rgba(244,240,232,0.18);border-radius:999px;color:#f7f3ea!important;text-decoration:none!important;font-weight:500;letter-spacing:-0.005em;box-shadow:0 6px 24px rgba(0,0,0,0.32);transition:transform 220ms cubic-bezier(0.22,1,0.36,1),border-color 220ms}
.if-sig a:hover{border-color:rgba(229,255,0,0.7);transform:translateY(-1px)}
.if-sig .if-sig-mark{width:18px;height:18px;background:#e5ff00;color:#0d0e0f;display:grid;place-items:center;border-radius:4px;font-weight:900;font-family:'JetBrains Mono',ui-monospace,monospace;font-size:11px}
.if-sig strong{font-weight:700;color:#f7f3ea}
@media print{.if-sig{display:none}}
</style>
<noscript><style>section,.feature,.plan,.card,.product,.listing,.item,.testimonial,.case-study,.post,.work,.member,.instructor,.speaker,.dish,.menu-item{opacity:1!important}</style></noscript>
${HEAD_CLOSE}`;

const BRAND_FOOT = `${FOOT_OPEN}
<a class="if-sig" href="https://ironflyer.dev" target="_blank" rel="noopener" aria-label="Built with Ironflyer"><span class="if-sig-mark">▮</span><span>Built with <strong>Ironflyer</strong></span></a>
<script data-if-brand>
(function(){
  var sel='section,.feature,.plan,.card,.product,.listing,.item,.testimonial,.case-study,.post,.work,.member,.instructor,.speaker,.dish,.menu-item';
  if(!('IntersectionObserver' in window)||matchMedia('(prefers-reduced-motion:reduce)').matches){
    document.querySelectorAll(sel).forEach(function(el){el.style.opacity='1'});
    return;
  }
  function go(){
    var els=document.querySelectorAll(sel);
    var io=new IntersectionObserver(function(entries){
      entries.forEach(function(e){
        if(e.isIntersecting){e.target.classList.add('if-r-in');io.unobserve(e.target)}
      });
    },{rootMargin:'-6% 0px -2% 0px'});
    els.forEach(function(el){io.observe(el)});
  }
  document.readyState!=='loading'?go():document.addEventListener('DOMContentLoaded',go);
})();
</script>
${FOOT_CLOSE}`;

// One regex matches both: idempotent replace.
const HEAD_RE = new RegExp(`${HEAD_OPEN}[\\s\\S]*?${HEAD_CLOSE}`);
const FOOT_RE = new RegExp(`${FOOT_OPEN}[\\s\\S]*?${FOOT_CLOSE}`);

function upgrade(html) {
  let out = html;
  if (HEAD_RE.test(out)) {
    out = out.replace(HEAD_RE, BRAND_HEAD);
  } else if (out.includes('</head>')) {
    out = out.replace('</head>', `${BRAND_HEAD}\n</head>`);
  } else {
    // unusual — leave the file alone
    return { html: out, changed: false, reason: 'no </head> tag' };
  }
  if (FOOT_RE.test(out)) {
    out = out.replace(FOOT_RE, BRAND_FOOT);
  } else if (out.includes('</body>')) {
    out = out.replace('</body>', `${BRAND_FOOT}\n</body>`);
  } else {
    return { html: out, changed: false, reason: 'no </body> tag' };
  }
  return { html: out, changed: out !== html, reason: null };
}

// ---------- run over the tree ----------

let scanned = 0, updated = 0, skipped = [];
const cats = readdirSync(SITES_ROOT).filter((n) => isDir(join(SITES_ROOT, n))).sort();
for (const cat of cats) {
  if (cat === '_shared') continue;
  const catDir = join(SITES_ROOT, cat);
  for (const slug of readdirSync(catDir).filter((n) => isDir(join(catDir, n))).sort()) {
    const htmlPath = join(catDir, slug, 'index.html');
    if (!existsSync(htmlPath)) continue;
    const full = `${cat}/${slug}`;
    // Accept manifest-style slugs too (e.g., "saas-01-codeforge" or just "saas").
    const manifestSlug = `${cat}-${slug.replace(/^\d+-/, '')}`;
    const dashedSlug = `${cat}-${slug}`; // cat-NN-name form
    if (FILTER && ![full, slug, cat, manifestSlug, dashedSlug].some((s) => s.includes(FILTER))) continue;
    scanned++;
    const before = readFileSync(htmlPath, 'utf8');
    const { html: after, changed, reason } = upgrade(before);
    if (reason) { skipped.push(`${full} — ${reason}`); continue; }
    if (changed) {
      writeFileSync(htmlPath, after, 'utf8');
      updated++;
    }
  }
}
console.log(`scanned ${scanned}, updated ${updated}` + (skipped.length ? `, skipped ${skipped.length}` : ''));
for (const s of skipped) console.log(`  skip: ${s}`);

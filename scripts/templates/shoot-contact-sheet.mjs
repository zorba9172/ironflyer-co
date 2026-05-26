#!/usr/bin/env node
// Take a screenshot of the contact sheet so we can verify the full
// gallery view renders correctly. Output: clients/web/public/template-previews/contact-sheet.jpg.

import { writeFileSync } from 'node:fs';
import { join, resolve, dirname } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { createRequire } from 'node:module';

const REPO_ROOT = resolve(dirname(fileURLToPath(import.meta.url)), '..', '..');
const require = createRequire(join(REPO_ROOT, 'apps', 'web', 'package.json'));
const { chromium } = require('playwright');

const sheet = join(REPO_ROOT, 'apps', 'web', 'public', 'template-previews', 'index.html');
const out = join(REPO_ROOT, 'apps', 'web', 'public', 'template-previews', 'contact-sheet.jpg');

const browser = await chromium.launch({ headless: true });
const ctx = await browser.newContext({ viewport: { width: 1400, height: 900 }, deviceScaleFactor: 1.5 });
const page = await ctx.newPage();
await page.goto(pathToFileURL(sheet).href, { waitUntil: 'networkidle', timeout: 30000 }).catch(async () => {
  await page.goto(pathToFileURL(sheet).href, { waitUntil: 'domcontentloaded', timeout: 10000 });
});
await page.waitForTimeout(1200);
// Top-of-page screenshot first (1400x900)
await page.screenshot({ path: out, type: 'jpeg', quality: 80, fullPage: false });
console.log(`wrote ${out.replace(REPO_ROOT + '/', '')}`);
await browser.close();

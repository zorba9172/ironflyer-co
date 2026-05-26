// Drive the app as a real user would: sign in, start a project, send a
// chat message, watch what happens. Capture screenshots, console errors,
// network failures, and SSE/GraphQL bodies along the way.

import { createRequire } from "node:module";
import { mkdir, writeFile } from "node:fs/promises";
import path from "node:path";

const require = createRequire(path.resolve("apps/web/package.json"));
const { chromium } = require("@playwright/test");

const BASE = "http://127.0.0.1:3000";
const OUT = path.resolve("tmp/journey");
await mkdir(OUT, { recursive: true });

const browser = await chromium.launch({ headless: true });
const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 } });
const page = await ctx.newPage();

const log = [];
const say = (msg) => {
  console.log(msg);
  log.push(msg);
};

page.on("console", (m) => {
  const t = m.type();
  if (t === "error" || t === "warning") say(`[console.${t}] ${m.text()}`);
});
page.on("pageerror", (e) => say(`[pageerror] ${e.message}`));
page.on("requestfailed", (req) => {
  const url = req.url();
  if (url.includes("localhost") || url.includes("127.0.0.1")) {
    say(`[netfail] ${req.method()} ${url} → ${req.failure()?.errorText}`);
  }
});
page.on("response", async (resp) => {
  const url = resp.url();
  const status = resp.status();
  if (status >= 400 && (url.includes("localhost") || url.includes("127.0.0.1"))) {
    let body = "";
    try {
      body = (await resp.text()).slice(0, 300);
    } catch {}
    say(`[http ${status}] ${resp.request().method()} ${url} :: ${body}`);
  }
});

const shot = async (label) => {
  const p = path.join(OUT, `${String(log.length).padStart(3, "0")}-${label}.png`);
  await page.screenshot({ path: p, fullPage: true });
  say(`[screenshot] ${p}`);
};

async function step(name, fn) {
  say(`\n=== STEP: ${name} ===`);
  try {
    await fn();
  } catch (e) {
    say(`[step-error] ${name}: ${e.message}`);
    await shot(`error-${name.replace(/\s+/g, "_")}`);
  }
}

// ---- 1. landing -----------------------------------------------------
await step("open home", async () => {
  await page.goto(BASE, { waitUntil: "networkidle", timeout: 30000 });
  await shot("home");
});

// ---- 2. sign in -----------------------------------------------------
await step("sign in", async () => {
  await page.goto(BASE + "/login", { waitUntil: "networkidle", timeout: 30000 });
  await shot("login");
  await page.fill('input[name="email"], input[type="email"]', "demo@ironflyer.dev");
  await page.fill('input[name="password"], input[type="password"]', "demo1234");
  await shot("login-filled");
  // The submit button text usually says Sign in / Log in
  const submit = page
    .locator('button[type="submit"]')
    .or(page.getByRole("button", { name: /sign in|log in|continue/i }))
    .first();
  await Promise.all([
    page.waitForLoadState("networkidle", { timeout: 30000 }).catch(() => {}),
    submit.click(),
  ]);
  await page.waitForTimeout(2000);
  await shot("post-login");
  say(`url after login: ${page.url()}`);
});

// ---- 3. land somewhere with a chat ----------------------------------
await step("open studio/demo", async () => {
  await page.goto(BASE + "/studio/demo", { waitUntil: "networkidle", timeout: 30000 });
  await page.waitForTimeout(2000);
  await shot("studio-demo");
  say(`url after studio/demo: ${page.url()}`);
});

await step("open projects page", async () => {
  await page.goto(BASE + "/projects", { waitUntil: "networkidle", timeout: 30000 });
  await page.waitForTimeout(2000);
  await shot("projects");
});

await step("try to start a new project from home", async () => {
  await page.goto(BASE + "/", { waitUntil: "networkidle", timeout: 30000 });
  await page.waitForTimeout(1000);
  await shot("home-after-login");
  // Look for a textarea or "describe your idea" / "build" / "start" input.
  const ideaBox = page
    .locator("textarea")
    .or(page.locator('input[placeholder*="idea" i]'))
    .or(page.locator('input[placeholder*="build" i]'))
    .first();
  const visible = await ideaBox.isVisible().catch(() => false);
  if (!visible) {
    say("[note] no idea input visible on home — looking on /studio");
    return;
  }
  await ideaBox.fill("Build a simple React todo app with add/remove and dark mode.");
  await shot("home-idea-filled");
  const goBtn = page
    .getByRole("button", { name: /build|start|submit|describe|generate|begin/i })
    .first();
  if (await goBtn.isVisible().catch(() => false)) {
    await goBtn.click();
    await page.waitForTimeout(3000);
    await shot("home-after-submit");
    say(`url after submit: ${page.url()}`);
  } else {
    say("[note] no build/submit button visible");
  }
});

// ---- 4. try the studio chat directly --------------------------------
await step("try the studio chat input", async () => {
  // Look for any chat textarea on the page.
  const url = page.url();
  say(`current url: ${url}`);
  await shot("before-chat-try");
  const chatBox = page
    .locator('textarea[placeholder*="ask" i], textarea[placeholder*="chat" i], textarea[placeholder*="message" i], textarea[placeholder*="describe" i]')
    .first();
  const visible = await chatBox.isVisible().catch(() => false);
  if (!visible) {
    // fallback: just any textarea
    const anyText = page.locator("textarea").first();
    if (await anyText.isVisible().catch(() => false)) {
      await anyText.fill("Build a React counter app with + and - buttons.");
      await shot("chat-filled-fallback");
      const send = page
        .getByRole("button", { name: /send|build|generate|submit/i })
        .first();
      if (await send.isVisible().catch(() => false)) {
        await send.click();
        await page.waitForTimeout(8000);
        await shot("chat-after-send");
      } else {
        await anyText.press("Enter");
        await page.waitForTimeout(8000);
        await shot("chat-after-enter");
      }
      return;
    }
    say("[note] no chat input found at all");
    return;
  }
  await chatBox.fill("Build a React counter app with + and - buttons.");
  await shot("chat-filled");
  const send = page
    .getByRole("button", { name: /send|build|generate|submit/i })
    .first();
  if (await send.isVisible().catch(() => false)) {
    await send.click();
  } else {
    await chatBox.press("Enter");
  }
  // Wait long enough for SSE deltas to land.
  await page.waitForTimeout(10000);
  await shot("chat-after-send");
});

// dump body text of the final page for grep
try {
  const text = await page.evaluate(() => document.body.innerText.slice(0, 4000));
  say(`\n--- final page body excerpt ---\n${text}\n--- end excerpt ---`);
} catch {}

await writeFile(path.join(OUT, "log.txt"), log.join("\n"), "utf8");
say(`\nlog → ${path.join(OUT, "log.txt")}`);

await browser.close();

import { createRequire } from "node:module";
import path from "node:path";

const require = createRequire(path.resolve("apps/web/package.json"));
const { chromium } = require("@playwright/test");

const BASE = "http://127.0.0.1:3000";
const ROUTE = process.argv[2] || "/";

const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1280, height: 800 } });
const page = await ctx.newPage();

page.on("console", (msg) => console.log(`[console.${msg.type()}] ${msg.text()}`));
page.on("pageerror", (err) => console.log(`[pageerror] ${err.stack || err.message}`));
page.on("requestfailed", (req) =>
  console.log(`[netfail] ${req.method()} ${req.url()} → ${req.failure()?.errorText}`),
);
page.on("response", (resp) => {
  if (resp.status() >= 400)
    console.log(`[http ${resp.status()}] ${resp.request().method()} ${resp.url()}`);
});

const resp = await page.goto(BASE + ROUTE, { waitUntil: "networkidle", timeout: 30000 });
console.log(`-- nav status: ${resp?.status()}`);
await page.waitForTimeout(2000);
console.log("-- done");
await browser.close();

import { createRequire } from "node:module";
import path from "node:path";

const require = createRequire(path.resolve("apps/web/package.json"));
const { chromium } = require("@playwright/test");

const BASE = process.env.PROBE_BASE_URL || "http://127.0.0.1:3000";
const ROUTES = [
  "/",
  "/login",
  "/signup",
  "/dashboard",
  "/projects",
  "/templates",
  "/wallet",
  "/studio",
  "/studio/demo",
  "/p/demo?executionID=demo",
  "/execution",
  "/execution/demo",
  "/deploy",
  "/settings",
  "/operator",
];

const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1280, height: 800 } });
const report = [];

for (const route of ROUTES) {
  const page = await ctx.newPage();
  const consoleErrors = [];
  const pageErrors = [];
  const failedRequests = [];
  page.on("console", (msg) => {
    if (msg.type() === "error") consoleErrors.push(msg.text());
  });
  page.on("pageerror", (err) => pageErrors.push(`${err.name}: ${err.message}`));
  page.on("requestfailed", (req) => {
    const url = req.url();
    if (url.startsWith(BASE) || url.includes("localhost")) {
      failedRequests.push(`${req.method()} ${url} → ${req.failure()?.errorText}`);
    }
  });
  let status = "ok";
  try {
    const resp = await page.goto(BASE + route, { waitUntil: "networkidle", timeout: 30000 });
    status = String(resp?.status() ?? "no-response");
    await page.waitForTimeout(800);
  } catch (e) {
    status = `nav-error: ${e.message}`;
  }
  report.push({ route, status, consoleErrors, pageErrors, failedRequests });
  await page.close();
}

await browser.close();

for (const r of report) {
  const issues = r.consoleErrors.length + r.pageErrors.length + r.failedRequests.length;
  if (issues === 0 && (r.status === "200" || r.status === "ok")) {
    console.log(`OK    ${r.route} (${r.status})`);
    continue;
  }
  console.log(`FAIL  ${r.route} (${r.status})  console=${r.consoleErrors.length} page=${r.pageErrors.length} netfail=${r.failedRequests.length}`);
  for (const e of r.pageErrors) console.log(`   PAGE ERR: ${e}`);
  for (const e of r.consoleErrors.slice(0, 8)) console.log(`   CON ERR: ${e}`);
  for (const e of r.failedRequests.slice(0, 8)) console.log(`   NET ERR: ${e}`);
}

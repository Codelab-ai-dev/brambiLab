// Lab measurement of LCP and CLS (WEB-009 §7). Not field data: a local, throttled Chromium run
// used only to compare before/after with the same method.
//   BASE_URL=http://localhost:8000 node scripts/vitals.mjs /es /en /es/proyectos
// Profile: mobile 412×823, CPU 4× slower, 150 ms RTT, 1.6 Mb/s down, cache disabled, 5 runs
// per page (median reported). Desktop 1440×900 without throttling as a second profile.
// A run whose page never fires "load" within 20 s (the emulated connection sometimes stalls with
// every request pending, including the CSS) is discarded and counted, never averaged in.
// JS cost: transferred .js bytes and Chromium's ScriptDuration (main-thread script time).
import { chromium } from "@playwright/test";

const BASE = process.env.BASE_URL ?? "http://localhost:8000";
const RUNS = Number(process.env.RUNS ?? 5);
const paths = process.argv.slice(2).length ? process.argv.slice(2) : ["/es", "/en"];
const profiles = [
  { name: "mobile-4g", viewport: { width: 412, height: 823 }, cpu: 4, net: { latency: 150, downloadThroughput: (1.6 * 1024 * 1024) / 8, uploadThroughput: (750 * 1024) / 8 } },
  { name: "desktop", viewport: { width: 1440, height: 900 }, cpu: 1, net: null },
];

const median = (xs) => [...xs].sort((a, b) => a - b)[Math.floor(xs.length / 2)];
const browser = await chromium.launch();
for (const p of profiles) {
  for (const path of paths) {
    const lcp = [];
    const cls = [];
    const kb = [];
    const jsKb = [];
    const scriptMs = [];
    let stalled = 0;
    for (let i = 0; i < RUNS && stalled < RUNS; ) {
      const ctx = await browser.newContext({ viewport: p.viewport });
      const page = await ctx.newPage();
      const cdp = await ctx.newCDPSession(page);
      await cdp.send("Network.enable");
      await cdp.send("Network.setCacheDisabled", { cacheDisabled: true });
      if (p.net) await cdp.send("Network.emulateNetworkConditions", { offline: false, ...p.net });
      await cdp.send("Emulation.setCPUThrottlingRate", { rate: p.cpu });
      await cdp.send("Performance.enable");
      await page.addInitScript(() => {
        window.__v = { lcp: 0, cls: 0 };
        new PerformanceObserver((l) => { for (const e of l.getEntries()) window.__v.lcp = e.startTime; }).observe({ type: "largest-contentful-paint", buffered: true });
        new PerformanceObserver((l) => { for (const e of l.getEntries()) if (!e.hadRecentInput) window.__v.cls += e.value; }).observe({ type: "layout-shift", buffered: true });
      });
      try {
        await page.goto(BASE + path, { waitUntil: "load", timeout: 20_000 });
      } catch {
        stalled++;
        await ctx.close();
        continue;
      }
      i++;
      await page.waitForTimeout(2000);
      const v = await page.evaluate(() => {
        const all = [...performance.getEntriesByType("navigation"), ...performance.getEntriesByType("resource")];
        return { ...window.__v, bytes: all.reduce((n, e) => n + e.transferSize, 0), js: all.filter((e) => /\.js(\?|$)/.test(e.name)).reduce((n, e) => n + e.transferSize, 0) };
      });
      // Main-thread time spent running script (Chromium's ScriptDuration), throttled CPU included.
      const metrics = Object.fromEntries((await cdp.send("Performance.getMetrics")).metrics.map((m) => [m.name, m.value]));
      lcp.push(v.lcp);
      cls.push(v.cls);
      kb.push(v.bytes / 1024);
      jsKb.push(v.js / 1024);
      scriptMs.push(metrics.ScriptDuration * 1000);
      await ctx.close();
    }
    console.log(`${p.name.padEnd(10)} ${path.padEnd(28)} LCP ${Math.round(median(lcp))} ms (min ${Math.round(Math.min(...lcp))}, max ${Math.round(Math.max(...lcp))})  CLS ${median(cls).toFixed(3)}  ${Math.round(median(kb))} KB  JS ${Math.round(median(jsKb))} KB  script ${Math.round(median(scriptMs))} ms${stalled ? `  (${stalled} stalled runs discarded)` : ""}`);
  }
}
await browser.close();

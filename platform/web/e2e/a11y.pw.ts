// WEB-009 delivery 3: accessibility audit of the public site through Caddy. Automated checks only
// cover part of WCAG; they complement, never replace, a review with real assistive technology.
// - axe (WCAG 2.0/2.1/2.2 A and AA rules, contrast included) on every public route, ES and EN,
//   on resting states (see below).
// - Keyboard: every focusable element shows a ring and is never hidden under the sticky header.
// - Zoom: 200 % page zoom (1280 px at 200 % = 640 CSS px) and 200 % text without horizontal scroll.
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";

const BASE = process.env.BASE_URL ?? "http://localhost:8000";
const run = Date.now().toString(36);
const png = readFileSync(fileURLToPath(new URL("../../api/internal/media/testdata/text.png", import.meta.url)));
const project = `/es/proyectos/audit-${run}`;
const routes = [
  "/es", "/en", "/es/proyectos", "/en/projects", "/es/articulos", "/en/articles", "/es/buscar", `/es/buscar?q=audit-${run}`, "/en/search",
  "/es/acerca-de", "/en/about", "/es/contacto", "/en/contact", "/es/no-existe", project, `${project}/bitacora/nota-${run}`, `/es/articulos/texto-${run}`,
];

async function seed(page: Page) {
  await page.goto("/admin");
  await page.getByRole("link", { name: "Iniciar sesión con GitHub" }).click();
  await page.waitForURL(/\/admin$/);
  const me = await (await page.request.get("/api/v1/auth/me")).json();
  const headers = { "X-CSRF-Token": me.csrf_token, Origin: BASE };
  const call = async (method: string, path: string, data?: unknown) => {
    const r = await page.request.fetch(path, { method, data, headers: { ...headers, "Content-Type": "application/json" } });
    expect(r.ok(), `${method} ${path}: ${await r.text()}`).toBe(true);
    return r.json();
  };
  const up = await page.request.post(`/api/v1/admin/assets?filename=audit-${run}.png`, { data: png, headers });
  const asset = (await up.json()).id;
  await call("PATCH", `/api/v1/admin/assets/${asset}`, { public_enabled: true, downloadable: false });
  const text = (s: string) => ({ type: "paragraph", content: [{ type: "text", text: s }, { type: "text", text: " enlace", marks: [{ type: "link", attrs: { href: "https://example.com" } }] }] });
  const body = { type: "doc", content: [text("Texto con un"), { type: "image", attrs: { assetId: asset, alt: "Placa de prueba" } }, { type: "heading", attrs: { level: 2 }, content: [{ type: "text", text: "Sección" }] }, text("Más texto con un")] };
  const publish = async (kind: string, snapshot: Record<string, unknown>, projectId?: string) => {
    const id = (await call("POST", "/api/v1/admin/contents", { kind, locale: "es", ...(projectId ? { project_id: projectId } : {}) })).id;
    await call("POST", `/api/v1/admin/contents/${id}/translations/es/revisions`, { expected_version: 0, kind: "manual", snapshot: { summary: "Resumen de prueba", body, ...snapshot } });
    const st = await call("GET", `/api/v1/admin/contents/${id}/translations/es/publication`);
    await call("POST", `/api/v1/admin/contents/${id}/translations/es/publication/publish`, { revision_version: 1, expected_editorial_version: st.editorial_version });
    return id;
  };
  const p = await publish("project", { title: `Audit ${run}`, slug: `audit-${run}`, cover_asset_id: asset, project_fields: { status: "completed", technologies: ["ESP32"], links: [{ label: "Repositorio", url: "https://example.com/repo" }] } });
  await publish("log", { title: `Nota ${run}`, slug: `nota-${run}` }, p);
  await publish("article", { title: `Texto ${run}`, slug: `texto-${run}` });
}

test.beforeAll(async ({ browser }) => {
  const page = await browser.newPage();
  await seed(page);
  await page.close();
});

const axe = (page: Page) => new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"]).analyze();

// axe measures colours as painted at that instant, so it audits resting states: reduced motion
// shows every final state at once (#52), with the same colours as after the animations.
test("axe finds no WCAG A/AA violations on any public route (phone and desktop)", async ({ browser }) => {
  for (const width of [375, 1440]) {
    const ctx = await browser.newContext({ viewport: { width, height: 900 }, reducedMotion: "reduce" });
    const page = await ctx.newPage();
    for (const route of routes) {
      await page.goto(route);
      const { violations } = await axe(page);
      expect(violations.map((v) => `${v.id}: ${v.nodes.map((n) => n.target.join(" ")).join(", ")}`), `${route} @${width}`).toEqual([]);
    }
    await ctx.close();
  }
});

test("with motion on, the home settles into the same accessible state", async ({ page }) => {
  await page.goto("/es");
  const height = await page.evaluate(() => document.documentElement.scrollHeight);
  for (let y = 0; y < height; y += 400) await page.mouse.wheel(0, 400);
  await expect.poll(() => page.evaluate(() => document.querySelectorAll("[data-reveal-pending]").length)).toBe(0);
  // Wait for entrances and reveals to finish.
  await expect.poll(() => page.evaluate(() => document.getAnimations().filter((a) => a.playState === "running").length)).toBe(0);
  const { violations } = await axe(page);
  expect(violations.map((v) => `${v.id}: ${v.nodes.map((n) => n.target.join(" ")).join(", ")}`)).toEqual([]);
});

test("keyboard: every focusable element shows a ring and is never hidden under the sticky header", async ({ browser }) => {
  for (const width of [375, 1440]) {
    const ctx = await browser.newContext({ viewport: { width, height: 800 } });
    const page = await ctx.newPage();
    for (const route of routes) {
      await page.goto(route);
      const seen = new Set<string>();
      for (let i = 0; i < 80; i++) {
        await page.keyboard.press("Tab");
        const f = await page.evaluate(() => {
          const e = document.activeElement as HTMLElement | null;
          if (!e || e === document.body) return null;
          const s = getComputedStyle(e);
          const r = e.getBoundingClientRect();
          const bar = document.querySelector("header.sticky")!.getBoundingClientRect();
          const exempt = !!e.closest("header.sticky") || s.position === "fixed";
          return {
            id: `${e.tagName} ${(e.textContent || e.getAttribute("aria-label") || e.getAttribute("name") || "").trim().slice(0, 40)}`,
            ring: s.outlineStyle !== "none" && parseFloat(s.outlineWidth) >= 2,
            covered: !exempt && r.top < bar.bottom - 1,
            visible: r.bottom > 0 && r.top < innerHeight,
          };
        });
        if (!f || seen.has(f.id)) break; // back to the start: the whole page was traversed
        seen.add(f.id);
        expect(f.ring, `${route} @${width}: ${f.id} has no focus ring`).toBe(true);
        expect(f.covered, `${route} @${width}: ${f.id} is under the header`).toBe(false);
        expect(f.visible, `${route} @${width}: ${f.id} is off screen`).toBe(true);
      }
      expect(seen.size, route).toBeGreaterThan(3);
    }
    await ctx.close();
  }
});

test("200 % page zoom and 200 % text keep every route without horizontal scroll", async ({ browser }) => {
  for (const [name, viewport, css] of [["zoom", { width: 640, height: 400 }, ""], ["text", { width: 1280, height: 800 }, "html{font-size:200%!important}"]] as const) {
    const ctx = await browser.newContext({ viewport });
    const page = await ctx.newPage();
    for (const route of routes) {
      await page.goto(route);
      if (css) await page.addStyleTag({ content: css });
      await page.evaluate(() => document.fonts.ready);
      expect(await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth), `${name} ${route}`).toBe(0);
    }
    await ctx.close();
  }
});

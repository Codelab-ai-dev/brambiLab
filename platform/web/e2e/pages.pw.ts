// WEB-009 delivery 2: interior public pages in the editorial direction, through Caddy with real
// fixtures — rows instead of cards, the cover in colour and uncropped, a moderate reading width,
// related links that exist, and one heading, no cut words and no horizontal scroll on every route.
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { expect, test, type Page } from "@playwright/test";

const BASE = process.env.BASE_URL ?? "http://localhost:8000";
const run = Date.now().toString(36);
const png = readFileSync(fileURLToPath(new URL("../../api/internal/media/testdata/text.png", import.meta.url)));
const paths = { project: `/es/proyectos/ficha-${run}`, log: `/es/proyectos/ficha-${run}/bitacora/prueba-${run}`, article: `/es/articulos/lectura-${run}` };

async function ownerApi(page: Page) {
  await page.goto("/admin");
  await page.getByRole("link", { name: "Iniciar sesión con GitHub" }).click();
  await page.waitForURL(/\/admin$/);
  const me = await (await page.request.get("/api/v1/auth/me")).json();
  const headers = { "X-CSRF-Token": me.csrf_token, Origin: BASE };
  const call = async (method: string, path: string, data?: unknown) => {
    const r = await page.request.fetch(path, { method, data, headers: { ...headers, "Content-Type": "application/json" } });
    expect(r.ok(), `${method} ${path}: ${r.status()} ${await r.text()}`).toBe(true);
    return r.status() === 204 ? null : r.json();
  };
  const paragraph = (text: string) => ({ type: "paragraph", content: [{ type: "text", text }] });
  return {
    call,
    async upload() {
      const r = await page.request.post(`/api/v1/admin/assets?filename=ficha-${run}.png`, { data: png, headers });
      expect(r.status()).toBe(201);
      const id = (await r.json()).id as string;
      await call("PATCH", `/api/v1/admin/assets/${id}`, { public_enabled: true, downloadable: false });
      return id;
    },
    async publish(kind: string, snapshot: Record<string, unknown>, projectId?: string) {
      const id = (await call("POST", "/api/v1/admin/contents", { kind, locale: "es", ...(projectId ? { project_id: projectId } : {}) })).id as string;
      const long = Array.from({ length: 6 }, (_, i) => paragraph(`Párrafo ${i + 1} de lectura: el texto largo mantiene un ancho cómodo aunque la pantalla sea muy ancha.`));
      await call("POST", `/api/v1/admin/contents/${id}/translations/es/revisions`, { expected_version: 0, kind: "manual", snapshot: { summary: "", ...snapshot, body: { type: "doc", content: long } } });
      const st = await call("GET", `/api/v1/admin/contents/${id}/translations/es/publication`);
      await call("POST", `/api/v1/admin/contents/${id}/translations/es/publication/publish`, { revision_version: 1, expected_editorial_version: st.editorial_version });
      return id;
    },
  };
}

let cover = "";

test.beforeAll(async ({ browser }) => {
  const page = await browser.newPage();
  const api = await ownerApi(page);
  cover = await api.upload();
  const cat = (await api.call("POST", "/api/v1/admin/categories", { slug: `lectura-${run}`, labels: { es: `Lectura ${run}`, en: `Reading ${run}` } })).id;
  const p = await api.publish("project", {
    title: `Ficha ${run}`, slug: `ficha-${run}`, summary: "Plataforma de pruebas", cover_asset_id: cover,
    project_fields: { status: "in_development", technologies: ["ESP32", "MQTT"], objective: "Medir el consumo en marcha." },
  });
  await api.publish("log", { title: `Prueba ${run}`, slug: `prueba-${run}` }, p);
  await api.publish("article", { title: `Lectura ${run}`, slug: `lectura-${run}`, category_id: cat });
  await page.close();
});

test("indexes list editorial rows; the project row shows real facts and a desaturated thumbnail", async ({ page }) => {
  await page.goto("/es/proyectos");
  const row = page.getByRole("article").filter({ has: page.getByRole("heading", { name: `Ficha ${run}` }) });
  await expect(row).toHaveCount(1);
  await expect(row).toContainText("En desarrollo");
  await expect(row).toContainText("ESP32 · MQTT");
  const thumb = row.locator(`img[src="/media/${cover}"]`);
  expect(await thumb.evaluate((el) => getComputedStyle(el).filter)).toContain("grayscale");
  // The count in the header is the API total for this language, never the page length.
  const total = (await (await page.request.get("/api/v1/public/es/contents?kind=project&page_size=1")).json()).total;
  await expect(page.locator("header").nth(1)).toContainText(`${total} proyectos publicados`);
});

test("project page: cover in colour and uncropped, technical sheet, log and related links that exist", async ({ page, request }) => {
  await page.goto(paths.project);
  const img = page.locator(`main img[src="/media/${cover}"]`).first();
  await expect.poll(() => img.evaluate((el: HTMLImageElement) => el.complete && el.naturalWidth > 0)).toBe(true);
  const shown = await img.evaluate((el: HTMLImageElement) => ({ filter: getComputedStyle(el).filter, ratio: el.clientWidth / el.clientHeight, natural: el.naturalWidth / el.naturalHeight }));
  expect(shown.filter).toBe("none");
  expect(Math.abs(shown.ratio - shown.natural)).toBeLessThan(0.02);
  const sheet = page.getByRole("complementary", { name: "Ficha técnica" });
  await expect(sheet).toContainText("Medir el consumo en marcha.");
  await expect(page.getByRole("region", { name: /Bitácora/ })).toContainText(`Prueba ${run}`);
  const related = page.getByRole("navigation", { name: "Seguir explorando" }).getByRole("link");
  await expect(related).toHaveCount(1);
  for (const href of await related.evaluateAll((as) => as.map((a) => a.getAttribute("href")!))) expect((await request.get(href)).status(), href).toBe(200);
});

test("article and log: moderate reading width on paper, and every related link resolves", async ({ page, request }) => {
  await page.setViewportSize({ width: 1920, height: 1000 });
  await page.goto(paths.article);
  const prose = page.locator(".bl-prose > div").first();
  const measure = await prose.evaluate((el) => el.clientWidth / parseFloat(getComputedStyle(el).fontSize));
  expect(measure).toBeLessThanOrEqual(34.5); // 34 em ≈ 70 characters of Plex Sans per line
  expect(await page.locator(".bl-prose").evaluate((el) => getComputedStyle(el.closest(".bl-light")!).backgroundColor)).toBe("rgb(244, 244, 241)");
  const related = page.getByRole("navigation", { name: "Seguir explorando" }).getByRole("link");
  await expect(related).toHaveCount(2); // all articles + its category
  for (const href of await related.evaluateAll((as) => as.map((a) => a.getAttribute("href")!))) expect((await request.get(href)).status(), href).toBe(200);

  await page.goto(paths.log);
  await expect(page.getByRole("link", { name: `← Ficha ${run}` })).toHaveAttribute("href", paths.project);
  await expect(page.getByRole("navigation", { name: "Seguir explorando" }).getByRole("link", { name: /Volver al proyecto/ })).toHaveAttribute("href", paths.project);
});

test("every public route: one h1, no word cut in a heading, no horizontal scroll", async ({ browser }) => {
  const routes = ["/es/proyectos", "/en/projects", "/es/articulos", "/en/articles", "/es/buscar", "/es/buscar?q=ficha", "/en/search", "/es/acerca-de", "/en/about", "/es/contacto", "/en/contact", "/es/no-existe", paths.project, paths.log, paths.article];
  for (const width of [320, 768, 1440]) {
    const ctx = await browser.newContext({ viewport: { width, height: 900 } });
    const page = await ctx.newPage();
    for (const route of routes) {
      await page.goto(route);
      await page.evaluate(() => document.fonts.ready);
      await expect(page.getByRole("heading", { level: 1 }), `${route} @${width}`).toHaveCount(1);
      expect(await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth), `${route} @${width}`).toBe(0);
      const cut = await page.locator("main h1, main h2").evaluateAll((hs) =>
        hs.flatMap((h) => {
          const s = getComputedStyle(h);
          const probe = document.createElement("span");
          probe.style.cssText = `position:absolute;visibility:hidden;white-space:nowrap;font:${s.font};letter-spacing:${s.letterSpacing}`;
          document.body.appendChild(probe);
          const out = (h.textContent ?? "").split(/\s+/).filter(Boolean).filter((w) => {
            probe.textContent = w;
            // Generated test slugs have no spaces and may legitimately wrap anywhere.
            return !/[0-9a-z]{8}/.test(w) && probe.getBoundingClientRect().width > h.clientWidth + 1;
          });
          probe.remove();
          return out;
        }),
      );
      expect(cut, `${route} @${width}`).toEqual([]);
    }
    await ctx.close();
  }
});

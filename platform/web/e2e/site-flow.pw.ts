// WEB-006 end to end through Caddy: site settings from the panel, featured projects with a real
// public cover, drafts that change nothing public, withdrawal and media revocation, a scheduled
// publication reaching listings/search/sitemap, and reduced motion / keyboard on the public site.
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { expect, test, type Browser, type Page } from "@playwright/test";

const BASE = process.env.BASE_URL ?? "http://localhost:8000";
const run = Date.now().toString(36);
const png = readFileSync(fileURLToPath(new URL("../../api/internal/media/testdata/text.png", import.meta.url)));

async function login(page: Page) {
  await page.goto("/admin");
  await page.getByRole("link", { name: "Iniciar sesión con GitHub" }).click();
  await page.waitForURL(/\/admin$/);
}

async function ownerApi(page: Page) {
  const me = await (await page.request.get("/api/v1/auth/me")).json();
  const headers = { "X-CSRF-Token": me.csrf_token, Origin: BASE };
  const call = async (method: string, path: string, body?: unknown) => {
    const r = await page.request.fetch(path, { method, data: body, headers: { ...headers, "Content-Type": "application/json" } });
    expect(r.ok(), `${method} ${path}: ${r.status()} ${await r.text()}`).toBe(true);
    return r.json();
  };
  const doc = (text: string) => ({ type: "doc", content: [{ type: "paragraph", content: [{ type: "text", text }] }] });
  const state = (id: string, l: string) => call("GET", `/api/v1/admin/contents/${id}/translations/${l}/publication`);
  return {
    call,
    async upload(): Promise<string> {
      const r = await page.request.post(`/api/v1/admin/assets?filename=portada-${run}.png`, { data: png, headers });
      expect(r.status()).toBe(201);
      return (await r.json()).id;
    },
    setAsset: (id: string, public_enabled: boolean) => call("PATCH", `/api/v1/admin/assets/${id}`, { public_enabled, downloadable: false }),
    async create(kind: string): Promise<string> {
      return (await call("POST", "/api/v1/admin/contents", { kind, locale: "es" })).id;
    },
    translate: (id: string) => call("POST", `/api/v1/admin/contents/${id}/translations`, { locale: "en" }),
    async save(id: string, l: string, snap: Record<string, unknown>): Promise<number> {
      const tr = await call("GET", `/api/v1/admin/contents/${id}/translations/${l}`);
      const { text, ...rest } = snap;
      await call("POST", `/api/v1/admin/contents/${id}/translations/${l}/revisions`, { expected_version: tr.latest_version, kind: "manual", snapshot: { summary: "", ...rest, body: doc(String(text ?? "Texto")) } });
      return tr.latest_version + 1;
    },
    async publish(id: string, l: string, v: number) {
      await call("POST", `/api/v1/admin/contents/${id}/translations/${l}/publication/publish`, { revision_version: v, expected_editorial_version: (await state(id, l)).editorial_version });
    },
    async withdraw(id: string, l: string) {
      await call("POST", `/api/v1/admin/contents/${id}/translations/${l}/publication/withdraw`, { expected_editorial_version: (await state(id, l)).editorial_version });
    },
    async schedule(id: string, l: string, v: number, local: string) {
      await call("POST", `/api/v1/admin/contents/${id}/translations/${l}/publication/schedule`, {
        revision_version: v, run_at_local: local, time_zone: "America/Mexico_City", expected_editorial_version: (await state(id, l)).editorial_version,
      });
    },
  };
}

function localIn(minutes: number) {
  const at = Date.now() + minutes * 60_000;
  const p = Object.fromEntries(
    new Intl.DateTimeFormat("en-CA", { timeZone: "America/Mexico_City", year: "numeric", month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", hourCycle: "h23" })
      .formatToParts(new Date(at)).map((x) => [x.type, x.value]),
  );
  return `${p.year}-${p.month}-${p.day}T${p.hour}:${p.minute}`;
}

async function anon(browser: Browser, js = false) {
  const ctx = await browser.newContext({ javaScriptEnabled: js });
  return { ctx, page: await ctx.newPage(), request: ctx.request };
}

test("site settings, featured projects, drafts, withdrawal, revoked cover and a scheduled article", async ({ page, browser }) => {
  test.setTimeout(300_000);
  await login(page);
  const api = await ownerApi(page);
  const cover = await api.upload();
  await api.setAsset(cover, true);
  const p = await api.create("project");
  await api.translate(p);
  await api.publish(p, "es", await api.save(p, "es", { title: `Estación ${run}`, slug: `estacion-${run}`, summary: "Estación meteorológica", cover_asset_id: cover, text: "Sensores al aire libre." }));
  await api.publish(p, "en", await api.save(p, "en", { title: `Station ${run}`, slug: `station-${run}`, summary: "Weather station", cover_asset_id: cover, text: "Outdoor sensors." }));
  const q = await api.create("project");
  await api.publish(q, "es", await api.save(q, "es", { title: `Brújula ${run}`, slug: `brujula-${run}`, text: "Sólo en español." }));

  // Panel: an invalid link is explained next to its row and nothing is saved.
  await page.goto("/admin/sitio");
  await page.getByLabel("Presentación (español)").fill(`Laboratorio de prueba ${run}.`);
  await page.getByLabel("Presentación (inglés)").fill(`Test lab ${run}.`);
  await page.getByLabel("Biografía (español)").fill(`Primer párrafo ${run}.\n\nSegundo párrafo.`);
  await page.getByLabel("Correo de contacto").fill("contacto@example.com");
  await page.getByLabel("Texto del enlace 1").fill("GitHub");
  await page.getByLabel("URL del enlace 1").fill("https://github.com/example");
  await page.getByLabel("Texto del enlace 2").fill("Inseguro");
  await page.getByLabel("URL del enlace 2").evaluate((el: HTMLInputElement) => (el.type = "text"));
  await page.getByLabel("URL del enlace 2").fill("http://example.com");
  const rowOf = (title: string) => page.getByRole("listitem").filter({ hasText: title });
  // Independent of earlier runs on the same database: start with no featured selection.
  const checked = page.locator('input[name="featured"]:checked');
  while ((await checked.count()) > 0) await checked.first().uncheck();
  await rowOf(`Estación ${run}`).getByRole("checkbox").check();
  await rowOf(`Estación ${run}`).getByLabel("Orden").fill("2");
  await rowOf(`Brújula ${run}`).getByRole("checkbox").check();
  await rowOf(`Brújula ${run}`).getByLabel("Orden").fill("1");
  await page.getByRole("button", { name: "Guardar cambios públicos" }).click();
  await expect(page.getByRole("alert")).toContainText("Revisa los campos marcados");
  await expect(page.getByText("must be an absolute https:// URL without credentials")).toBeVisible();
  await page.getByLabel("Texto del enlace 2").fill("");
  await page.getByLabel("URL del enlace 2").fill("");
  await page.getByRole("button", { name: "Guardar cambios públicos" }).click();
  await expect(page.getByRole("status").filter({ hasText: "Cambios públicos guardados" })).toBeVisible();

  // Visitors (no JS): featured in the chosen order, only where published; real public cover.
  const v = await anon(browser);
  await v.page.goto("/es");
  await expect(v.page.getByText(`Laboratorio de prueba ${run}.`)).toBeVisible();
  const featured = v.page.getByRole("region", { name: /Proyectos destacados/ }).getByRole("heading", { level: 3 });
  await expect(featured).toHaveText([`Brújula ${run}`, `Estación ${run}`]);
  const cardImg = v.page.getByRole("region", { name: /Proyectos destacados/ }).locator(`img[src="/media/${cover}"]`);
  await expect(cardImg).toHaveCount(1);
  expect((await v.request.get(`/media/${cover}`)).status()).toBe(200);
  await v.page.goto("/en");
  await expect(v.page.getByRole("region", { name: /Featured projects/ }).getByRole("heading", { level: 3 })).toHaveText([`Station ${run}`]);
  const detail = await (await v.request.get(`/es/proyectos/estacion-${run}`)).text();
  expect(detail).toContain(`<meta property="og:image" content="${BASE}/media/${cover}"/>`);
  await v.page.goto("/es/contacto");
  await expect(v.page.getByRole("link", { name: /contacto@example.com/ })).toHaveAttribute("href", "mailto:contacto@example.com");
  await expect(v.page.getByRole("link", { name: "GitHub" })).toHaveAttribute("href", "https://github.com/example");
  await v.page.goto("/es/acerca-de");
  await expect(v.page.getByText(`Primer párrafo ${run}.`)).toBeVisible();
  await expect(v.page.getByText("Segundo párrafo.")).toBeVisible();

  // A draft changes nothing public: home, detail, search.
  await api.save(p, "es", { title: `Borrador oculto ${run}`, slug: `estacion-${run}`, cover_asset_id: cover, text: `Palabra secreta ${run}` });
  await v.page.goto("/es");
  await expect(v.page.locator("body")).not.toContainText(`Borrador oculto ${run}`);
  expect(await (await v.request.get(`/es/proyectos/estacion-${run}`)).text()).not.toContain("Borrador oculto");
  const searchSecret = await (await v.request.get(`/api/v1/public/es/search?q=${encodeURIComponent(`secreta ${run}`)}`)).json();
  expect(searchSecret.total).toBe(0);

  // Withdrawing a featured project removes it from the home at once; the selection stays.
  await api.withdraw(q, "es");
  await v.page.goto("/es");
  await expect(v.page.getByRole("region", { name: /Proyectos destacados/ }).getByRole("heading", { level: 3 })).toHaveText([`Estación ${run}`]);
  await page.goto("/admin/sitio");
  await expect(rowOf(`Brújula ${run}`).getByRole("checkbox")).toBeChecked();
  await expect(rowOf(`Brújula ${run}`)).toContainText("ES no publicado");

  // Revoking the cover: no image in cards, no og:image, and the file is gone for visitors.
  await api.setAsset(cover, false);
  await v.page.goto("/es");
  await expect(v.page.locator(`img[src="/media/${cover}"]`)).toHaveCount(0);
  expect(await (await v.request.get(`/es/proyectos/estacion-${run}`)).text()).not.toContain("og:image");
  expect((await v.request.get(`/media/${cover}`)).status()).toBe(404);

  // A scheduled article appears in listings, search and sitemap once the scheduler runs.
  const a = await api.create("article");
  const av = await api.save(a, "es", { title: `Programada ${run}`, slug: `programada-${run}`, text: `Anemómetro-${run} calibrado.` });
  await api.schedule(a, "es", av, localIn(2));
  expect((await v.request.get(`/es/articulos/programada-${run}`)).status()).toBe(404);
  await expect.poll(async () => (await v.request.get(`/es/articulos/programada-${run}`)).status(), { timeout: 200_000, intervals: [2_000] }).toBe(200);
  await v.page.goto(`/es/buscar?q=${encodeURIComponent(`anemometro-${run}`)}`);
  await expect(v.page.getByRole("article")).toHaveCount(1);
  await v.page.goto("/es/articulos");
  await expect(v.page.getByRole("heading", { name: `Programada ${run}` })).toBeVisible();
  expect(await (await v.request.get("/sitemap.xml")).text()).toContain(`${BASE}/es/articulos/programada-${run}</loc>`);
  await v.ctx.close();
});

test("reduced motion freezes the hero; keyboard users can skip to the content", async ({ browser }) => {
  const trace = async (reducedMotion: "reduce" | "no-preference") => {
    const ctx = await browser.newContext({ reducedMotion });
    const page = await ctx.newPage();
    await page.goto("/es");
    const path = page.locator(".bl-scope path[stroke-width='2']");
    const before = await path.getAttribute("d");
    await page.waitForTimeout(600);
    const after = await path.getAttribute("d");
    await ctx.close();
    return before !== after;
  };
  expect(await trace("reduce")).toBe(false);
  expect(await trace("no-preference")).toBe(true);

  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  await page.goto("/es/proyectos");
  await page.keyboard.press("Tab");
  const skip = page.getByRole("link", { name: "Saltar al contenido" });
  await expect(skip).toBeFocused();
  expect((await skip.boundingBox())?.width ?? 0).toBeGreaterThan(0);
  await page.keyboard.press("Enter");
  await expect(page).toHaveURL(/#main$/);
  // Next Tab goes into the page content, not back to the header.
  await page.keyboard.press("Tab");
  expect(await page.evaluate(() => !!document.activeElement?.closest("#main"))).toBe(true);
  await ctx.close();
});

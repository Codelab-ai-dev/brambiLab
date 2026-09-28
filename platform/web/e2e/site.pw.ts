// WEB-006 public site through Caddy, SSR and Go (compose.e2e.yaml). Fixtures are created through
// the owner API with unique slugs; visitors are anonymous contexts, most with JavaScript disabled.
import { expect, test, type Browser, type Page } from "@playwright/test";

const BASE = process.env.BASE_URL ?? "http://localhost:8000";
const run = Date.now().toString(36);

async function login(page: Page) {
  await page.goto("/admin");
  await page.getByRole("link", { name: "Iniciar sesión con GitHub" }).click();
  await page.waitForURL(/\/admin$/);
}

/** Owner API client bound to the logged-in page (session cookie + CSRF + Origin). */
async function ownerApi(page: Page) {
  const me = await (await page.request.get("/api/v1/auth/me")).json();
  const call = async (method: string, path: string, body?: unknown) => {
    const r = await page.request.fetch(path, { method, data: body, headers: { "X-CSRF-Token": me.csrf_token, Origin: BASE, "Content-Type": "application/json" } });
    expect(r.ok(), `${method} ${path}: ${r.status()} ${await r.text()}`).toBe(true);
    return r.status() === 204 ? null : r.json();
  };
  const doc = (text: string) => ({ type: "doc", content: [{ type: "paragraph", content: [{ type: "text", text }] }] });
  const api = {
    call,
    async create(kind: string, projectId?: string): Promise<string> {
      return (await call("POST", "/api/v1/admin/contents", { kind, locale: "es", ...(projectId ? { project_id: projectId } : {}) })).id;
    },
    async translate(id: string) {
      await call("POST", `/api/v1/admin/contents/${id}/translations`, { locale: "en" });
    },
    async save(id: string, locale: string, snapshot: Record<string, unknown>): Promise<number> {
      const tr = await call("GET", `/api/v1/admin/contents/${id}/translations/${locale}`);
      const body = { summary: "", ...snapshot, body: doc(String(snapshot.text ?? "Texto")) };
      delete (body as { text?: string }).text;
      await call("POST", `/api/v1/admin/contents/${id}/translations/${locale}/revisions`, { expected_version: tr.latest_version, kind: "manual", snapshot: body });
      return tr.latest_version + 1;
    },
    async publish(id: string, locale: string, version: number) {
      const st = await call("GET", `/api/v1/admin/contents/${id}/translations/${locale}/publication`);
      await call("POST", `/api/v1/admin/contents/${id}/translations/${locale}/publication/publish`, { revision_version: version, expected_editorial_version: st.editorial_version });
    },
    async withdraw(id: string, locale: string) {
      const st = await call("GET", `/api/v1/admin/contents/${id}/translations/${locale}/publication`);
      await call("POST", `/api/v1/admin/contents/${id}/translations/${locale}/publication/withdraw`, { expected_editorial_version: st.editorial_version });
    },
    async term(kind: "categories" | "tags", slug: string, es: string, en: string): Promise<string> {
      return (await call("POST", `/api/v1/admin/${kind}`, { slug, labels: { es, en } })).id;
    },
  };
  return api;
}

async function visitor(browser: Browser, js = false) {
  const ctx = await browser.newContext({ javaScriptEnabled: js });
  return { ctx, page: await ctx.newPage(), request: ctx.request };
}

const noFollow = { maxRedirects: 0 };

test("project, log and article: SSR without JS, SEO tags and language switch", async ({ page, browser }) => {
  await login(page);
  const api = await ownerApi(page);
  const p = await api.create("project");
  await api.translate(p);
  await api.publish(p, "es", await api.save(p, "es", { title: `Rover ${run}`, slug: `rover-${run}`, summary: "Robot de pruebas", text: "Cuerpo del proyecto en español.", project_fields: { status: "in_development", technologies: ["ESP32"] } }));
  await api.publish(p, "en", await api.save(p, "en", { title: `Rover ${run} EN`, slug: `rover-${run}-en`, summary: "Test robot", text: "Project body in English." }));
  const l = await api.create("log", p);
  await api.publish(l, "es", await api.save(l, "es", { title: `Día 1 ${run}`, slug: `dia-1-${run}`, text: "Primer avance." }));
  const a = await api.create("article");
  await api.publish(a, "es", await api.save(a, "es", { title: `Nota ${run}`, slug: `nota-${run}`, text: "Sólo en español." }));
  // A newer draft must never show publicly, for anyone.
  await api.save(p, "es", { title: `Borrador secreto ${run}`, slug: `rover-${run}`, text: "Texto del borrador." });

  const v = await visitor(browser);
  const projectPath = `/es/proyectos/rover-${run}`;
  const res = await v.request.get(projectPath);
  expect(res.status()).toBe(200);
  expect(res.headers()["cache-control"]).toBe("no-store");
  const html = await res.text();
  // Initial HTML: body, metadata and reciprocal hreflang, all absolute from PUBLIC_ORIGIN.
  expect(html).toContain("Cuerpo del proyecto en español.");
  expect(html).toContain(`<title>Rover ${run} · BrambiLab</title>`);
  expect(html).toContain(`<link rel="canonical" href="${BASE}${projectPath}"/>`);
  expect(html).toMatch(new RegExp(`<link rel="alternate" hrefLang="en" href="${BASE}/en/projects/rover-${run}-en"/>`));
  expect(html).toContain('<meta name="description" content="Robot de pruebas"/>');
  expect(html).not.toContain("secreto");
  expect(html).not.toContain("/api/v1/admin");
  expect(html).not.toContain("api:8080");

  await v.page.goto(projectPath);
  await expect(v.page.getByRole("heading", { level: 1 })).toHaveText(`Rover ${run}`);
  await expect(v.page.getByRole("region", { name: /Bitácora/ })).toContainText(`Día 1 ${run}`);
  await expect(v.page.locator("html")).toHaveAttribute("lang", "es");
  // Switch keeps the content identity: same project in English, by its own slug.
  await v.page.getByRole("link", { name: "Read this page in English" }).click();
  await expect(v.page).toHaveURL(`${BASE}/en/projects/rover-${run}-en`);
  await expect(v.page.locator("html")).toHaveAttribute("lang", "en");
  await expect(v.page.getByRole("heading", { level: 1 })).toHaveText(`Rover ${run} EN`);

  // The log only exists in Spanish: English shows the project's empty log, never a fake page.
  await expect(v.page.getByText("This project has no progress published in English yet.")).toBeVisible();
  await v.page.goto(`/es/proyectos/rover-${run}/bitacora/dia-1-${run}`);
  await expect(v.page.getByRole("link", { name: `← Rover ${run}` })).toBeVisible();
  const sw = v.page.locator("[data-switch]");
  await expect(sw).toHaveAttribute("data-switch", "index");
  await expect(sw).toHaveAttribute("href", "/en/projects");
  await expect(v.page.locator('[data-translation="missing"]')).toBeVisible();
  expect(await v.page.locator('link[rel="alternate"][hreflang]').count()).toBe(0);

  // The owner, logged in, sees the same published page (the SSR never forwards the session).
  await page.goto(projectPath);
  await expect(page.getByRole("heading", { level: 1 })).toHaveText(`Rover ${run}`);
  await expect(page.locator("body")).not.toContainText("secreto");
  await v.ctx.close();
});

test("aliases answer one 301 to the canonical page; withdrawn pages are 404 everywhere", async ({ page, browser }) => {
  await login(page);
  const api = await ownerApi(page);
  const p = await api.create("project");
  await api.publish(p, "es", await api.save(p, "es", { title: "Brazo", slug: `brazo-${run}`, text: "x" }));
  const l = await api.create("log", p);
  await api.publish(l, "es", await api.save(l, "es", { title: "Prueba", slug: `prueba-${run}`, text: "x" }));
  // Rename both the project and the log.
  await api.publish(p, "es", await api.save(p, "es", { title: "Brazo", slug: `brazo-v2-${run}`, text: "x" }));
  await api.publish(l, "es", await api.save(l, "es", { title: "Prueba", slug: `prueba-v2-${run}`, text: "x" }));

  const { ctx, request } = await visitor(browser);
  const target = `/es/proyectos/brazo-v2-${run}/bitacora/prueba-v2-${run}`;
  for (const old of [`/es/proyectos/brazo-${run}/bitacora/prueba-${run}`, `/es/proyectos/brazo-${run}/bitacora/prueba-v2-${run}`, `/es/proyectos/brazo-v2-${run}/bitacora/prueba-${run}`]) {
    const r = await request.get(old, noFollow);
    expect(r.status(), old).toBe(301);
    expect(r.headers()["location"], old).toBe(target);
  }
  const r = await request.get(`/es/proyectos/brazo-${run}`, noFollow);
  expect(r.status()).toBe(301);
  expect(r.headers()["location"]).toBe(`/es/proyectos/brazo-v2-${run}`);
  expect((await request.get(target, noFollow)).status()).toBe(200);

  // Withdrawn: 404 on the page and on its aliases, and gone from the sitemap.
  await api.withdraw(l, "es");
  expect((await request.get(target, noFollow)).status()).toBe(404);
  expect((await request.get(`/es/proyectos/brazo-${run}/bitacora/prueba-${run}`, noFollow)).status()).toBe(404);
  const sitemap = await (await request.get("/sitemap.xml")).text();
  expect(sitemap).toContain(`<loc>${BASE}/es/proyectos/brazo-v2-${run}</loc>`);
  expect(sitemap).not.toContain(`brazo-${run}<`);
  expect(sitemap).not.toContain(`prueba-v2-${run}`);
  await ctx.close();
});

test("filters, pagination and search work as plain GET forms without JavaScript", async ({ page, browser }) => {
  await login(page);
  const api = await ownerApi(page);
  const cat = await api.term("categories", `cat-${run}`, `Categoría ${run}`, `Category ${run}`);
  const tag = await api.term("tags", `tag-${run}`, `Etiqueta ${run}`, `Tag ${run}`);
  for (let i = 1; i <= 14; i++) {
    const a = await api.create("article");
    const tagged = i % 2 === 0;
    await api.publish(a, "es", await api.save(a, "es", {
      title: `Filtro ${run} ${String(i).padStart(2, "0")}${i === 3 ? ' <img src=x onerror="alert(1)">' : ""}`,
      slug: `filtro-${run}-${i}`,
      text: i === 5 ? `Medición de telemetría-${run} con osciloscopio.` : "Texto de relleno.",
      category_id: cat,
      tag_ids: tagged ? [tag] : [],
    }));
  }
  const { ctx, page: v } = await visitor(browser);
  await v.goto("/es/articulos");
  await v.getByLabel("Categoría").selectOption(`cat-${run}`);
  await v.getByLabel("Etiqueta").selectOption(`tag-${run}`);
  await v.getByRole("button", { name: "Aplicar" }).click();
  await expect(v).toHaveURL(new RegExp(`/es/articulos\\?category=cat-${run}&tag=tag-${run}$`));
  await expect(v.locator('meta[name="robots"]')).toHaveAttribute("content", "noindex, follow");
  await expect(v.getByRole("article")).toHaveCount(7);
  // Unfiltered: 12 per page; the next page keeps no filters, a filtered next page keeps them.
  await v.goto(`/es/articulos?category=cat-${run}`);
  await expect(v.getByRole("article")).toHaveCount(12);
  await v.getByRole("link", { name: "Siguientes →" }).click();
  await expect(v).toHaveURL(new RegExp(`category=cat-${run}&page=2`));
  await expect(v.getByRole("article")).toHaveCount(2);
  await v.goBack();
  await expect(v.getByRole("article")).toHaveCount(12);
  // Author text that looks like HTML is shown as text.
  expect(await v.locator("img[onerror]").count()).toBe(0);
  await expect(v.getByRole("link", { name: /Filtro .* 03 <img src=x onerror="alert\(1\)">/ })).toBeVisible();

  await v.goto("/es/buscar");
  await expect(v.getByRole("status")).toContainText("Escribe una o más palabras");
  await v.getByLabel("Texto a buscar").fill(`telemetria-${run}`);
  await v.getByRole("button", { name: "Buscar" }).click();
  await expect(v).toHaveURL(new RegExp(`/es/buscar\\?q=telemetria-${run}`));
  await expect(v.getByRole("article")).toHaveCount(1);
  await expect(v.getByRole("article").locator("mark").first()).toBeVisible();
  await expect(v.locator('meta[name="robots"]')).toHaveAttribute("content", "noindex, follow");
  expect((await v.request.get("/es/buscar?q=x&page=0")).status()).toBe(400);
  await ctx.close();
});

test("robots.txt and sitemap.xml are well-formed and absolute", async ({ browser }) => {
  const { ctx, page: v, request } = await visitor(browser, true);
  const robots = await (await request.get("/robots.txt")).text();
  expect(robots).toContain("Disallow: /admin");
  expect(robots).toContain(`Sitemap: ${BASE}/sitemap.xml`);
  const res = await request.get("/sitemap.xml");
  expect(res.headers()["content-type"]).toContain("application/xml");
  const xml = await res.text();
  await v.goto("/es");
  const parsed = await v.evaluate((text) => {
    const d = new DOMParser().parseFromString(text, "application/xml");
    const err = d.getElementsByTagName("parsererror").length;
    const locs = [...d.getElementsByTagName("loc")].map((n) => n.textContent ?? "");
    return { err, locs };
  }, xml);
  expect(parsed.err).toBe(0);
  expect(parsed.locs.length).toBeGreaterThan(0);
  for (const loc of parsed.locs) {
    expect(loc.startsWith(`${BASE}/`), loc).toBe(true);
    expect(loc).not.toMatch(/\/admin|\/api\/|buscar|search/);
  }
  await ctx.close();
});

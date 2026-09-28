// WEB-005 publication flows in a real browser through Caddy, SSR and Go (compose.e2e.yaml):
// manual publish with real media, what anonymous visitors get, withdrawal, and a scheduled
// publication executed by the in-process scheduler with the real clock.
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { expect, test, type APIRequestContext, type Browser, type Page } from "@playwright/test";

const fixtures = fileURLToPath(new URL("../../api/internal/media/testdata/", import.meta.url));
const shots = process.env.SHOTS_DIR;
const shot = async (page: Page, name: string) => {
  if (shots) await page.screenshot({ path: `${shots}/${name}.png`, fullPage: true });
};

async function login(page: Page) {
  await page.goto("/admin");
  await page.getByRole("link", { name: "Iniciar sesión con GitHub" }).click();
  await page.waitForURL(/\/admin$/);
}

/** Accepts every confirmation and records its text (the zone must be visible in them). */
function acceptDialogs(page: Page) {
  const messages: string[] = [];
  page.on("dialog", (d) => {
    messages.push(d.message());
    void d.accept();
  });
  return messages;
}

async function newArticle(page: Page, title: string) {
  await page.goto("/admin/contenidos/nuevo?tipo=article");
  await page.getByRole("button", { name: "Crear y empezar a escribir" }).click();
  await page.waitForURL(/\/es$/);
  await page.locator(".ProseMirror").waitFor();
  await page.getByLabel("Título", { exact: true }).fill(title);
  return page.getByLabel("Slug (URL)").inputValue();
}

const saveNow = async (page: Page, v: number) => {
  await page.getByRole("button", { name: "Guardar revisión" }).click();
  await expect(page.locator("[data-save-status]")).toHaveAttribute("data-save-status", "saved", { timeout: 25_000 });
  await expect(page.locator("[data-save-status]")).toContainText(`v${v}`);
};

async function anonymous(browser: Browser): Promise<APIRequestContext> {
  const ctx = await browser.newContext();
  return ctx.request;
}

/** "YYYY-MM-DDTHH:MM" in America/Mexico_City, `minutes` from now. */
function localIn(minutes: number): { local: string; at: number } {
  const at = Date.now() + minutes * 60_000;
  const p = Object.fromEntries(
    new Intl.DateTimeFormat("en-CA", { timeZone: "America/Mexico_City", year: "numeric", month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", hourCycle: "h23" })
      .formatToParts(new Date(at))
      .map((x) => [x.type, x.value]),
  );
  const local = `${p.year}-${p.month}-${p.day}T${p.hour}:${p.minute}`;
  // The instant the scheduler targets: that wall-clock minute (UTC-6, no DST).
  return { local, at: Date.parse(`${local}:00-06:00`) };
}

test("publish with real media, anonymous access, unsaved buffer and withdrawal", async ({ page, browser }) => {
  const dialogs = acceptDialogs(page);
  await login(page);
  const slug = await newArticle(page, `Placa publicada ${Date.now()}`);
  await page.getByRole("button", { name: "Imagen", exact: true }).click();
  const picker = page.getByRole("dialog");
  await picker.locator('input[type="file"]').setInputFiles(join(fixtures, "text.png"));
  await expect(picker.locator('[data-upload-state="done"]').first()).toBeVisible({ timeout: 20_000 });
  await picker.getByRole("button", { name: /^Elegir text\.png/ }).first().click();
  await saveNow(page, 1);

  const panel = page.locator('[data-publication="es"]');
  await expect(panel.locator("[data-status]")).toHaveAttribute("data-status", "unpublished");

  // The image is private: publishing is blocked and nothing is enabled automatically.
  await panel.getByRole("button", { name: "Publicar v1" }).click();
  await expect(panel.getByRole("alert")).toContainText("Esta versión todavía no se puede publicar");
  await expect(panel.getByRole("alert")).toContainText("no está marcado como público");
  const editorUrl = page.url();
  await panel.getByRole("alert").getByRole("link").first().click();
  await page.getByLabel("Permitir que sea público").check();
  await page.getByRole("button", { name: "Guardar", exact: true }).click();
  await expect(page.getByRole("status").filter({ hasText: "Guardado." })).toBeVisible();
  await page.goto(editorUrl);

  await panel.getByRole("button", { name: "Publicar v1" }).click();
  await expect(panel.getByRole("status").filter({ hasText: "Versión 1 publicada." })).toBeVisible();
  await expect(panel.locator("[data-status]")).toHaveAttribute("data-status", "published");
  const route = `/api/v1/public/es/articles/${slug}`;
  await expect(panel.getByRole("link", { name: /^Ver publicación/ })).toHaveAttribute("href", `/es/articulos/${slug}`);
  expect(dialogs.at(-1)).toContain("¿Publicar ahora la versión 1 en Español?");
  await shot(page, "publication-published");

  // Anonymous visitors see the published revision; responses are never cached.
  const anon = await anonymous(browser);
  const pub = await anon.get(route);
  expect(pub.status()).toBe(200);
  expect(pub.headers()["cache-control"]).toBe("no-store");
  const body = (await pub.json()) as { title: string; body: { content: { type: string; attrs?: { assetId?: string } }[] } };
  const assetId = body.body.content.find((n) => n.type === "image")?.attrs?.assetId;
  expect(assetId).toBeTruthy();
  const media = await anon.get(`/media/${assetId}`);
  expect(media.status()).toBe(200);
  const etag = media.headers()["etag"];

  // Unsaved edits are never presented as published, and cannot be published.
  const published = body.title;
  await page.getByLabel("Título", { exact: true }).fill(`${published} (borrador)`);
  await expect(panel.getByRole("button", { name: /^Publicar v1/ })).toBeDisabled();
  await expect(panel).toContainText("Hay cambios sin guardar");
  expect(((await (await anon.get(route)).json()) as { title: string }).title).toBe(published);
  await saveNow(page, 2);
  expect(((await (await anon.get(route)).json()) as { title: string }).title).toBe(published);
  await expect(panel.getByRole("button", { name: "Publicar v2 ahora" })).toBeEnabled();

  // The history shows which version is public and publishes an explicit one.
  await page.goto(`${editorUrl}/historial`);
  await expect(page.getByRole("row", { name: /v1/ })).toContainText("pública");
  await page.getByRole("button", { name: "Publicar la versión 2" }).click();
  await expect(page.getByRole("status").filter({ hasText: "Versión 2 publicada." })).toBeVisible();
  await expect(page.getByRole("row", { name: /v2/ })).toContainText("pública");
  expect(((await (await anon.get(route)).json()) as { title: string }).title).toBe(`${published} (borrador)`);

  // Withdraw: the page and its files disappear for visitors, revalidation included.
  await page.goto(editorUrl);
  await panel.getByRole("button", { name: "Retirar" }).click();
  await expect(panel.locator("[data-status]")).toHaveAttribute("data-status", "withdrawn");
  expect(dialogs.at(-1)).toContain("¿Retirar la publicación en Español?");
  expect((await anon.get(route)).status()).toBe(404);
  const variants: Record<string, string>[] = [{}, { Range: "bytes=0-10" }, { "If-None-Match": etag }];
  for (const headers of variants) {
    const r = await anon.get(`/media/${assetId}`, { headers });
    expect(r.status(), JSON.stringify(headers)).toBe(404);
  }
  expect((await anon.head(`/media/${assetId}`)).status()).toBe(404);
  await shot(page, "publication-withdrawn");
});

test("a scheduled publication runs within a minute of its local time", async ({ page, browser }) => {
  test.setTimeout(240_000);
  const dialogs = acceptDialogs(page);
  await login(page);
  const slug = await newArticle(page, `Programado ${Date.now()}`);
  await saveNow(page, 1);
  const panel = page.locator('[data-publication="es"]');

  // Schedule, then cancel: nothing gets published.
  await panel.getByRole("button", { name: "Programar…" }).click();
  await expect(panel.getByLabel("Fecha y hora (America/Mexico_City)")).toBeVisible();
  const later = localIn(60 * 24);
  await panel.getByLabel("Fecha y hora (America/Mexico_City)").fill(later.local);
  await panel.getByRole("button", { name: "Programar", exact: true }).click();
  await expect(panel).toContainText("Programada: v1 para el");
  await expect(panel).toContainText("(America/Mexico_City)");
  expect(dialogs.at(-1)).toMatch(/America\/Mexico_City/);
  await panel.getByRole("button", { name: "Cancelar programación" }).click();
  await expect(panel.getByRole("status").filter({ hasText: "Programación cancelada." })).toBeVisible();
  await expect(panel).not.toContainText("Programada: v1");

  // Schedule for the next-but-one minute and wait for the scheduler (15 s tick).
  const soon = localIn(2);
  await panel.getByRole("button", { name: "Programar…" }).click();
  await panel.getByLabel("Fecha y hora (America/Mexico_City)").fill(soon.local);
  await panel.getByRole("button", { name: "Programar", exact: true }).click();
  await expect(panel).toContainText("Programada: v1 para el");
  await shot(page, "publication-scheduled");

  const anon = await anonymous(browser);
  const route = `/api/v1/public/es/articles/${slug}`;
  expect((await anon.get(route)).status()).toBe(404);
  await expect
    .poll(async () => (await anon.get(route)).status(), { timeout: 200_000, intervals: [2_000] })
    .toBe(200);
  const lag = Date.now() - soon.at;
  expect(lag, "published within 60 s of the scheduled minute").toBeLessThan(60_000);

  await page.reload();
  await expect(panel.locator("[data-status]")).toHaveAttribute("data-status", "published");
  await page.goto(`${page.url()}/historial`);
  await expect(page.getByRole("region", { name: "Programaciones" })).toContainText("Publicada · v1");
  await expect(page.getByRole("region", { name: "Programaciones" })).toContainText("cancelada a mano");
});

// WEB-004 media flows in a real browser through Caddy, SSR and Go (compose.e2e.yaml).
// The video fixture is VP9 in MP4: Playwright's Chromium has no proprietary H.264 decoder.
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { expect, test, type Page } from "@playwright/test";

const fixtures = fileURLToPath(new URL("../../api/internal/media/testdata/", import.meta.url));
const file = (name: string) => join(fixtures, name);
const shots = process.env.SHOTS_DIR;
const shot = async (page: Page, name: string) => {
  if (shots) await page.screenshot({ path: `${shots}/${name}.png`, fullPage: true });
};

async function login(page: Page) {
  await page.goto("/admin");
  await page.getByRole("link", { name: "Iniciar sesión con GitHub" }).click();
  await page.waitForURL(/\/admin$/);
}

async function newArticle(page: Page) {
  await page.goto("/admin/contenidos/nuevo?tipo=article");
  await page.getByRole("button", { name: "Crear y empezar a escribir" }).click();
  await page.waitForURL(/\/es$/);
  await page.locator(".ProseMirror").waitFor();
  await page.getByLabel("Título", { exact: true }).fill(`Artículo con medios ${Date.now()}`);
  return page.url();
}

const saved = async (page: Page, v: number) => {
  await expect(page.locator("[data-save-status]")).toHaveAttribute("data-save-status", "saved", { timeout: 25_000 });
  await expect(page.locator("[data-save-status]")).toContainText(`v${v}`);
};

// Pick an existing library asset, or upload it from the picker when missing.
async function pickOrUpload(page: Page, fixture: string) {
  const dialog = page.getByRole("dialog");
  await dialog.locator('input[type="file"]').setInputFiles(file(fixture));
  await expect(dialog.locator('[data-upload-state="done"]').first()).toBeVisible({ timeout: 20_000 });
  await dialog.getByRole("button", { name: new RegExp(`^Elegir ${fixture}`) }).first().click();
  await expect(dialog).toBeHidden();
}

test("library upload stores a sanitized, correctly rotated image", async ({ page }) => {
  await login(page);
  await page.getByRole("navigation", { name: "Panel" }).getByRole("link", { name: "Medios" }).click();
  await page.locator('input[type="file"]').setInputFiles(file("rotated.jpg"));
  await expect(page.locator('[data-upload-state="done"]')).toBeVisible({ timeout: 20_000 });
  // The queue links to the exact file just uploaded.
  await page.getByRole("link", { name: "Ver archivo rotated.jpg" }).click();
  const img = page.getByRole("region", { name: "Vista previa" }).locator("img");
  await expect(img).toBeVisible();
  // Stored 64×32 with EXIF orientation 6 → the browser receives real 32×64 pixels.
  const size = await img.evaluate((el: HTMLImageElement) => ({ w: el.naturalWidth, h: el.naturalHeight }));
  expect(size).toEqual({ w: 32, h: 64 });
  await expect(page.getByText("Ninguna revisión lo usa todavía.")).toBeVisible();
  await shot(page, "media-detail");
});

test("insert image, video with poster and download; save, reload, preview and play", async ({ page, browser }) => {
  await login(page);
  const editorUrl = await newArticle(page);
  const editor = page.locator(".ProseMirror");

  await page.getByRole("button", { name: "Imagen", exact: true }).click();
  await pickOrUpload(page, "text.png");
  await expect(editor.locator('[data-bl-node="image"] img')).toBeVisible();

  await editor.press("End");
  await page.getByRole("button", { name: "Vídeo", exact: true }).click();
  await pickOrUpload(page, "vp9.mp4");
  // Select the video node and give it a poster and caption.
  await editor.locator('[data-bl-node="video"]').click();
  const form = page.getByRole("form", { name: "Medio seleccionado" });
  await form.getByLabel("Pie (opcional)").fill("Prueba con ruedas levantadas");
  await form.getByRole("button", { name: "Elegir póster" }).click();
  await pickOrUpload(page, "rotated.jpg");
  await form.getByRole("button", { name: "Aplicar" }).click();
  await expect(editor.locator('[data-bl-node="video"] img')).toBeVisible();

  await page.getByRole("button", { name: "Descarga", exact: true }).click();
  await pickOrUpload(page, "doc.pdf");

  await page.getByRole("button", { name: "Elegir portada" }).click();
  await pickOrUpload(page, "text.png");

  await page.getByRole("button", { name: "Guardar revisión" }).click();
  await saved(page, 1);
  await shot(page, "editor-with-media");

  await page.reload();
  await expect(editor.locator('[data-bl-node="image"]')).toBeVisible();
  await expect(editor.locator('[data-bl-node="video"]')).toContainText("Prueba con ruedas levantadas");
  await expect(editor.locator('[data-bl-node="download"]')).toContainText("doc.pdf");

  await page.goto(`${editorUrl}/v/1`);
  const article = page.locator("article");
  const images = article.locator("img");
  await expect(images).toHaveCount(2); // cover + body image
  for (const img of await images.all()) {
    await expect.poll(() => img.evaluate((el: HTMLImageElement) => el.complete && el.naturalWidth > 0)).toBe(true);
  }
  await expect(article.locator("figure:has(video) figcaption")).toHaveText("Prueba con ruedas levantadas");
  const video = article.locator("video");
  // Playback and seeking go through Range requests (206) to Go via Caddy.
  const seek = await video.evaluate(async (el: HTMLVideoElement) => {
    await new Promise<void>((ok, fail) => {
      if (el.readyState >= 1) return ok();
      el.onloadedmetadata = () => ok();
      el.onerror = () => fail(new Error(`media error ${el.error?.code}`));
    });
    el.muted = true;
    await el.play();
    el.currentTime = 3;
    await new Promise<void>((ok) => (el.onseeked = () => ok()));
    return { duration: el.duration, time: el.currentTime, error: el.error?.code ?? null };
  });
  expect(seek.error).toBeNull();
  expect(seek.duration).toBeGreaterThan(3);
  expect(seek.time).toBeGreaterThanOrEqual(2.9);
  const downloadHref = await article.getByRole("link", { name: /doc\.pdf/ }).getAttribute("href");
  expect(downloadHref).toMatch(/^\/media\/[0-9a-f-]+\/download$/);
  const dl = await page.request.get(downloadHref!);
  expect(dl.status()).toBe(200);
  expect(dl.headers()["content-disposition"]).toMatch(/^attachment;/);
  await shot(page, "preview-with-media");

  // Anonymous visitors get nothing: the content is a draft and the files are private.
  const anon = await browser.newContext();
  for (const src of [await images.first().getAttribute("src"), downloadHref, await video.getAttribute("src")]) {
    const r = await anon.request.get(src!, { headers: { Range: "bytes=0-10" } });
    expect(r.status(), src!).toBe(404);
  }
  await anon.close();
});

test("pasting an image uploads it and inserts it, without data URLs", async ({ page }) => {
  await login(page);
  await newArticle(page);
  const editor = page.locator(".ProseMirror");
  await editor.click();
  const bytes = [...readFileSync(file("text.png"))];
  await editor.evaluate((el, data) => {
    const dt = new DataTransfer();
    dt.items.add(new File([new Uint8Array(data)], "pegada.png", { type: "image/png" }));
    el.dispatchEvent(new ClipboardEvent("paste", { clipboardData: dt, bubbles: true, cancelable: true }));
  }, bytes);
  await expect(editor.locator('[data-bl-node="image"] img')).toBeVisible({ timeout: 20_000 });
  expect(await editor.innerHTML()).not.toContain("data:image");
});

test("a slow upload can be cancelled and retried", async ({ page }) => {
  await login(page);
  await page.goto("/admin/medios");
  let delay = true;
  await page.route("**/api/v1/admin/assets?filename=*", async (route) => {
    if (delay) await new Promise((r) => setTimeout(r, 5000));
    await route.continue().catch(() => undefined);
  });
  await page.locator('input[type="file"]').setInputFiles(file("doc.pdf"));
  await page.getByRole("button", { name: /^Cancelar la subida de doc\.pdf/ }).click();
  await expect(page.locator('[data-upload-state="cancelled"]')).toContainText("Subida cancelada.");
  delay = false;
  await page.getByRole("button", { name: /^Reintentar doc\.pdf/ }).click();
  await expect(page.locator('[data-upload-state="done"]')).toBeVisible({ timeout: 20_000 });
});

test("importing Markdown with an unknown asset explains it and keeps the buffer", async ({ page }) => {
  await login(page);
  await newArticle(page);
  await page.getByRole("button", { name: "Importar Markdown" }).click();
  const dialog = page.getByRole("dialog", { name: "Importar Markdown" });
  await dialog.getByLabel("…o pega el texto").fill('Texto importado.\n\n::image{asset="00000000-0000-4000-8000-000000000042" alt="Inexistente"}\n');
  await dialog.getByRole("button", { name: "Analizar" }).click();
  await dialog.getByRole("button", { name: "Reemplazar el cuerpo actual" }).click();
  await page.getByRole("button", { name: "Guardar revisión" }).click();
  await expect(page.locator("[data-save-status]")).toHaveAttribute("data-save-status", "error", { timeout: 20_000 });
  await expect(page.getByText(/no existe en la biblioteca/)).toBeVisible();
  await expect(page.locator(".ProseMirror")).toContainText("Texto importado.");
});

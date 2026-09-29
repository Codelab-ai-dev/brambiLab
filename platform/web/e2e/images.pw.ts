// #41: document images as compact cards with an accessible viewer, through Caddy with real files.
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { expect, test, type Page } from "@playwright/test";

const BASE = process.env.BASE_URL ?? "http://localhost:8000";
const run = Date.now().toString(36);
const fixture = (n: string) => readFileSync(fileURLToPath(new URL(`../../api/internal/media/testdata/${n}`, import.meta.url)));
let path = "";
const ids: Record<string, string> = {};

async function login(page: Page) {
  await page.goto("/admin");
  await page.getByRole("link", { name: "Iniciar sesión con GitHub" }).click();
  await page.waitForURL(/\/admin$/);
}

test.beforeAll(async ({ browser }) => {
  const page = await browser.newPage();
  await login(page);
  const me = await (await page.request.get("/api/v1/auth/me")).json();
  const h = { "X-CSRF-Token": me.csrf_token, Origin: BASE };
  const call = async (method: string, url: string, data?: unknown) => {
    const r = await page.request.fetch(url, { method, data, headers: { ...h, "Content-Type": "application/json" } });
    expect(r.ok(), `${method} ${url}: ${await r.text()}`).toBe(true);
    return r.json();
  };
  for (const [key, file] of [["vertical", "rotated.jpg"], ["horizontal", "text.png"], ["alone", "exif.webp"]] as const) {
    const r = await page.request.post(`/api/v1/admin/assets?filename=${key}-${run}.${file.split(".")[1]}`, { data: fixture(file), headers: h });
    expect(r.status()).toBe(201);
    ids[key] = (await r.json()).id;
    await call("PATCH", `/api/v1/admin/assets/${ids[key]}`, { public_enabled: true, downloadable: false });
  }
  const p = (text: string) => ({ type: "paragraph", content: [{ type: "text", text }] });
  const img = (id: string, alt: string, caption?: string) => ({ type: "image", attrs: { assetId: id, alt, ...(caption ? { caption } : {}) } });
  const id = (await call("POST", "/api/v1/admin/contents", { kind: "article", locale: "es" })).id;
  await call("POST", `/api/v1/admin/contents/${id}/translations/es/revisions`, {
    expected_version: 0, kind: "manual",
    snapshot: { title: `Galería ${run}`, slug: `galeria-${run}`, body: { type: "doc", content: [
      p("Texto inicial del artículo."),
      img(ids.vertical, "Placa en vertical", "Placa de control, vista vertical"),
      img(ids.horizontal, "Etiqueta en horizontal"),
      p("Texto intermedio que separa los grupos."),
      img(ids.alone, "Imagen aislada"),
    ] } },
  });
  const st = await call("GET", `/api/v1/admin/contents/${id}/translations/es/publication`);
  await call("POST", `/api/v1/admin/contents/${id}/translations/es/publication/publish`, { revision_version: 1, expected_editorial_version: st.editorial_version });
  path = `/es/articulos/galeria-${run}`;
  await page.close();
});

test("cards are compact, complete and grouped only when consecutive", async ({ page }) => {
  await page.goto(path);
  const galleries = page.locator("[data-gallery]");
  await expect(galleries).toHaveCount(2);
  await expect(galleries.nth(0)).toHaveAttribute("data-gallery", "2");
  await expect(galleries.nth(1)).toHaveAttribute("data-gallery", "1");
  // The intermediate text stays between the two groups.
  const order = await page.locator("article p, [data-gallery]").evaluateAll((els) => els.map((e) => (e.hasAttribute("data-gallery") ? "G" : e.textContent?.slice(0, 12))));
  expect(order.filter((x) => x === "G" || x === "Texto inicia" || x === "Texto interm")).toEqual(["Texto inicia", "G", "Texto interm", "G"]);
  for (const card of await page.locator("[data-image-card]").all()) {
    const box = (await card.boundingBox())!;
    expect(box.width).toBeLessThanOrEqual(321);
    const area = (await card.locator("a").first().boundingBox())!;
    expect(Math.round(area.height)).toBe(220);
    // object-fit: contain: the whole image is visible inside the area (no cropping).
    const fit = await card.locator("img").evaluate((el: HTMLImageElement) => getComputedStyle(el).objectFit);
    expect(fit).toBe("contain");
  }
  await expect(page.getByText("Placa de control, vista vertical")).toBeVisible();
  await expect(page.locator("[data-image-card]").first()).toContainText("FIG. 01");
});

test("the viewer opens the right image, closes three ways and returns focus", async ({ page }) => {
  await page.goto(path);
  const cards = page.locator("[data-image-card] a");
  const viewer = page.locator("dialog[data-viewer]");
  const shown = viewer.locator("[data-viewer-image]");

  // Mouse: second card opens the horizontal image; the page scroll is locked.
  await cards.nth(1).click();
  await expect(viewer).toBeVisible();
  await expect(shown).toHaveAttribute("src", `/media/${ids.horizontal}`);
  await expect(shown).toHaveAttribute("alt", "Etiqueta en horizontal");
  expect(await page.evaluate(() => document.body.style.overflow)).toBe("hidden");
  await expect(viewer.getByRole("button", { name: /Cerrar/ })).toBeFocused();
  await expect(viewer.getByRole("link", { name: /Abrir original/ })).toHaveAttribute("href", `/media/${ids.horizontal}`);
  await expect(viewer).toContainText("2 de 2"); // accessible text; visible as "2/2"
  await expect(viewer).toContainText("2/2");
  // Arrows move within the group; a click on the image does not close.
  await page.keyboard.press("ArrowLeft");
  await expect(shown).toHaveAttribute("src", `/media/${ids.vertical}`);
  await expect(viewer).toContainText("Placa de control, vista vertical");
  await shown.click();
  await expect(viewer).toBeVisible();
  // Escape closes and focus returns to the card that opened it; the scroll is restored.
  await page.keyboard.press("Escape");
  await expect(viewer).toHaveCount(0);
  await expect(cards.nth(1)).toBeFocused();
  expect(await page.evaluate(() => document.body.style.overflow)).toBe("");

  // Keyboard: Enter on a card opens it; the close button closes.
  await cards.nth(0).focus();
  await page.keyboard.press("Enter");
  await expect(shown).toHaveAttribute("src", `/media/${ids.vertical}`);
  await viewer.getByRole("button", { name: /Cerrar/ }).click();
  await expect(viewer).toHaveCount(0);
  await expect(cards.nth(0)).toBeFocused();

  // Tab stays inside the modal (the page behind is inert).
  await cards.nth(2).click();
  for (let i = 0; i < 6; i++) {
    await page.keyboard.press("Tab");
    expect(await page.evaluate(() => !!document.activeElement?.closest("dialog[data-viewer]"))).toBe(true);
  }
  // A click on the dark area closes.
  await page.mouse.click(5, 300);
  await expect(viewer).toHaveCount(0);
  await expect(cards.nth(2)).toBeFocused();
});

test("without JavaScript the card opens the image file; the original is the real file", async ({ browser }) => {
  const ctx = await browser.newContext({ javaScriptEnabled: false });
  const page = await ctx.newPage();
  await page.goto(path);
  await page.locator("[data-image-card] a").first().click();
  await expect(page).toHaveURL(`${BASE}/media/${ids.vertical}`);
  const r = await ctx.request.get(`/media/${ids.vertical}`);
  expect(r.status()).toBe(200);
  expect(r.headers()["content-type"]).toBe("image/jpeg");
  await ctx.close();
});

test("the viewer fits phones and 200 % zoom, and respects reduced motion", async ({ browser }) => {
  for (const vp of [{ width: 360, height: 740 }, { width: 640, height: 450 }]) {
    const ctx = await browser.newContext({ viewport: vp, reducedMotion: "reduce" });
    const page = await ctx.newPage();
    await page.goto(path);
    expect(await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBe(0);
    await page.locator("[data-image-card] a").first().click();
    const img = page.locator("[data-viewer-image]");
    await expect(img).toBeVisible();
    await expect.poll(() => img.evaluate((el: HTMLImageElement) => el.complete && el.naturalWidth > 0)).toBe(true);
    const box = (await img.boundingBox())!;
    expect(box.width).toBeLessThanOrEqual(vp.width);
    expect(box.y + box.height).toBeLessThanOrEqual(vp.height);
    // Fitted to the viewport: even a small image uses most of the available area.
    expect(Math.max(box.width / vp.width, box.height / vp.height)).toBeGreaterThan(0.5);
    expect(await img.evaluate((el) => getComputedStyle(el).objectFit)).toBe("contain");
    await expect(page.getByRole("button", { name: /Cerrar/ })).toBeInViewport();
    expect(await page.locator("dialog[data-viewer]").evaluate((d) => d.getAnimations().length)).toBe(0);
    await ctx.close();
  }
});

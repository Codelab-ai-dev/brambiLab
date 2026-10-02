// #53: the hero's Three.js scene as an enhancement over a complete static hero. Assertions are on
// end states (which image is shown, whether the chunk was requested, whether a render loop runs),
// never on exact timings.
import { expect, test, type Page } from "@playwright/test";

const isChunk = (url: string | URL) => /\/assets\/HeroScene-[^/]+\.js$/.test(String(url));
const figure = (page: Page) => page.locator("figure[data-scene]");

/** Records whether the 3D chunk is requested by this page. */
function watchChunk(page: Page) {
  const seen: string[] = [];
  page.on("request", (r) => isChunk(r.url()) && seen.push(r.url()));
  return seen;
}

async function ready(page: Page) {
  await expect(figure(page)).toHaveAttribute("data-scene", "ready", { timeout: 20_000 });
}

test("static image without JavaScript, with reduced motion, on phones and on other routes: no 3D download", async ({ browser }) => {
  for (const [name, options, path] of [
    ["no JS", { javaScriptEnabled: false }, "/es"],
    ["reduced motion", { reducedMotion: "reduce" }, "/es"],
    ["phone", { viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true }, "/es"],
    ["other route", {}, "/es/proyectos"],
  ] as const) {
    const ctx = await browser.newContext(options);
    const page = await ctx.newPage();
    const chunks = watchChunk(page);
    await page.goto(path, { waitUntil: "networkidle" });
    await page.waitForTimeout(500);
    expect(chunks, name).toEqual([]);
    await expect(page.locator("canvas"), name).toHaveCount(0);
    if (path === "/es") {
      await expect(figure(page), name).toHaveAttribute("data-scene", "static");
      await expect(page.locator("[data-scene-static]"), name).toBeVisible();
      await expect(page.getByRole("button", { name: "Pausar animación" }), name).toHaveCount(0);
    }
    await ctx.close();
  }
});

test("title and CTA work before Three.js; then one decorative canvas replaces the image", async ({ page }) => {
  let release!: () => void;
  const gate = new Promise<void>((r) => (release = r));
  await page.route(isChunk, async (route) => {
    await gate;
    await route.continue();
  });
  await page.goto("/es");
  await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
  const cta = page.getByRole("link", { name: "Explorar proyectos" });
  await expect(cta).toBeVisible();
  await expect(page.locator("[data-scene-static]")).toBeVisible(); // while the chunk is held back
  release();
  await ready(page);
  await expect(page.locator("canvas")).toHaveCount(1);
  // The canvas is decorative: out of the accessibility tree, never focusable, never a pointer target.
  const canvas = page.locator("figure[data-scene] canvas");
  expect(await canvas.evaluate((c) => !!c.closest("[aria-hidden='true']"))).toBe(true);
  expect(await canvas.evaluate((c) => getComputedStyle(c).pointerEvents)).toBe("none");
  for (let i = 0; i < 12; i++) {
    await page.keyboard.press("Tab");
    expect(await page.evaluate(() => document.activeElement?.tagName)).not.toBe("CANVAS");
  }
  await expect.poll(() => page.locator("[data-scene-static]").evaluate((s) => getComputedStyle(s).opacity)).toBe("0");
  await cta.click();
  await expect(page).toHaveURL(/\/es\/proyectos$/);
});

test("pause, out of view and hidden tab stop the render loop", async ({ page }) => {
  // Count animation frames requested by the page: a stopped loop requests none.
  await page.addInitScript(() => {
    const raf = window.requestAnimationFrame.bind(window);
    (window as unknown as { __raf: number }).__raf = 0;
    window.requestAnimationFrame = (cb) => {
      (window as unknown as { __raf: number }).__raf++;
      return raf(cb);
    };
  });
  await page.goto("/es");
  await ready(page);
  const frames = async () => {
    const a = await page.evaluate(() => (window as unknown as { __raf: number }).__raf);
    await page.waitForTimeout(600);
    return (await page.evaluate(() => (window as unknown as { __raf: number }).__raf)) - a;
  };
  await expect(figure(page)).toHaveAttribute("data-playing", "true");
  expect(await frames()).toBeGreaterThan(5);

  await page.getByRole("button", { name: "Pausar animación" }).click();
  await expect(figure(page)).toHaveAttribute("data-playing", "false");
  await expect.poll(frames).toBeLessThan(3);
  await page.getByRole("button", { name: "Reanudar animación" }).click();
  await expect(figure(page)).toHaveAttribute("data-playing", "true");

  await page.getByRole("heading", { name: "Cómo se documenta" }).scrollIntoViewIfNeeded();
  await expect(figure(page)).toHaveAttribute("data-playing", "false");
  await expect.poll(frames).toBeLessThan(3);
  await page.evaluate(() => window.scrollTo(0, 0));
  await expect(figure(page)).toHaveAttribute("data-playing", "true");

  await page.evaluate(() => {
    Object.defineProperty(document, "visibilityState", { value: "hidden", configurable: true });
    document.dispatchEvent(new Event("visibilitychange"));
  });
  await expect(figure(page)).toHaveAttribute("data-playing", "false");

  await page.goto("/en");
  await ready(page);
  await expect(page.getByRole("button", { name: "Pause animation" })).toBeVisible();
});

test("turning on reduced motion during the visit removes the scene", async ({ page }) => {
  await page.goto("/es");
  await ready(page);
  await page.emulateMedia({ reducedMotion: "reduce" });
  await expect(page.locator("canvas")).toHaveCount(0);
  await expect(figure(page)).toHaveAttribute("data-scene", "static");
  await expect(page.locator("[data-scene-static]")).toHaveCSS("opacity", "1");
});

test("a failed download, no WebGL or a lost context keep the static hero working", async ({ browser }) => {
  // Chunk download fails.
  const a = await browser.newContext();
  const pa = await a.newPage();
  await pa.route(isChunk, (route) => route.abort());
  await pa.goto("/es");
  await expect(figure(pa)).toHaveAttribute("data-scene", "static", { timeout: 20_000 });
  await expect(pa.locator("[data-scene-static]")).toHaveCSS("opacity", "1");
  await expect(pa.getByRole("link", { name: "Explorar proyectos" })).toBeVisible();
  await a.close();

  // No WebGL: the chunk is never requested.
  const b = await browser.newContext();
  const pb = await b.newPage();
  await pb.addInitScript(() => {
    const get = HTMLCanvasElement.prototype.getContext;
    HTMLCanvasElement.prototype.getContext = function (this: HTMLCanvasElement, type: string, ...rest: unknown[]) {
      return /webgl/.test(type) ? null : (get as (...x: unknown[]) => unknown).call(this, type, ...rest);
    } as typeof get;
  });
  const chunks = watchChunk(pb);
  await pb.goto("/es", { waitUntil: "networkidle" });
  await pb.waitForTimeout(500);
  expect(chunks).toEqual([]);
  await expect(figure(pb)).toHaveAttribute("data-scene", "static");
  await b.close();

  // Lost context: back to the image, the page keeps working.
  const c = await browser.newContext();
  const pc = await c.newPage();
  await pc.goto("/es");
  await ready(pc);
  await pc.locator("figure[data-scene] canvas").evaluate((canvas: HTMLCanvasElement) => {
    const gl = (canvas.getContext("webgl2") ?? canvas.getContext("webgl")) as WebGLRenderingContext;
    gl.getExtension("WEBGL_lose_context")!.loseContext();
  });
  await expect(figure(pc)).toHaveAttribute("data-scene", "static");
  await expect(pc.locator("canvas")).toHaveCount(0);
  await expect(pc.locator("[data-scene-static]")).toHaveCSS("opacity", "1");
  await c.close();
});

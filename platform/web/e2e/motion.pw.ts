// #52: subtle motion as progressive enhancement. These checks avoid exact timings: they assert end
// states (visible, paused, static) and poll for them, never a number of milliseconds.
import { expect, test, type Page } from "@playwright/test";

const pending = (page: Page) => page.evaluate(() => document.querySelectorAll("[data-reveal-pending]").length);
const allRevealedVisible = (page: Page) =>
  page.evaluate(() => [...document.querySelectorAll("[data-reveal]")].every((e) => getComputedStyle(e).opacity === "1"));

test("without JavaScript everything is visible and no pause control is offered", async ({ browser }) => {
  const ctx = await browser.newContext({ javaScriptEnabled: false });
  const page = await ctx.newPage();
  await page.goto("/es");
  expect(await pending(page)).toBe(0);
  await expect.poll(() => allRevealedVisible(page)).toBe(true);
  await expect(page.getByRole("button", { name: "Pausar animación" })).toHaveCount(0);
  await ctx.close();
});

test("reduced motion: final states, nothing hidden, no loop, no pause control", async ({ browser }) => {
  const ctx = await browser.newContext({ reducedMotion: "reduce" });
  const page = await ctx.newPage();
  await page.goto("/es");
  expect(await pending(page)).toBe(0);
  expect(await allRevealedVisible(page)).toBe(true);
  await expect(page.getByRole("button", { name: "Pausar animación" })).toBeHidden();
  await expect.poll(() => page.evaluate(() => document.getAnimations().filter((a) => a.playState === "running").length)).toBe(0);
  await ctx.close();
});

test("hero title is never transparent; sections appear once and stay visible", async ({ page }) => {
  await page.goto("/es", { waitUntil: "domcontentloaded" });
  // The title is the LCP element: it only travels, it never fades.
  expect(await page.locator("h1").evaluate((h) => getComputedStyle(h).opacity)).toBe("1");
  await page.waitForLoadState("networkidle");
  await expect.poll(() => pending(page)).toBeGreaterThan(0); // below the fold, waiting
  const height = await page.evaluate(() => document.documentElement.scrollHeight);
  for (let y = 0; y < height; y += 400) await page.mouse.wheel(0, 400);
  await expect.poll(() => pending(page)).toBe(0);
  await page.evaluate(() => window.scrollTo(0, 0));
  // Going back up never hides anything again.
  expect(await pending(page)).toBe(0);
  await expect.poll(() => allRevealedVisible(page)).toBe(true);
});

test("keyboard focus inside a waiting section shows it at once", async ({ page }) => {
  await page.goto("/es");
  await expect.poll(() => pending(page)).toBeGreaterThan(0);
  // Focus the last link of the page content (the CTA), far below the fold.
  const cta = page.getByRole("link", { name: "Contactar" });
  await cta.focus();
  const section = cta.locator("xpath=ancestor::*[@data-reveal][1]");
  await expect(section).not.toHaveAttribute("data-reveal-pending", "");
  expect(await section.evaluate((e) => getComputedStyle(e).transitionDuration.split(",").every((d) => parseFloat(d) === 0))).toBe(true);
  await expect(cta).toBeInViewport();
});

test("the relief animation can be paused, and stops out of view or with the tab hidden", async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto("/es");
  const figure = page.locator("figure[data-motion='on']");
  const scan = figure.locator(".bl-scan");
  const state = () => scan.evaluate((el) => el.getAnimations()[0]?.playState ?? "none");
  await expect.poll(state).toBe("running");
  await page.getByRole("button", { name: "Pausar animación" }).click();
  await expect.poll(state).toBe("paused");
  await page.getByRole("button", { name: "Reanudar animación" }).click();
  await expect.poll(state).toBe("running");
  // Out of view.
  await page.getByRole("heading", { name: "Cómo se documenta" }).scrollIntoViewIfNeeded();
  await expect(figure).toHaveAttribute("data-paused", "true");
  await page.evaluate(() => window.scrollTo(0, 0));
  await expect.poll(state).toBe("running");
  // Hidden tab.
  await page.evaluate(() => {
    Object.defineProperty(document, "visibilityState", { value: "hidden", configurable: true });
    document.dispatchEvent(new Event("visibilitychange"));
  });
  await expect(figure).toHaveAttribute("data-paused", "true");
  // English label.
  await page.goto("/en");
  await expect(page.getByRole("button", { name: "Pause animation" })).toBeVisible();
});

test("phones and touch: static visual, no pause control, no hover zoom", async ({ browser }) => {
  const ctx = await browser.newContext({ viewport: { width: 390, height: 844 }, hasTouch: true, isMobile: true });
  const page = await ctx.newPage();
  await page.goto("/es");
  await expect(page.getByRole("button", { name: "Pausar animación" })).toBeHidden();
  expect(await page.locator(".bl-scan").evaluate((el) => el.getAnimations().length)).toBe(0);
  expect(await page.evaluate(() => matchMedia("(hover: hover)").matches)).toBe(false);
  expect(await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBe(0);
  await ctx.close();
});

test("navigation background changes on scroll without changing its size", async ({ page }) => {
  await page.goto("/es/proyectos");
  const header = page.locator("header.sticky");
  const before = await header.boundingBox();
  await expect(header).not.toHaveAttribute("data-scrolled", "true");
  await page.mouse.wheel(0, 600);
  await expect(header).toHaveAttribute("data-scrolled", "true");
  expect((await header.boundingBox())!.height).toBe(before!.height);
  await page.evaluate(() => window.scrollTo(0, 0));
  await expect(header).not.toHaveAttribute("data-scrolled", "true");
});

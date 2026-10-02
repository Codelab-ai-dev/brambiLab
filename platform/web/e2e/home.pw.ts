// WEB-009: editorial home — titles that fit, real figures only, navigation on every width.
import { expect, test } from "@playwright/test";

const widths = [320, 375, 768, 1024, 1440, 1920];

test("hero title keeps whole lines at every width, in both languages, without horizontal scroll", async ({ browser }) => {
  for (const [path, name] of [["/es", "Ingeniería para sistemas inteligentes"], ["/en", "Engineering Intelligent Systems"]] as const) {
    for (const width of widths) {
      const ctx = await browser.newContext({ viewport: { width, height: 900 } });
      const page = await ctx.newPage();
      await page.goto(path);
      await expect(page.getByRole("heading", { level: 1 })).toHaveCount(1);
      await expect(page.getByRole("heading", { level: 1 })).toHaveAccessibleName(name);
      await page.evaluate(() => document.fonts.ready);
      // Each of the three lines is one line box: no word pushed to a fourth line, none cut.
      const lines = await page.locator("#hero-title > span").evaluateAll((spans) => spans.map((s) => s.getClientRects().length));
      expect(lines, `${path} @${width}`).toEqual([1, 1, 1]);
      expect(await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth), `${path} @${width}`).toBe(0);
      await ctx.close();
    }
  }
});

test("no word is wider than its heading (words never break in the middle)", async ({ browser }) => {
  for (const path of ["/es", "/en"]) {
    for (const width of widths) {
      const ctx = await browser.newContext({ viewport: { width, height: 900 } });
      const page = await ctx.newPage();
      await page.goto(path);
      await page.evaluate(() => document.fonts.ready);
      const broken = await page.locator("main h1, main h2, main h3").evaluateAll((hs) =>
        hs.flatMap((h) => {
          const style = getComputedStyle(h);
          const probe = document.createElement("span");
          probe.style.cssText = `position:absolute;visibility:hidden;white-space:nowrap;font:${style.font};letter-spacing:${style.letterSpacing}`;
          document.body.appendChild(probe);
          const out = (h.textContent ?? "").split(/\s+/).filter(Boolean).filter((w) => {
            probe.textContent = w;
            return probe.getBoundingClientRect().width > h.clientWidth + 1;
          });
          probe.remove();
          return out.map((w) => `${h.tagName} "${w}"`);
        }),
      );
      expect(broken, `${path} @${width}`).toEqual([]);
      await ctx.close();
    }
  }
});

test("hero leads to the projects; the illustration is labelled as conceptual", async ({ page }) => {
  await page.goto("/es");
  await expect(page.getByRole("link", { name: "Explorar proyectos" })).toHaveAttribute("href", "/es/proyectos");
  await expect(page.getByText("Ilustración conceptual")).toBeVisible();
  await page.goto("/en");
  await expect(page.getByRole("link", { name: "Explore projects" })).toHaveAttribute("href", "/en/projects");
  await expect(page.getByRole("link", { name: "Get in touch" })).toHaveAttribute("href", "/en/contact");
});

test("the figures band shows only real counts", async ({ page, request }) => {
  for (const locale of ["es", "en"] as const) {
    const total = (await (await request.get(`/api/v1/public/${locale}/contents?kind=project&page_size=1`)).json()).total as number;
    await page.goto(`/${locale}`);
    const band = page.locator("[data-stats]");
    await expect(band.locator("dd").first()).toHaveText("3");
    // The project total is the API total for this language, shown only from 3 on.
    if (total >= 3) await expect(band.locator("dd").nth(1)).toHaveText(String(total));
    else await expect(band.locator("dd")).toHaveCount(1);
  }
});

test("mobile menu: disclosure with Escape; without JavaScript it leads to the footer navigation", async ({ browser }) => {
  const ctx = await browser.newContext({ viewport: { width: 375, height: 800 } });
  const page = await ctx.newPage();
  await page.goto("/es");
  const menu = page.getByRole("button", { name: "Menú" });
  const list = page.locator("#site-nav");
  await expect(menu).toHaveAttribute("aria-expanded", "false");
  await expect(list).toBeHidden();
  await menu.click();
  await expect(page.getByRole("button", { name: "Cerrar menú" })).toHaveAttribute("aria-expanded", "true");
  await expect(list.getByRole("link", { name: "Artículos" })).toBeVisible();
  await list.getByRole("link", { name: "Artículos" }).focus();
  await page.keyboard.press("Escape");
  await expect(list).toBeHidden();
  await expect(page.getByRole("button", { name: "Menú" })).toBeFocused();
  // Target size of the control (WCAG 2.5.8 minimum; 44 px chosen).
  expect((await menu.boundingBox())!.height).toBeGreaterThanOrEqual(44);
  await ctx.close();

  const noJs = await browser.newContext({ viewport: { width: 375, height: 800 }, javaScriptEnabled: false });
  const p = await noJs.newPage();
  await p.goto("/es");
  await expect(p.getByRole("link", { name: "Menú" })).toHaveAttribute("href", "#footer-nav");
  const footerNav = p.getByRole("navigation", { name: "Pie de página" });
  for (const name of ["Proyectos", "Artículos", "Buscar", "Acerca de", "Contacto"]) await expect(footerNav.getByRole("link", { name })).toBeVisible();
  await noJs.close();

  const wide = await browser.newContext({ viewport: { width: 1280, height: 800 } });
  const d = await wide.newPage();
  await d.goto("/es");
  await expect(d.getByRole("button", { name: "Menú" })).toBeHidden();
  await expect(d.locator("#site-nav").getByRole("link", { name: "Contacto" })).toBeVisible();
  await wide.close();
});

test("focus on the menu control survives hydration (#52 regression)", async ({ browser }) => {
  const ctx = await browser.newContext({ viewport: { width: 375, height: 800 } });
  const page = await ctx.newPage();
  // Hold the client entry back, focus the control while it is still the plain link, then let
  // React hydrate: the same element becomes the button and keeps the focus.
  let release!: () => void;
  const gate = new Promise<void>((r) => (release = r));
  await page.route(/entry\.client[^/]*\.js$/, async (route) => {
    await gate;
    await route.continue();
  });
  await page.goto("/es", { waitUntil: "domcontentloaded" });
  await page.getByRole("link", { name: "Menú" }).focus();
  release();
  await expect(page.getByRole("button", { name: "Menú" })).toBeFocused();
  await ctx.close();
});

test("the public redesign does not reach the private panel", async ({ browser }) => {
  const ctx = await browser.newContext({ colorScheme: "light" });
  const page = await ctx.newPage();
  await page.goto("/admin/login");
  const body = await page.locator("main").evaluate((el) => ({ bg: getComputedStyle(document.body).backgroundColor, font: getComputedStyle(el).fontFamily }));
  expect(body.bg).toBe("rgb(247, 245, 239)");
  expect(body.font).toContain("IBM Plex Sans");
  await ctx.close();
});

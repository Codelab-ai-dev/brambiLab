// #45: BrambiLab logo in header and footer, localized home link, favicons.
import { expect, test } from "@playwright/test";

test("header and footer logos link home with one accessible name", async ({ page }) => {
  for (const [locale, other] of [["es", "/es/proyectos"], ["en", "/en/projects"]] as const) {
    await page.goto(other);
    const header = page.locator("header").first();
    const homeLink = header.getByRole("link", { name: "BrambiLab", exact: true });
    await expect(homeLink).toHaveAttribute("href", `/${locale}`);
    await expect(homeLink.locator("svg[data-brand-logo]")).toHaveAttribute("aria-hidden", "true");
    const footerLink = page.locator("footer").getByRole("link", { name: "BrambiLab", exact: true });
    await expect(footerLink).toHaveAttribute("href", `/${locale}`);
  }
});

test("logo keeps its proportion and uses the right colors per surface", async ({ browser }) => {
  for (const colorScheme of ["light", "dark"] as const) {
    for (const width of [360, 1280]) {
      const ctx = await browser.newContext({ viewport: { width, height: 800 }, colorScheme });
      const page = await ctx.newPage();
      await page.goto("/es");
      const header = page.locator("header svg[data-brand-logo]");
      const box = (await header.boundingBox())!;
      expect(Math.abs(box.width / box.height - 1220 / 380)).toBeLessThan(0.02);
      expect(box.height).toBeGreaterThanOrEqual(28);
      const fills = (sel: string) =>
        page.locator(sel).evaluate((svg) => [...svg.querySelectorAll("path")].map((p) => getComputedStyle(p).fill));
      // Public site (WEB-009): always on dark surfaces, whatever the system scheme — white text
      // and the light stem (optical variant, #45) in header and footer.
      for (const where of ["header", "footer"]) {
        const fill = await fills(`${where} svg[data-brand-logo]`);
        expect(fill[0], where).toBe("rgb(236, 231, 220)");
        expect(fill[3], where).toBe("rgb(255, 255, 255)");
        // The two blue pieces never change.
        expect(fill.slice(1, 3), where).toEqual(["rgb(0, 185, 237)", "rgb(40, 100, 240)"]);
      }
      // The private panel keeps following the system: the kit's dark version on paper.
      await page.goto("/admin/login");
      const login = await fills("main svg[data-brand-logo]");
      expect(login[3]).toBe(colorScheme === "light" ? "rgb(10, 30, 63)" : "rgb(255, 255, 255)");
      await page.goto("/es");
      expect(await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBe(0);
      await ctx.close();
    }
  }
});

test("favicons are declared and served", async ({ page, request }) => {
  await page.goto("/es");
  await expect(page.locator('link[rel="icon"][type="image/svg+xml"]')).toHaveAttribute("href", "/favicon.svg");
  await expect(page.locator('link[rel="icon"][href="/favicon.ico"]')).toHaveCount(1);
  await expect(page.locator('link[rel="apple-touch-icon"]')).toHaveAttribute("href", "/apple-touch-icon.png");
  await expect(page.locator('link[rel="manifest"]')).toHaveCount(0);
  for (const [path, type] of [["/favicon.svg", "image/svg+xml"], ["/favicon.ico", "image/"], ["/apple-touch-icon.png", "image/png"]]) {
    const r = await request.get(path);
    expect(r.status(), path).toBe(200);
    expect(r.headers()["content-type"], path).toContain(type);
  }
});

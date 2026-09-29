// WEB-008: the operations page shows the same checks as `server ops-check`, owner only.
import { expect, test } from "@playwright/test";

test("operations page lists the checks and the backup runs", async ({ page, browser }) => {
  await page.goto("/admin");
  await page.getByRole("link", { name: "Iniciar sesión con GitHub" }).click();
  await page.waitForURL(/\/admin$/);
  await page.getByRole("navigation", { name: "Panel" }).getByRole("link", { name: "Operación" }).click();
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("Operación");
  for (const name of ["database", "backup", "disk", "maintenance", "publication_jobs"]) {
    await expect(page.locator(`[data-check="${name}"]`)).toBeVisible();
  }
  await expect(page.locator('[data-check="database"]')).toHaveAttribute("data-status", "ok");
  await expect(page.getByText("Avisos desactivados")).toBeVisible(); // no approved channel in e2e
  // Anonymous visitors never reach it.
  const anon = await browser.newContext();
  expect((await anon.request.get("/api/v1/admin/ops")).status()).toBe(401);
  await anon.close();
});

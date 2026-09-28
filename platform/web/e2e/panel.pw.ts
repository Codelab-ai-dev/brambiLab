// End-to-end panel flow through the real proxy, SSR and Go API with the fake GitHub
// (compose.e2e.yaml): WEB-003 acceptance in a browser.
import { expect, test, type Page } from "@playwright/test";

const shots = process.env.SHOTS_DIR;
const shot = async (page: Page, name: string) => {
  if (shots) await page.screenshot({ path: `${shots}/${name}.png`, fullPage: true });
};

async function login(page: Page) {
  await page.goto("/admin");
  await page.getByRole("link", { name: "Iniciar sesión con GitHub" }).click();
  await page.waitForURL(/\/admin$/);
}

const editor = (page: Page) => page.locator(".ProseMirror");
const status = (page: Page) => page.locator("[data-save-status]");

async function waitSaved(page: Page, version: number) {
  await expect(status(page)).toHaveAttribute("data-save-status", "saved", { timeout: 25_000 });
  await expect(status(page)).toContainText(`v${version}`);
}

test("create, write, translate, log, history, preview", async ({ page }) => {
  await login(page);
  await page.getByRole("navigation", { name: "Panel" }).getByRole("link", { name: "Proyectos" }).click();
  await expect(page.getByRole("link", { name: "Proyectos", exact: true })).toHaveAttribute("aria-current", "page");
  await expect(page.getByRole("link", { name: "Artículos", exact: true })).not.toHaveAttribute("aria-current", "page");
  await page.getByRole("link", { name: "Nuevo proyecto" }).first().click();
  await page.getByRole("button", { name: "Crear y empezar a escribir" }).click();
  await page.waitForURL(/\/admin\/contenidos\/[0-9a-f-]+\/es$/);
  const editorUrl = page.url();
  const contentUrl = editorUrl.replace(/\/es$/, "");

  await page.getByLabel("Título", { exact: true }).fill("Rover de prueba con ñandú");
  await expect(page.getByLabel("Slug (URL)")).toHaveValue("rover-de-prueba-con-nandu");
  await editor(page).click();
  await page.keyboard.type("Primer párrafo. ");
  await page.getByRole("button", { name: "Negrita" }).click();
  await page.keyboard.type("en negrita");
  await expect(status(page)).toHaveAttribute("data-save-status", "dirty");
  await waitSaved(page, 1);
  await shot(page, "editor-saved");

  // Reload recovers the last saved revision.
  await page.reload();
  await expect(page.getByLabel("Título", { exact: true })).toHaveValue("Rover de prueba con ñandú");
  await expect(editor(page).locator("strong")).toHaveText("en negrita");

  // Manual save of a metadata change.
  await page.getByLabel("Resumen").fill("Resumen del rover");
  await page.getByRole("button", { name: "Guardar revisión" }).click();
  await waitSaved(page, 2);

  // English as an explicit copy, marked as pending translation.
  await page.goto(contentUrl);
  await page.getByRole("button", { name: "Copiar desde ES como borrador" }).click();
  await expect(page.getByRole("status").filter({ hasText: "Traducción creada" })).toBeVisible();
  await page.getByRole("link", { name: "Editar" }).nth(1).click();
  await expect(page.getByText("Borrador copiado")).toBeVisible();
  await page.getByLabel("Título", { exact: true }).fill("Test rover");
  await page.getByRole("button", { name: "Guardar revisión" }).click();
  await waitSaved(page, 2);

  // Spanish stays untouched by the English edits.
  await page.goto(editorUrl);
  await expect(page.getByLabel("Título", { exact: true })).toHaveValue("Rover de prueba con ñandú");

  // A log under the project.
  await page.goto(contentUrl);
  await page.getByRole("link", { name: "Nueva entrada" }).click();
  await page.getByRole("button", { name: "Crear y empezar a escribir" }).click();
  await page.waitForURL(/\/es$/);
  await page.getByLabel("Título", { exact: true }).fill("Día 1: inventario");
  await page.getByRole("button", { name: "Guardar revisión" }).click();
  await waitSaved(page, 1);
  await page.goto(contentUrl);
  await expect(page.getByRole("link", { name: "Día 1: inventario" })).toBeVisible();
  await shot(page, "content-overview");

  // History and restore-as-new-version.
  await page.goto(`${editorUrl}/historial`);
  await expect(page.getByRole("row")).toHaveCount(3); // header + v2 + v1
  await page.getByRole("button", { name: "Restaurar la versión 1" }).click();
  await expect(page.getByText("La versión 1 se restauró como nueva versión 3.")).toBeVisible();
  await expect(page.getByRole("row")).toHaveCount(4);
  await shot(page, "history");

  // Private preview of v1: shows the content, never the admin chrome in metadata.
  await page.goto(`${editorUrl}/v/1`);
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("Rover de prueba con ñandú");
  await expect(page.locator("article strong")).toHaveText("en negrita");
  await expect(page.getByText("Vista previa privada")).toBeVisible();
  await expect(page).toHaveTitle("Vista previa privada · Panel");
  await shot(page, "preview");
});

test("two tabs: the second one gets a conflict and keeps its text", async ({ browser }) => {
  const context = await browser.newContext();
  const a = await context.newPage();
  await login(a);
  await a.goto("/admin/contenidos/nuevo?tipo=article");
  await a.getByRole("button", { name: "Crear y empezar a escribir" }).click();
  await a.waitForURL(/\/es$/);
  await a.getByLabel("Título", { exact: true }).fill("Artículo compartido");
  await a.getByRole("button", { name: "Guardar revisión" }).click();
  await waitSaved(a, 1);

  const b = await context.newPage();
  await b.goto(a.url());
  await editor(b).click();

  await editor(a).click();
  await a.keyboard.type("Texto de la pestaña A.");
  await a.getByRole("button", { name: "Guardar revisión" }).click();
  await waitSaved(a, 2);

  await b.keyboard.type("Texto de la pestaña B que no debe perderse.");
  await b.getByRole("button", { name: "Guardar revisión" }).click();
  const dialog = b.getByRole("dialog", { name: "Otra pestaña guardó una versión más nueva" });
  await expect(dialog).toBeVisible();
  await expect(status(b)).toHaveAttribute("data-save-status", "conflict");
  await expect(editor(b)).toContainText("Texto de la pestaña B que no debe perderse.");
  await shot(b, "conflict");

  const download = b.waitForEvent("download");
  await dialog.getByRole("button", { name: "Descargar mis cambios (.md)" }).click();
  const file = await download;
  expect(file.suggestedFilename()).toMatch(/\.es\.md$/);

  b.once("dialog", (d) => d.accept());
  await dialog.getByRole("button", { name: /Descartar mis cambios y cargar v2/ }).click();
  await expect(editor(b)).toContainText("Texto de la pestaña A.");
  await expect(editor(b)).not.toContainText("pestaña B");
  await waitSaved(b, 2);
  await context.close();
});

test("Markdown import shows warnings and only replaces after confirming", async ({ page }) => {
  await login(page);
  await page.goto("/admin/contenidos/nuevo?tipo=article");
  await page.getByRole("button", { name: "Crear y empezar a escribir" }).click();
  await page.waitForURL(/\/es$/);
  await editor(page).click();
  await page.keyboard.type("Texto previo");

  await page.getByRole("button", { name: "Importar Markdown" }).click();
  const dialog = page.getByRole("dialog", { name: "Importar Markdown" });
  await dialog.getByLabel("…o pega el texto").fill("# Título uno\n\n<div>html</div>\n\n| a | b |\n|---|---|\n| 1 | 2 |\n");
  await dialog.getByRole("button", { name: "Analizar" }).click();
  await expect(dialog.getByText("2 aviso(s)")).toBeVisible();
  // The confirm/cancel actions stay inside the viewport, without scrolling the page.
  for (const name of ["Cancelar", "Reemplazar el cuerpo actual"]) {
    await expect(dialog.getByRole("button", { name })).toBeInViewport();
  }
  await shot(page, "import");
  await dialog.getByRole("button", { name: "Cancelar" }).click();
  await expect(editor(page)).toContainText("Texto previo");

  await page.getByRole("button", { name: "Importar Markdown" }).click();
  await dialog.getByLabel("…o pega el texto").fill("## Importado\n\nCon **negrita**.\n");
  await dialog.getByRole("button", { name: "Analizar" }).click();
  await dialog.getByRole("button", { name: "Reemplazar el cuerpo actual" }).click();
  await expect(editor(page).locator("h2")).toHaveText("Importado");
  await expect(editor(page)).not.toContainText("Texto previo");
});

test("leaving with unsaved changes asks first", async ({ page }) => {
  await login(page);
  await page.goto("/admin/contenidos/nuevo?tipo=article");
  await page.getByRole("button", { name: "Crear y empezar a escribir" }).click();
  await page.waitForURL(/\/es$/);
  const here = page.url();
  await editor(page).click();
  await page.keyboard.type("sin guardar");
  let asked = "";
  page.once("dialog", (d) => {
    asked = d.message();
    void d.dismiss();
  });
  await page.getByRole("navigation", { name: "Panel" }).getByRole("link", { name: "Inicio" }).click();
  await expect.poll(() => asked).toContain("cambios sin guardar");
  expect(page.url()).toBe(here);
});

test("toolbar is one tab stop with arrow-key navigation", async ({ page }) => {
  await login(page);
  await page.goto("/admin/contenidos/nuevo?tipo=article");
  await page.getByRole("button", { name: "Crear y empezar a escribir" }).click();
  await page.waitForURL(/\/es$/);
  await editor(page).waitFor();
  await page.getByRole("button", { name: "Negrita" }).focus();
  await page.keyboard.press("ArrowRight");
  await expect(page.getByRole("button", { name: "Cursiva" })).toBeFocused();
  // End goes to the last enabled button: media, undo and redo are disabled in a fresh editor.
  await page.keyboard.press("End");
  await expect(page.getByRole("button", { name: "YouTube" })).toBeFocused();
  await page.keyboard.press("Home");
  await expect(page.getByRole("button", { name: "Negrita" })).toBeFocused();
  const tabbable = await page.getByRole("toolbar", { name: "Formato del contenido" }).locator('button[tabindex="0"]').count();
  expect(tabbable).toBe(1);
});

test("mobile layout keeps the title first and no horizontal page scroll", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await login(page);
  await page.goto("/admin/contenidos/nuevo?tipo=project");
  await page.getByRole("button", { name: "Crear y empezar a escribir" }).click();
  await page.waitForURL(/\/es$/);
  await editor(page).waitFor();
  const title = await page.getByLabel("Título", { exact: true }).boundingBox();
  const body = await editor(page).boundingBox();
  expect(title!.y).toBeLessThan(body!.y);
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
  expect(overflow).toBeLessThanOrEqual(0);
  await shot(page, "editor-mobile");
});

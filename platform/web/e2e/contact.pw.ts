// WEB-007 contact through Caddy, SSR and Go with the Resend double (compose.e2e.yaml): form →
// DB → job → provider acceptance → panel. Each test uses its own client IP through the trusted
// proxy chain so the per-client limit is exercised deliberately, not by accident.
import { expect, test, type Browser, type Page } from "@playwright/test";

const BASE = process.env.BASE_URL ?? "http://localhost:8000";
const FAKE = process.env.FAKE_RESEND_URL ?? "http://localhost:9998";
const run = Date.now().toString(36);
let ipSeq = Math.floor(Math.random() * 200);
const nextIP = () => `203.0.113.${(ipSeq++ % 250) + 1}`;

type Sent = { id: string; idempotency_key: string; body: { from: string; to: string[]; reply_to: string; subject: string; text: string } };

async function sentWith(page: Page, marker: string): Promise<Sent | undefined> {
  const all = (await (await page.request.get(`${FAKE}/_emails`)).json()) as Sent[] | null;
  return (all ?? []).find((e) => e.body.text.includes(marker));
}

async function visitor(browser: Browser, js = true, viewport?: { width: number; height: number }) {
  const ip = nextIP();
  const ctx = await browser.newContext({ javaScriptEnabled: js, extraHTTPHeaders: { "X-Forwarded-For": ip }, ...(viewport ? { viewport } : {}) });
  return { ctx, page: await ctx.newPage(), ip };
}

async function login(page: Page) {
  await page.goto("/admin");
  await page.getByRole("link", { name: "Iniciar sesión con GitHub" }).click();
  await page.waitForURL(/\/admin$/);
}

const apiPost = (page: Page, ip: string, body: Record<string, string>) =>
  page.request.post("/api/v1/contact", { data: body, headers: { Origin: BASE, "X-Forwarded-For": ip } });

test("with JS: errors keep the values, receipt reaches the provider and the panel", async ({ browser, page }) => {
  const v = await visitor(browser);
  await v.page.goto("/es/contacto");
  const form = v.page.getByRole("region", { name: "Escríbeme" });
  await form.getByLabel("Nombre").fill("Ana Prueba");
  await form.getByLabel("Correo").fill("ana@example.org");
  await form.getByLabel("Mensaje").fill("corto");
  await form.getByRole("button", { name: "Enviar mensaje" }).click();
  await expect(form.getByRole("alert")).toHaveText("Revisa los campos marcados.");
  await expect(form.getByText("El mensaje debe tener entre 10 y 5000 caracteres.")).toBeVisible();
  await expect(form.getByLabel("Mensaje")).toBeFocused();
  await expect(form.getByLabel("Nombre")).toHaveValue("Ana Prueba");

  const marker = `MARCA-${run}-js`;
  await form.getByLabel("Mensaje").fill(`<script>alert("x")</script>\nHola, ${marker}.`);
  await form.getByRole("button", { name: "Enviar mensaje" }).click();
  await expect(form.getByRole("status")).toContainText("Mensaje recibido");
  await expect(form.getByLabel("Mensaje")).toHaveValue(""); // reset only after the acknowledgement

  let sent: Sent | undefined;
  await expect.poll(async () => (sent = await sentWith(v.page, marker)), { timeout: 30_000, intervals: [1_000] }).toBeTruthy();
  expect(sent!.body.from).toBe('"BrambiLab" <contacto@example.test>');
  expect(sent!.body.to).toEqual(["owner@example.test"]);
  expect(sent!.body.reply_to).toBe("ana@example.org");
  expect(sent!.body.subject).toMatch(/^Contacto BrambiLab #[0-9a-f]{8}$/);
  expect(sent!.idempotency_key).toMatch(/^contact\/[0-9a-f-]{36}$/);

  await login(page);
  await page.goto("/admin/contacto");
  const row = page.getByRole("listitem").filter({ hasText: "Ana Prueba" }).first();
  await row.getByRole("link", { name: "Ana Prueba" }).click();
  await expect(page.locator("[data-message]")).toContainText(marker);
  await expect(page.locator("[data-status]").first()).toHaveText("Aceptado por Resend");
  // The provider id is the one Resend (the double) returned; the text is shown as text.
  const shownIds = await page.locator("[data-provider-id]").allTextContents();
  expect(shownIds.some((id) => (sent?.id ?? "") === id)).toBe(true);
  await expect(page.locator("[data-message]")).toContainText('<script>alert("x")</script>');
  expect(await page.locator("[data-message] script").count()).toBe(0);
  await v.ctx.close();
});

test("without JS: plain form post, keyboard order skips the honeypot, localized result", async ({ browser }) => {
  const v = await visitor(browser, false);
  await v.page.goto("/es/contacto");
  await v.page.getByLabel("Nombre").focus();
  const order: (string | null)[] = [];
  for (let i = 0; i < 4; i++) {
    order.push(await v.page.evaluate(() => document.activeElement?.getAttribute("name") ?? document.activeElement?.tagName ?? null));
    await v.page.keyboard.press("Tab");
  }
  expect(order).toEqual(["name", "email", "message", "BUTTON"]);
  await v.page.getByLabel("Nombre").fill("Beto SinJS");
  await v.page.getByLabel("Correo").fill("beto@example.org");
  await v.page.getByLabel("Mensaje").fill(`Mensaje sin JavaScript MARCA-${run}-nojs`);
  await v.page.getByRole("button", { name: "Enviar mensaje" }).click();
  await expect(v.page).toHaveURL(`${BASE}/es/contacto?estado=recibido#formulario`);
  await expect(v.page.getByRole("status").filter({ hasText: "Mensaje recibido" })).toBeVisible();
  await expect(v.page.locator('meta[name="robots"]')).toHaveAttribute("content", "noindex, follow");
  await expect.poll(async () => !!(await sentWith(v.page, `MARCA-${run}-nojs`)), { timeout: 30_000, intervals: [1_000] }).toBe(true);
  // The browser enforces lengths before sending: a short message never leaves the page.
  await v.page.goto("/es/contacto");
  await v.page.getByLabel("Nombre").fill("Beto");
  await v.page.getByLabel("Correo").fill("beto@example.org");
  await v.page.getByLabel("Mensaje").fill("corto");
  await v.page.getByRole("button", { name: "Enviar mensaje" }).click();
  await expect(v.page).toHaveURL(`${BASE}/es/contacto`);
  await v.ctx.close();
});

test("English, rate limit with Retry-After keeps the text; spoofed IPs do not escape", async ({ browser }) => {
  const v = await visitor(browser);
  const body = (i: number) => ({ name: "Carla", email: "carla@example.org", message: `Relleno número ${i} para la cuota.`, locale: "en", key: `e2e-${run}-quota-${i}-padding` });
  for (let i = 0; i < 5; i++) expect((await apiPost(v.page, v.ip, body(i))).status()).toBe(202);
  // Same client behind a forged left part: still limited.
  expect((await apiPost(v.page, `198.51.100.7, ${v.ip}`, body(9))).status()).toBe(429);
  // Another client is not affected.
  expect((await apiPost(v.page, nextIP(), body(10))).status()).toBe(202);

  await v.page.goto("/en/contact");
  const form = v.page.getByRole("region", { name: "Write to me" });
  await form.getByLabel("Name").fill("Carla");
  await form.getByLabel("E-mail").fill("carla@example.org");
  await form.getByLabel("Message").fill("This one should hit the limit.");
  await form.getByRole("button", { name: "Send message" }).click();
  await expect(form.getByRole("alert")).toContainText(/You can send again in about \d+ minutes/);
  await expect(form.getByLabel("Message")).toHaveValue("This one should hit the limit.");
  await v.ctx.close();
});

test("a failed send is shown as failed and retried consciously from the panel", async ({ browser, page }) => {
  const r = await page.request.post(`${FAKE}/_script`, { data: [{ status: 422, name: "validation_error", match: `MARCA-${run}-retry` }] });
  expect(r.status()).toBe(204);
  const v = await visitor(browser);
  const name = `Dora ${run}`;
  expect((await apiPost(v.page, v.ip, { name, email: "dora@example.org", message: `Fallará la primera vez MARCA-${run}-retry`, locale: "es", key: `e2e-${run}-retry-key-1` })).status()).toBe(202);
  await v.ctx.close();

  await login(page);
  const open = async () => {
    await page.goto("/admin/contacto?estado=failed");
    const link = page.getByRole("link", { name });
    if ((await link.count()) === 0) return false;
    await link.click();
    return true;
  };
  await expect.poll(open, { timeout: 30_000, intervals: [1_000] }).toBe(true);
  await expect(page.locator("[data-status]").first()).toHaveText("Fallido");
  await expect(page.getByRole("table")).toContainText("validation_error");
  page.once("dialog", (d) => void d.accept());
  await page.getByRole("button", { name: "Reintentar envío" }).click();
  await expect(page.getByRole("status").filter({ hasText: "Vuelve a la cola" })).toBeVisible();
  await expect
    .poll(async () => {
      await page.reload();
      return page.locator("[data-status]").first().textContent();
    }, { timeout: 30_000, intervals: [2_000] })
    .toBe("Aceptado por Resend");
  // Accepted messages offer no retry at all.
  await expect(page.getByRole("button", { name: /Reintentar/ })).toHaveCount(0);
});

test("contact page fits 360 px and the honeypot is not exposed", async ({ browser }) => {
  const v = await visitor(browser, true, { width: 360, height: 780 });
  await v.page.goto("/es/contacto");
  const overflow = await v.page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
  expect(overflow).toBe(0);
  // Hidden from assistive technology: not in the accessibility tree as a textbox.
  expect(await v.page.getByRole("textbox", { name: "Deja este campo vacío" }).count()).toBe(0);
  await v.ctx.close();
});

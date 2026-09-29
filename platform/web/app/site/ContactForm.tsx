// Contact form (web-v1.md §13.1). Without JavaScript it is a plain HTML form that posts to Go,
// which answers 303 back here with ?estado (no personal data in the URL); the browser validates
// lengths and format first. With JavaScript it posts JSON, keeps every value on errors, retries
// with the same key after a network failure and only shows success after Go's acknowledgement.
import { useEffect, useRef, useState, type FormEvent } from "react";
import { t, type Locale, type MessageKey } from "~/i18n";

export type ContactState = "recibido" | "invalido" | "limite" | "no-disponible" | "conflicto" | "error";
export const contactStates: ContactState[] = ["recibido", "invalido", "limite", "no-disponible", "conflicto", "error"];

const stateMessage: Record<Exclude<ContactState, "recibido">, MessageKey> = {
  invalido: "form.error.invalid",
  limite: "form.error.limit",
  "no-disponible": "form.error.unavailable",
  conflicto: "form.error.conflict",
  error: "form.error.generic",
};

type Field = "name" | "email" | "message";
type Status = { kind: "idle" } | { kind: "sending" } | { kind: "received" } | { kind: "error"; message: string; focus?: Field };

const fieldError: Record<Field, MessageKey> = { name: "form.error.name", email: "form.error.email", message: "form.error.message" };

function newKey(): string {
  return `web-${crypto.randomUUID().replaceAll("-", "")}`;
}

/** Mirrors the API limits so most mistakes are explained before sending (Go still decides). */
function check(values: Record<Field, string>): Partial<Record<Field, true>> {
  const bad: Partial<Record<Field, true>> = {};
  const name = values.name.trim();
  if (name.length < 1 || [...name].length > 120 || /[\u0000-\u001f\u007f]/.test(name)) bad.name = true;
  const email = values.email.trim();
  if (email.length > 254 || !/^[^\s@<>(),;:"]+@[^\s@<>(),;:"]+\.[^\s@<>(),;:"]+$/.test(email)) bad.email = true;
  const len = [...values.message.trim()].length;
  if (len < 10 || len > 5000) bad.message = true;
  return bad;
}

export function ContactForm({ locale, serverKey, initialState, retentionDays }: { locale: Locale; serverKey: string; initialState: ContactState | null; retentionDays: number }) {
  const [enhanced, setEnhanced] = useState(false);
  const [status, setStatus] = useState<Status>(
    initialState === "recibido" ? { kind: "received" } : initialState ? { kind: "error", message: t(locale, stateMessage[initialState]) } : { kind: "idle" },
  );
  const [errors, setErrors] = useState<Partial<Record<Field, true>>>({});
  const formRef = useRef<HTMLFormElement>(null);
  const statusRef = useRef<HTMLDivElement>(null);
  // The key stays with an unchanged payload (safe retries); any edit gets a new one.
  const attempt = useRef<{ key: string; payload: string }>({ key: serverKey, payload: "" });

  useEffect(() => setEnhanced(true), []);
  // Focus goes to the first invalid field, otherwise to the announced result.
  useEffect(() => {
    if (status.kind === "error" && status.focus) (formRef.current?.elements.namedItem(status.focus) as HTMLElement | null)?.focus();
    else if (status.kind === "received" || status.kind === "error") statusRef.current?.focus();
  }, [status]);

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (status.kind === "sending") return; // no double submit
    const form = e.currentTarget;
    const data = new FormData(form);
    const values = { name: String(data.get("name") ?? ""), email: String(data.get("email") ?? ""), message: String(data.get("message") ?? "") };
    const bad = check(values);
    setErrors(bad);
    const first = (["name", "email", "message"] as Field[]).find((f) => bad[f]);
    if (first) {
      setStatus({ kind: "error", message: t(locale, "form.error.invalid"), focus: first });
      return;
    }
    const payload = JSON.stringify(values);
    if (payload !== attempt.current.payload) attempt.current = { key: attempt.current.payload ? newKey() : attempt.current.key, payload };
    setStatus({ kind: "sending" });
    let response: Response;
    try {
      response = await fetch("/api/v1/contact", {
        method: "POST",
        credentials: "omit", // anonymous form: never the owner's session
        headers: { "Content-Type": "application/json", Accept: "application/json" },
        body: JSON.stringify({ ...values, locale, key: attempt.current.key, bl_hp: String(data.get("bl_hp") ?? "") }),
      });
    } catch {
      setStatus({ kind: "error", message: t(locale, "form.error.network") });
      return;
    }
    if (response.status === 202) {
      form.reset();
      attempt.current = { key: newKey(), payload: "" }; // a new message needs a new key
      setStatus({ kind: "received" });
      return;
    }
    const body = (await response.json().catch(() => null)) as { fields?: Record<string, string> } | null;
    let focus: Field | undefined;
    if (response.status === 422 && body?.fields) {
      const server: Partial<Record<Field, true>> = {};
      for (const f of ["name", "email", "message"] as Field[]) if (body.fields[f]) server[f] = true;
      setErrors(server);
      focus = (["name", "email", "message"] as Field[]).find((f) => server[f]);
    }
    if (response.status === 409) attempt.current = { key: newKey(), payload };
    const retry = Number(response.headers.get("Retry-After"));
    const message =
      response.status === 429
        ? retry > 0
          ? t(locale, "form.error.limitMinutes").replace("{min}", String(Math.max(1, Math.ceil(retry / 60))))
          : t(locale, "form.error.limit")
        : response.status === 422
          ? t(locale, "form.error.invalid")
          : response.status === 409
            ? t(locale, "form.error.conflict")
            : response.status === 503
              ? t(locale, "form.error.unavailable")
              : t(locale, "form.error.generic");
    setStatus({ kind: "error", message, focus });
  }

  const input =
    "min-h-12 w-full rounded-sm border border-border-strong bg-white px-3 py-2 text-base aria-invalid:border-2 aria-invalid:border-danger focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-accent";
  const described = (f: Field, help?: boolean) => [help && `${f}-help`, errors[f] && `${f}-error`].filter(Boolean).join(" ") || undefined;

  return (
    <section aria-labelledby="form-title" id="formulario" className="border-t border-border pt-8">
      <h2 id="form-title" className="bl-h-project">
        {t(locale, "form.title")}
      </h2>
      <div ref={statusRef} tabIndex={-1} className="mt-4 outline-none" aria-live="polite">
        {status.kind === "received" && (
          <p role="status" className="rounded-sm border-l-4 border-success bg-surface-muted px-4 py-3" data-contact="received">
            {t(locale, "form.received")}
          </p>
        )}
        {status.kind === "error" && (
          <p role="alert" className="rounded-sm border-l-4 border-danger bg-surface-muted px-4 py-3" data-contact="error">
            {status.message}
          </p>
        )}
      </div>
      <form ref={formRef} method="post" action="/api/v1/contact" onSubmit={submit} noValidate={enhanced} className="mt-6 grid gap-5" aria-describedby="form-privacy">
        <input type="hidden" name="locale" value={locale} />
        <input type="hidden" name="key" value={serverKey} />
        {/* Honeypot: off-screen, out of the tab order and hidden from assistive technology. */}
        <div aria-hidden="true" className="absolute -left-[10000px] h-px w-px overflow-hidden">
          <label>
            {t(locale, "form.honeypot")}
            <input type="text" name="bl_hp" tabIndex={-1} autoComplete="off" defaultValue="" />
          </label>
        </div>
        <div className="grid gap-5 sm:grid-cols-2">
          <div className="flex flex-col gap-1">
            <label htmlFor="contact-name" className="text-sm font-medium">
              {t(locale, "form.name")}
            </label>
            <input id="contact-name" name="name" required maxLength={120} autoComplete="name" className={input} aria-invalid={errors.name || undefined} aria-describedby={described("name")} />
            {errors.name && <p id="name-error" className="text-sm text-danger">{t(locale, fieldError.name)}</p>}
          </div>
          <div className="flex flex-col gap-1">
            <label htmlFor="contact-email" className="text-sm font-medium">
              {t(locale, "form.email")}
            </label>
            <input id="contact-email" name="email" type="email" required maxLength={254} autoComplete="email" className={input} aria-invalid={errors.email || undefined} aria-describedby={described("email", true)} />
            <p id="email-help" className="text-xs text-text-muted">{t(locale, "form.emailHelp")}</p>
            {errors.email && <p id="email-error" className="text-sm text-danger">{t(locale, fieldError.email)}</p>}
          </div>
        </div>
        <div className="flex flex-col gap-1">
          <label htmlFor="contact-message" className="text-sm font-medium">
            {t(locale, "form.message")}
          </label>
          <textarea id="contact-message" name="message" required minLength={10} maxLength={5000} rows={7} className={input} aria-invalid={errors.message || undefined} aria-describedby={described("message", true)} />
          <p id="message-help" className="text-xs text-text-muted">{t(locale, "form.messageHelp")}</p>
          {errors.message && <p id="message-error" className="text-sm text-danger">{t(locale, fieldError.message)}</p>}
        </div>
        <p id="form-privacy" className="text-sm text-text-muted">
          {t(locale, "form.privacy").replace("{days}", String(retentionDays))}
        </p>
        <button
          type="submit"
          disabled={status.kind === "sending"}
          className="inline-flex min-h-12 items-center justify-center self-start rounded-sm bg-primary px-5 font-medium text-primary-contrast hover:opacity-90 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent disabled:opacity-60"
        >
          {status.kind === "sending" ? t(locale, "form.sending") : t(locale, "form.submit")}
        </button>
      </form>
    </section>
  );
}

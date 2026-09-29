import { Form, Link, useNavigation, useRouteLoaderData } from "react-router";
import type { Route } from "./+types/contact-message";
import { Button, Notice, PageHeader } from "~/components/admin/ui";
import { formatDate } from "~/content/api-types";
import { contactStatusHelp, contactStatusLabels, outcomeLabels, type ContactDetail } from "~/content/contact-types";
import { adminGet } from "~/lib/admin-api.server";
import { apiSend } from "~/lib/admin-client";
import { privateHeaders } from "~/lib/api.server";
import type { loader as layoutLoader } from "./layout";

export function headers() {
  return privateHeaders;
}

export function meta() {
  return [{ title: "Mensaje de contacto · Panel" }, { name: "robots", content: "noindex, nofollow" }];
}

export async function loader({ request, params }: Route.LoaderArgs) {
  return { message: await adminGet<ContactDetail>(request, `/api/v1/admin/contact/messages/${params.id}`) };
}

// Conscious retry (requires JS: it runs in the browser and calls Go with the CSRF token).
export async function clientAction({ request, params }: Route.ClientActionArgs) {
  const form = await request.formData();
  const result = await apiSend<ContactDetail>("POST", `/api/v1/admin/contact/messages/${params.id}/retry`, String(form.get("csrf_token")), {
    expected_version: Number(form.get("version")),
    confirm_possible_duplicate: form.get("confirm") === "yes",
  });
  if (result.ok) return { ok: true as const, message: "Vuelve a la cola: se envía en la siguiente pasada (unos segundos)." };
  const byCode: Record<string, string> = {
    version_conflict: "El mensaje cambió mientras tanto (otra pestaña o el envío automático). Recarga para ver su estado.",
    not_retryable: "Este mensaje no admite reintento: ya fue aceptado por Resend o sigue en curso.",
    confirmation_required: "Hace falta confirmar el posible duplicado.",
  };
  return { ok: false as const, message: byCode[result.error.code] ?? "No se pudo reintentar." };
}

export default function ContactMessage({ loaderData, actionData }: Route.ComponentProps) {
  const m = loaderData.message;
  const { owner } = useRouteLoaderData<typeof layoutLoader>("routes/admin/layout")!;
  const busy = useNavigation().state !== "idle";
  const confirmText =
    m.retry === "new_key_confirm"
      ? "Resend podría tener ya este mensaje (resultado incierto o fuera de la ventana de 23 h). Reintentar usa una clave nueva y puede llegarte duplicado. ¿Reintentar de todos modos?"
      : "¿Reintentar el envío con la misma clave? Resend no lo duplicará si ya lo tenía.";
  return (
    <>
      <PageHeader
        title={m.name}
        description={
          <>
            <Link to="/admin/contacto" className="underline">Contacto</Link> · {m.locale.toUpperCase()} · recibido {formatDate(m.received_at)}
          </>
        }
      />
      {actionData && <Notice tone={actionData.ok ? "success" : "danger"}>{actionData.message}</Notice>}
      <div className="mt-4 grid gap-8 lg:grid-cols-[minmax(0,1.4fr)_minmax(0,1fr)]">
        <section aria-labelledby="msg-title" className="min-w-0">
          <h2 id="msg-title" className="text-lg font-semibold">Mensaje</h2>
          <p className="mt-2 text-sm">
            De <span className="[overflow-wrap:anywhere]">{m.name}</span> ·{" "}
            <a href={`mailto:${m.email}`} className="text-accent underline [overflow-wrap:anywhere]">{m.email}</a>
          </p>
          {/* Plain text, rendered escaped by React; line breaks kept. */}
          <pre className="mt-3 rounded-md border border-border bg-surface-muted p-4 font-sans text-sm whitespace-pre-wrap [overflow-wrap:anywhere]" data-message>
            {m.message}
          </pre>
        </section>
        <section aria-labelledby="send-title" className="flex flex-col gap-3">
          <h2 id="send-title" className="text-lg font-semibold">Envío</h2>
          <p>
            <span data-status={m.status} className="rounded border border-border px-2 py-0.5 text-sm font-medium">{contactStatusLabels[m.status]}</span>
          </p>
          <p className="text-sm text-text-muted">{contactStatusHelp[m.status]}</p>
          <dl className="grid grid-cols-[max-content_minmax(0,1fr)] gap-x-4 gap-y-1 text-sm">
            <dt className="text-text-muted">Intentos</dt>
            <dd>{m.attempts} de {m.max_attempts}</dd>
            {m.next_attempt_at && (
              <>
                <dt className="text-text-muted">Próximo intento</dt>
                <dd>{formatDate(m.next_attempt_at)}</dd>
              </>
            )}
            {m.window_ends_at && (
              <>
                <dt className="text-text-muted">Ventana de idempotencia</dt>
                <dd>hasta {formatDate(m.window_ends_at)}</dd>
              </>
            )}
            {m.provider_email_id && (
              <>
                <dt className="text-text-muted">Id en Resend</dt>
                <dd className="font-mono text-xs break-all" data-provider-id>{m.provider_email_id}</dd>
              </>
            )}
            {m.last_error && (
              <>
                <dt className="text-text-muted">Último error</dt>
                <dd className="font-mono text-xs">{m.last_error}</dd>
              </>
            )}
            <dt className="text-text-muted">Clave</dt>
            <dd className="font-mono text-xs break-all">{m.idempotency_key}</dd>
          </dl>
          {m.retry !== "none" && (
            <Form method="post">
              <input type="hidden" name="csrf_token" value={owner.csrf_token} />
              <input type="hidden" name="version" value={m.version} />
              {m.retry === "new_key_confirm" && <input type="hidden" name="confirm" value="yes" />}
              <Button
                type="submit"
                tone={m.retry === "new_key_confirm" ? "danger" : "secondary"}
                disabled={busy}
                onClick={(e) => {
                  if (!confirm(confirmText)) e.preventDefault();
                }}
              >
                {m.retry === "new_key_confirm" ? "Reintentar con clave nueva (posible duplicado)" : "Reintentar envío"}
              </Button>
            </Form>
          )}
        </section>
      </div>
      <section aria-labelledby="attempts-title" className="mt-8">
        <h2 id="attempts-title" className="text-lg font-semibold">Intentos</h2>
        {m.attempt_log.length === 0 ? (
          <p className="mt-2 text-sm text-text-muted">Todavía ninguno.</p>
        ) : (
          <div className="mt-2 overflow-x-auto rounded-md border border-border">
            <table className="w-full text-sm">
              <thead className="bg-surface-muted text-left">
                <tr>
                  <th scope="col" className="px-3 py-2 font-medium">#</th>
                  <th scope="col" className="px-3 py-2 font-medium">Fecha</th>
                  <th scope="col" className="px-3 py-2 font-medium">Resultado</th>
                  <th scope="col" className="px-3 py-2 font-medium">HTTP</th>
                  <th scope="col" className="px-3 py-2 font-medium">Error</th>
                </tr>
              </thead>
              <tbody>
                {m.attempt_log.map((a) => (
                  <tr key={a.attempt} className="border-t border-border">
                    <td className="px-3 py-2 font-mono">{a.attempt}</td>
                    <td className="px-3 py-2 font-mono text-xs whitespace-nowrap">{formatDate(a.at)}</td>
                    <td className="px-3 py-2">{outcomeLabels[a.outcome] ?? a.outcome}</td>
                    <td className="px-3 py-2 font-mono text-xs">{a.http_status ?? "—"}</td>
                    <td className="px-3 py-2 font-mono text-xs">{a.error_name ?? "—"}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </>
  );
}

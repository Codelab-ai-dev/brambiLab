import { Link } from "react-router";
import type { Route } from "./+types/contact-messages";
import { EmptyState, Notice, PageHeader } from "~/components/admin/ui";
import { formatDate, type Page } from "~/content/api-types";
import { contactStatusLabels, type ContactStatus, type ContactSummary } from "~/content/contact-types";
import { adminGet } from "~/lib/admin-api.server";
import { internalApiUrl, privateHeaders } from "~/lib/api.server";

export function headers() {
  return privateHeaders;
}

export function meta() {
  return [{ title: "Contacto · Panel" }, { name: "robots", content: "noindex, nofollow" }];
}

const filters: (ContactStatus | "")[] = ["", "pending", "retry_wait", "accepted_by_provider", "failed", "unknown"];

export async function loader({ request }: Route.LoaderArgs) {
  const url = new URL(request.url);
  const estado = url.searchParams.get("estado") ?? "";
  const status = filters.includes(estado as ContactStatus) ? estado : "";
  const page = Math.max(1, Number.parseInt(url.searchParams.get("pagina") ?? "1", 10) || 1);
  const qs = new URLSearchParams({ page: String(page), page_size: "20", ...(status ? { status } : {}) });
  const [list, availability] = await Promise.all([
    adminGet<Page<ContactSummary>>(request, `/api/v1/admin/contact/messages?${qs}`),
    fetch(new URL("/api/v1/public/contact", internalApiUrl), { signal: AbortSignal.timeout(5000) })
      .then((r) => r.json() as Promise<{ available: boolean }>)
      .catch(() => null),
  ]);
  return { list, status, available: availability?.available ?? null };
}

export default function ContactMessages({ loaderData }: Route.ComponentProps) {
  const { list, status, available } = loaderData;
  const pages = Math.max(1, Math.ceil(list.total / list.page_size));
  const link = (s: string, p = 1) => {
    const q = new URLSearchParams();
    if (s) q.set("estado", s);
    if (p > 1) q.set("pagina", String(p));
    const str = q.toString();
    return str ? `?${str}` : "";
  };
  return (
    <>
      <PageHeader title="Contacto" description="Mensajes del formulario público. Se borran automáticamente al cumplir su plazo de retención." />
      {available === false && (
        <Notice tone="warning" title="Formulario desactivado">
          El sitio no acepta mensajes ni envía correos hasta configurar Resend en el servicio api (guía docs/operations/contacto-resend.md).
        </Notice>
      )}
      <nav aria-label="Filtrar por estado" className="mt-4 flex flex-wrap gap-2 text-sm">
        {filters.map((f) => (
          <Link key={f || "all"} to={link(f)} aria-current={status === f ? "page" : undefined} className="rounded-md border border-border px-3 py-1.5 aria-[current=page]:border-accent aria-[current=page]:text-accent">
            {f ? contactStatusLabels[f] : "Todos"}
          </Link>
        ))}
      </nav>
      <div className="mt-4">
        {list.items.length === 0 ? (
          <EmptyState title={status ? "No hay mensajes con este estado." : "Todavía no hay mensajes."} />
        ) : (
          <ul className="divide-y divide-border rounded-md border border-border text-sm">
            {list.items.map((m) => (
              <li key={m.id} className="flex flex-col gap-1 px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
                <div className="min-w-0">
                  <Link to={`/admin/contacto/${m.id}`} className="font-medium text-accent hover:underline [overflow-wrap:anywhere]">
                    {m.name}
                  </Link>{" "}
                  <span className="text-text-muted [overflow-wrap:anywhere]">&lt;{m.email}&gt;</span>
                </div>
                <div className="flex flex-wrap items-center gap-3 font-mono text-xs text-text-muted">
                  <span data-status={m.status} className="rounded border border-border px-2 py-0.5 font-sans text-text">{contactStatusLabels[m.status]}</span>
                  <span>{m.locale.toUpperCase()}</span>
                  <span>{formatDate(m.received_at)}</span>
                </div>
              </li>
            ))}
          </ul>
        )}
      </div>
      {pages > 1 && (
        <nav aria-label="Paginación" className="mt-4 flex items-center justify-between text-sm">
          {list.page > 1 ? <Link to={link(status, list.page - 1)}>← Más recientes</Link> : <span />}
          <span className="text-text-muted">Página {list.page} de {pages}</span>
          {list.page < pages ? <Link to={link(status, list.page + 1)}>Más antiguos →</Link> : <span />}
        </nav>
      )}
    </>
  );
}

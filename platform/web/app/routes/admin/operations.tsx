import type { Route } from "./+types/operations";
import { EmptyState, Notice, PageHeader } from "~/components/admin/ui";
import { formatBytes, formatDate } from "~/content/api-types";
import { adminGet } from "~/lib/admin-api.server";
import { privateHeaders } from "~/lib/api.server";

// Operational status (web-v1.md §15.3): the same checks as `server ops-check` and the alert
// webhook, plus the last backup runs. Read-only; runbooks in docs/operations/incidentes.md.

type Status = "ok" | "warn" | "critical";
type Ops = {
  report: { status: Status; checked_at: string; checks: { name: string; status: Status; detail: string }[] };
  backups: { started_at: string; finished_at: string | null; status: "running" | "succeeded" | "failed"; step: string | null; bytes: number | null; assets: number | null; error: string | null }[];
  alerts_configured: boolean;
};

const names: Record<string, string> = {
  database: "Base de datos",
  backup: "Backup externo",
  disk: "Disco (volumen de medios)",
  maintenance: "Mantenimiento",
  background_jobs: "Tareas en segundo plano",
  publication_jobs: "Publicaciones programadas",
  contact_jobs: "Envíos de contacto",
};
const statusLabel: Record<Status, string> = { ok: "Correcto", warn: "Atención", critical: "Crítico" };
const statusClass: Record<Status, string> = { ok: "border-success text-success", warn: "border-warning text-warning", critical: "border-danger text-danger" };

export function headers() {
  return privateHeaders;
}

export function meta() {
  return [{ title: "Operación · Panel" }, { name: "robots", content: "noindex, nofollow" }];
}

export async function loader({ request }: Route.LoaderArgs) {
  return adminGet<Ops>(request, "/api/v1/admin/ops");
}

export default function Operations({ loaderData }: Route.ComponentProps) {
  const { report, backups, alerts_configured } = loaderData;
  return (
    <>
      <PageHeader title="Operación" description={<>Estado general: <strong>{statusLabel[report.status]}</strong> · comprobado {formatDate(report.checked_at)}. Recarga para volver a comprobar.</>} />
      {!alerts_configured && (
        <Notice tone="warning" title="Avisos desactivados">
          No hay un canal de avisos aprobado (ALERT_WEBHOOK_URL). Los problemas sólo quedan en los logs y en esta página.
        </Notice>
      )}
      <ul className="mt-4 divide-y divide-border rounded-md border border-border text-sm">
        {report.checks.map((c) => (
          <li key={c.name} className="flex flex-col gap-1 px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
            <span className="font-medium">{names[c.name] ?? c.name}</span>
            <span className="flex flex-wrap items-center gap-3">
              <span className="text-text-muted">{c.detail}</span>
              <span data-check={c.name} data-status={c.status} className={`rounded border px-2 py-0.5 text-xs font-medium ${statusClass[c.status]}`}>
                {statusLabel[c.status]}
              </span>
            </span>
          </li>
        ))}
      </ul>
      <section aria-labelledby="backups-title" className="mt-8">
        <h2 id="backups-title" className="text-lg font-semibold">Últimos backups</h2>
        {backups.length === 0 ? (
          <div className="mt-3">
            <EmptyState title="Todavía no hay ejecuciones registradas." />
          </div>
        ) : (
          <div className="mt-3 overflow-x-auto rounded-md border border-border">
            <table className="w-full text-sm">
              <thead className="bg-surface-muted text-left">
                <tr>
                  <th scope="col" className="px-3 py-2 font-medium">Inicio</th>
                  <th scope="col" className="px-3 py-2 font-medium">Resultado</th>
                  <th scope="col" className="px-3 py-2 font-medium">Tamaño</th>
                  <th scope="col" className="px-3 py-2 font-medium">Archivos</th>
                  <th scope="col" className="px-3 py-2 font-medium">Detalle</th>
                </tr>
              </thead>
              <tbody>
                {backups.map((b) => (
                  <tr key={b.started_at} className="border-t border-border">
                    <td className="px-3 py-2 font-mono text-xs whitespace-nowrap">{formatDate(b.started_at)}</td>
                    <td className="px-3 py-2">{b.status === "succeeded" ? "Correcto" : b.status === "failed" ? "Fallido" : "En curso"}</td>
                    <td className="px-3 py-2 font-mono text-xs">{b.bytes ? formatBytes(b.bytes) : "—"}</td>
                    <td className="px-3 py-2 font-mono text-xs">{b.assets ?? "—"}</td>
                    <td className="px-3 py-2 font-mono text-xs">{b.error ?? (b.status === "running" ? b.step : "")}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
      <p className="mt-6 text-sm text-text-muted">Qué hacer ante cada problema: docs/operations/incidentes.md en el repositorio.</p>
    </>
  );
}

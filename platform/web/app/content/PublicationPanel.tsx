// Publication controls for one translation (WEB-005, web-v1.md §8.1). The server is the source of
// truth: every action sends the editorial version it was shown, and a 409 replaces the local state
// with the server's. Only saved revisions are ever published; the unsaved editor buffer never is.

import { useId, useState, type ReactNode } from "react";
import { Link, useRevalidator } from "react-router";
import { Button, Field, inputClass, Notice } from "~/components/admin/ui";
import {
  cancelReasonLabels,
  EDITORIAL_ZONE,
  editorialStatusLabels,
  formatDate,
  formatLocal,
  jobStatusLabels,
  localeLabels,
  nowLocal,
  type EditorialState,
  type Locale,
  type PublicationJob,
} from "./api-types";
import { siteFromApiPath } from "~/site/paths";
import { describe, publicationRequest, type ApiError, type PublicationAction } from "~/lib/admin-client";

type Outcome = { tone: "success" | "danger"; message: string; error?: ApiError } | null;

/** Shared state machine for the panel and the history rows. */
export function usePublication(contentId: string, locale: Locale, csrf: string, initial: EditorialState, revalidate = false) {
  const [state, setState] = useState(initial);
  const [busy, setBusy] = useState(false);
  const [outcome, setOutcome] = useState<Outcome>(null);
  const revalidator = useRevalidator();

  async function run(action: PublicationAction, body: Record<string, unknown>, confirmText: string, success: string): Promise<boolean> {
    if (busy || !window.confirm(confirmText)) return false;
    setBusy(true);
    setOutcome(null);
    const result = await publicationRequest(contentId, locale, action, csrf, { ...body, expected_editorial_version: state.editorial_version });
    setBusy(false);
    if (result.ok) {
      setState(result.data);
      setOutcome({ tone: "success", message: success });
      if (revalidate) void revalidator.revalidate();
      return true;
    }
    if (result.error.state) setState(result.error.state);
    setOutcome({ tone: "danger", message: describe(result.error), error: result.error });
    return false;
  }
  return { state, busy, outcome, run };
}

type Props = {
  contentId: string;
  locale: Locale;
  csrf: string;
  initial: EditorialState;
  /** Latest saved revision: the one Publish and Schedule act on. */
  savedVersion: number;
  /** Most recent jobs (to offer Retry on a failed one). */
  jobs?: PublicationJob[];
  /** Why publishing is not possible right now (e.g. unsaved changes), shown next to the buttons. */
  blockedReason?: string;
  archived?: boolean;
  revalidate?: boolean;
  /** Heading level context: the panel renders an h2 by default. */
  headingLevel?: 2 | 3;
};

export { Problems };

export function PublicationPanel({ contentId, locale, csrf, initial, savedVersion, jobs = [], blockedReason, archived, revalidate, headingLevel = 2 }: Props) {
  const { state, busy, outcome, run } = usePublication(contentId, locale, csrf, initial, revalidate);
  const [scheduling, setScheduling] = useState(false);
  const [when, setWhen] = useState("");
  const headingId = useId();
  const H = headingLevel === 2 ? "h2" : "h3";
  const job = state.active_job;
  const lastFailed = !job && jobs[0]?.status === "failed" ? jobs[0] : null;
  const alreadyPublic = state.published_version === savedVersion;
  const canPublish = !archived && !blockedReason && savedVersion > 0;
  const lang = localeLabels[locale];

  const publish = () =>
    run(
      "publish",
      { revision_version: savedVersion },
      `¿Publicar ahora la versión ${savedVersion} en ${lang}?` +
        (state.published_version ? `\n\nSustituye a la versión pública ${state.published_version}.` : "") +
        (job ? `\n\nTambién cancela la publicación programada para ${formatLocal(job.run_at_local)}.` : ""),
      `Versión ${savedVersion} publicada.`,
    );

  const schedule = async (e: React.FormEvent) => {
    e.preventDefault();
    const done = await run(
      "schedule",
      { revision_version: savedVersion, run_at_local: when, time_zone: EDITORIAL_ZONE, replace: Boolean(job) },
      `¿Programar la versión ${savedVersion} en ${lang} para el ${formatLocal(when)}?` +
        "\n\nSe publicará esa versión tal como está guardada ahora; los cambios posteriores no se incluyen." +
        (job ? `\n\nReemplaza la programación actual (${formatLocal(job.run_at_local)}).` : ""),
      `Versión ${savedVersion} programada para el ${formatLocal(when)}.`,
    );
    if (done) setScheduling(false);
  };

  return (
    <section aria-labelledby={headingId} className="flex flex-col gap-3 rounded-md border border-border p-4" data-publication={locale}>
      <div className="flex flex-wrap items-center justify-between gap-2">
        <H id={headingId} className="font-semibold">
          Publicación · {lang}
        </H>
        <StatusBadge state={state} />
      </div>

      <dl className="grid gap-x-4 gap-y-1 text-sm sm:grid-cols-[max-content_1fr]">
        {state.status === "published" && (
          <>
            <dt className="text-text-muted">Versión pública</dt>
            <dd>
              v{state.published_version} · desde {formatDate(state.published_at)}
            </dd>
            {state.route && siteFromApiPath(state.route) && (
              <>
                <dt className="text-text-muted">Página pública</dt>
                <dd>
                  <a href={siteFromApiPath(state.route)!} className="font-mono text-xs break-all text-accent underline underline-offset-2">
                    Ver publicación<span className="sr-only"> en {lang}</span> · {siteFromApiPath(state.route)}
                  </a>
                </dd>
              </>
            )}
          </>
        )}
        {state.status === "withdrawn" && (
          <>
            <dt className="text-text-muted">Retirada</dt>
            <dd>{formatDate(state.withdrawn_at)} · no visible para el público</dd>
          </>
        )}
        <dt className="text-text-muted">Última guardada</dt>
        <dd>
          {savedVersion > 0 ? `v${savedVersion}` : "Sin revisiones"}
          {alreadyPublic && " · es la versión pública"}
        </dd>
      </dl>

      {job && <ScheduledJob job={job} />}
      {lastFailed && <FailedJob job={lastFailed} />}

      {outcome && (
        <Notice tone={outcome.tone}>
          {outcome.message}
          {outcome.error && <Problems error={outcome.error} />}
        </Notice>
      )}

      {!archived && (
        <div className="flex flex-wrap items-center gap-2">
          <Button tone="primary" onClick={() => void publish()} disabled={busy || !canPublish || (alreadyPublic && !job)}>
            {state.published_version ? `Publicar v${savedVersion || ""} ahora` : `Publicar v${savedVersion || ""}`}
          </Button>
          <Button onClick={() => setScheduling((v) => !v)} disabled={busy || !canPublish} aria-expanded={scheduling}>
            {job ? "Cambiar programación" : "Programar…"}
          </Button>
          {job && (
            <Button
              onClick={() =>
                void run("cancel", {}, `¿Cancelar la publicación programada para ${formatLocal(job.run_at_local)}?\n\nLo que ya está publicado no cambia.`, "Programación cancelada.")
              }
              disabled={busy}
            >
              Cancelar programación
            </Button>
          )}
          {lastFailed && (
            <Button
              onClick={() =>
                void run(
                  `jobs/${lastFailed.id}/retry`,
                  {},
                  `¿Reintentar la publicación de la versión ${lastFailed.revision_version}?\n\nSe vuelve a validar ahora y se publica en la siguiente pasada del programador (menos de un minuto).`,
                  "Reintento en cola: se publicará en menos de un minuto si todo sigue en orden.",
                )
              }
              disabled={busy}
            >
              Reintentar v{lastFailed.revision_version}
            </Button>
          )}
          {state.status === "published" && (
            <Button
              tone="danger"
              onClick={() =>
                void run(
                  "withdraw",
                  {},
                  `¿Retirar la publicación en ${lang}?\n\nDeja de ser visible y sus archivos dejan de servirse al público. Se conservan las revisiones y la ruta queda reservada.` +
                    (job ? "\n\nTambién cancela la publicación programada." : ""),
                  "Publicación retirada.",
                )
              }
              disabled={busy}
            >
              Retirar
            </Button>
          )}
          {blockedReason && <span className="text-xs text-text-muted">{blockedReason}</span>}
        </div>
      )}
      {archived && <p className="text-sm text-text-muted">Contenido archivado: desarchívalo para publicar.</p>}

      {scheduling && !archived && (
        <form onSubmit={(e) => void schedule(e)} className="flex flex-wrap items-end gap-3 rounded-md bg-surface-muted p-3">
          <Field label={`Fecha y hora (${EDITORIAL_ZONE})`} help={`Se publicará la versión ${savedVersion} tal como está guardada ahora.`}>
            {({ id, describedBy }) => (
              <input
                id={id}
                type="datetime-local"
                required
                className={`${inputClass} w-auto`}
                min={nowLocal()}
                value={when}
                aria-describedby={describedBy}
                onChange={(e) => setWhen(e.target.value)}
              />
            )}
          </Field>
          <Button type="submit" tone="primary" disabled={busy || !when}>
            {job ? "Reemplazar programación" : "Programar"}
          </Button>
        </form>
      )}
    </section>
  );
}

export function StatusBadge({ state }: { state: EditorialState }) {
  const tone = state.status === "published" ? "border-success text-success" : state.status === "withdrawn" ? "border-warning text-warning" : "border-border text-text-muted";
  return (
    <span className="flex flex-wrap gap-1.5 text-xs font-medium" data-status={state.status}>
      <span className={`rounded border px-2 py-0.5 ${tone}`}>{editorialStatusLabels[state.status]}</span>
      {state.active_job && <span className="rounded border border-accent px-2 py-0.5 text-accent">Programada</span>}
    </span>
  );
}

function ScheduledJob({ job }: { job: PublicationJob }) {
  return (
    <Notice title={`Programada: v${job.revision_version} para el ${formatLocal(job.run_at_local)}`}>
      {job.attempts > 0 ? (
        <>
          Intento {job.attempts} de {job.max_attempts} falló{job.last_error ? `: ${job.last_error}` : ""}. Siguiente intento: {formatDate(job.next_attempt_at)}.
        </>
      ) : (
        "La revisión quedó fijada al programar; se valida de nuevo al publicarse."
      )}
    </Notice>
  );
}

function FailedJob({ job }: { job: PublicationJob }) {
  return (
    <Notice tone="warning" title={`La publicación programada de v${job.revision_version} falló`}>
      {job.last_error ?? "Error desconocido"} ({job.error_kind === "terminal" ? "requiere corregir el contenido o sus archivos" : `tras ${job.attempts} intentos`}).
    </Notice>
  );
}

/** Per-field problems of publish_blocked / has_published_logs, linked to where they are fixed. */
function Problems({ error }: { error: ApiError }) {
  const entries = Object.entries(error.fields ?? {});
  if (entries.length === 0) return null;
  const item = (key: string, text: string): ReactNode => {
    if (key.startsWith("assets.")) return <Link className="underline" to={`/admin/medios/${key.slice(7)}`}>{text}</Link>;
    if (key.startsWith("logs.")) return <Link className="underline" to={`/admin/contenidos/${key.slice(5)}`}>{text}</Link>;
    return text;
  };
  return (
    <ul className="mt-1 list-disc pl-5">
      {entries.map(([k, v]) => (
        <li key={k}>{item(k, v)}</li>
      ))}
    </ul>
  );
}

export function jobSummary(job: PublicationJob): string {
  const base = `${jobStatusLabels[job.status]} · v${job.revision_version} · ${formatLocal(job.run_at_local)}`;
  if (job.status === "cancelled" && job.cancel_reason) return `${base} · ${cancelReasonLabels[job.cancel_reason] ?? job.cancel_reason}`;
  return base;
}

// Browser-side mutations of private admin data. They go straight to the Go API on the same public
// origin (through the proxy) with the session CSRF token; Go authenticates and authorizes.

import type { EditorialState } from "~/content/api-types";
import type { SaveOutcome } from "~/content/autosave";

export type ApiError = { code: string; message: string; fields?: Record<string, string>; current_version?: number; state?: EditorialState };
export type ApiResult<T> = { ok: true; status: number; data: T } | { ok: false; status: number; error: ApiError };

export async function apiSend<T>(method: "POST" | "PATCH" | "PUT", path: string, csrf: string, body?: unknown, headers: Record<string, string> = {}): Promise<ApiResult<T>> {
  const response = await fetch(path, {
    method,
    credentials: "same-origin",
    headers: { "Content-Type": "application/json", Accept: "application/json", "X-CSRF-Token": csrf, ...headers },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const json = await response.json().catch(() => null);
  if (response.ok) return { ok: true, status: response.status, data: json as T };
  return { ok: false, status: response.status, error: (json as ApiError) ?? { code: "unknown", message: `HTTP ${response.status}` } };
}

export function revisionsPath(contentId: string, locale: string) {
  return `/api/v1/admin/contents/${contentId}/translations/${locale}/revisions`;
}

/** Adapter used by the Autosaver: maps HTTP answers to save outcomes. */
export async function saveRevision(
  contentId: string,
  locale: string,
  csrf: string,
  snapshot: unknown,
  expectedVersion: number,
  kind: "manual" | "auto",
  idempotencyKey: string,
): Promise<SaveOutcome> {
  let result: ApiResult<{ created: boolean; revision: { version: number } }>;
  try {
    result = await apiSend("POST", revisionsPath(contentId, locale), csrf, { expected_version: expectedVersion, kind, snapshot }, { "Idempotency-Key": idempotencyKey });
  } catch {
    return { ok: false, kind: "network", message: "Sin conexión con el servidor." };
  }
  if (result.ok) return { ok: true, version: result.data.revision.version, created: result.data.created };
  const { status, error } = result;
  if (status === 409 && error.code === "version_conflict") return { ok: false, kind: "conflict", currentVersion: error.current_version ?? -1 };
  if (status === 401) return { ok: false, kind: "unauthenticated" };
  if (status === 422 || status === 413 || status === 409) return { ok: false, kind: "invalid", message: describe(error), fields: error.fields };
  return { ok: false, kind: "server", message: `El servidor respondió ${status}.` };
}

const codes: Record<string, string> = {
  validation_failed: "Hay campos con errores.",
  media_not_available: "Los medios estarán disponibles con la carga de archivos (WEB-004).",
  payload_too_large: "El contenido supera 1 MiB.",
  content_archived: "El contenido está archivado: desarchívalo para editar.",
  csrf_failed: "La sesión cambió. Recarga la página.",
  origin_rejected: "Petición rechazada por origen.",
  editorial_conflict: "El estado de publicación cambió mientras tanto (otra pestaña o el programador). Revisa el estado actual y vuelve a intentarlo.",
  publish_blocked: "Esta versión todavía no se puede publicar:",
  slug_taken: "Otro contenido ya usa esa ruta pública (o la tuvo antes). Cambia el slug, guarda y vuelve a intentarlo.",
  has_published_logs: "Este proyecto tiene entradas de bitácora publicadas en este idioma. Retíralas primero:",
  schedule_exists: "Ya hay una publicación programada.",
  no_active_schedule: "No hay ninguna publicación programada.",
  not_published: "Esta traducción no está publicada ni programada.",
  job_not_failed: "Sólo se puede reintentar una programación fallida.",
  idempotency_key_reused: "La petición se repitió con otros datos. Recarga la página.",
  scheduled_content: "No se puede archivar con publicaciones programadas. Cancélalas primero.",
  published_content: "No se puede archivar un contenido publicado. Retíralo primero.",
};

export function describe(error: ApiError): string {
  return codes[error.code] ?? error.message ?? "Error desconocido.";
}

export function newIdempotencyKey(): string {
  return `save-${crypto.randomUUID()}`;
}

/** PATCH/DELETE of a library asset from the browser (Go checks session, CSRF and Origin). */
export async function assetRequest(method: "PATCH" | "DELETE", id: string, csrf: string, body?: unknown) {
  const r = await fetch(`/api/v1/admin/assets/${id}`, {
    method,
    credentials: "same-origin",
    headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const json = (r.status === 204 ? null : await r.json().catch(() => null)) as { code?: string } | null;
  return { ok: r.ok, status: r.status, json };
}

export type PublicationAction = "publish" | "schedule" | "cancel" | "withdraw" | `jobs/${string}/retry`;

export function publicationPath(contentId: string, locale: string, action?: PublicationAction | "jobs") {
  return `/api/v1/admin/contents/${contentId}/translations/${locale}/publication${action ? `/${action}` : ""}`;
}

/** One editorial mutation with a fresh Idempotency-Key (Go checks session, CSRF and Origin). */
export async function publicationRequest(
  contentId: string,
  locale: string,
  action: PublicationAction,
  csrf: string,
  body: Record<string, unknown>,
): Promise<ApiResult<EditorialState>> {
  try {
    return await apiSend<EditorialState>("POST", publicationPath(contentId, locale, action), csrf, body, { "Idempotency-Key": `pub-${crypto.randomUUID()}` });
  } catch {
    return { ok: false, status: 0, error: { code: "network", message: "Sin conexión con el servidor: no se sabe si la acción llegó. Recarga la página para ver el estado real antes de repetirla." } };
  }
}

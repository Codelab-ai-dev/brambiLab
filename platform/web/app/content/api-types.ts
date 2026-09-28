// Shapes of the private content API (platform/contracts/openapi.yaml, WEB-003).
import type { Doc } from "./schema";

export type ContentKind = "project" | "article" | "log";
export type Locale = "es" | "en";
export type RevisionKind = "manual" | "auto" | "restore" | "copy";
export type ProjectStatus = "idea" | "in_development" | "paused" | "completed";

export type TranslationSummary = {
  locale: Locale;
  latest_version: number;
  title: string | null;
  published: boolean;
  updated_at: string | null;
};

export type Content = {
  id: string;
  kind: ContentKind;
  project_id: string | null;
  created_at: string;
  archived_at: string | null;
  translations: TranslationSummary[];
};

export type Page<T> = { items: T[]; page: number; page_size: number; total: number };

export type ProjectFields = {
  objective?: string;
  status?: ProjectStatus | "";
  technologies?: string[];
  links?: { label: string; url: string }[];
  results?: string;
};

export type Snapshot = {
  title: string;
  slug: string;
  summary: string;
  body: Doc;
  seo: { title?: string; description?: string };
  project_fields?: ProjectFields;
  category_id: string | null;
  tag_ids: string[];
  /** Ready image used as cover (WEB-004). */
  cover_asset_id?: string | null;
};

export type AssetKind = "image" | "video" | "resource";

export type Asset = {
  id: string;
  kind: AssetKind;
  status: "pending" | "ready" | "failed";
  original_name: string;
  mime: string;
  bytes: number;
  sha256: string;
  width: number | null;
  height: number | null;
  public_enabled: boolean;
  downloadable: boolean;
  created_at: string;
  texts: Partial<Record<Locale, { alt: string; caption: string }>>;
};

export type AssetReference = { content_id: string; locale: Locale; version: number; usage: "image" | "video" | "poster" | "download" | "cover"; published: boolean };
export type AssetDetail = Asset & { references: AssetReference[] };

export const assetKindLabels: Record<AssetKind, string> = { image: "Imagen", video: "Vídeo", resource: "Recurso" };

export function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / 1024 / 1024).toFixed(1)} MB`;
}

export type Revision = Snapshot & {
  version: number;
  kind: RevisionKind;
  created_at: string;
  body_schema_version: number;
  restored_from_version: number | null;
  copied_from_locale: Locale | null;
};

export type RevisionSummary = {
  version: number;
  kind: RevisionKind;
  created_at: string;
  title: string;
  restored_from_version: number | null;
};

export type Translation = {
  content_id: string;
  locale: Locale;
  latest_version: number;
  published: boolean;
  latest: Revision | null;
};

export type Term = { id: string; slug: string; labels: { es: string; en: string } };

export const kindLabels: Record<ContentKind, { one: string; many: string; new: string }> = {
  project: { one: "Proyecto", many: "Proyectos", new: "Nuevo proyecto" },
  article: { one: "Artículo", many: "Artículos", new: "Nuevo artículo" },
  log: { one: "Entrada de bitácora", many: "Bitácora", new: "Nueva entrada de bitácora" },
};

export const localeLabels: Record<Locale, string> = { es: "Español", en: "English" };

export const statusLabels: Record<ProjectStatus, string> = {
  idea: "Idea",
  in_development: "En desarrollo",
  paused: "Pausado",
  completed: "Completado",
};

export const revisionKindLabels: Record<RevisionKind, string> = {
  manual: "Manual",
  auto: "Automática",
  restore: "Restauración",
  copy: "Copia de otro idioma",
};

/** Display title for the Spanish admin UI: the Spanish title first, then English, then a placeholder. */
export function displayTitle(c: Content, fallback: string): string {
  const title = (l: Locale) => c.translations.find((t) => t.locale === l && t.title)?.title;
  return title("es") ?? title("en") ?? fallback;
}

export function formatDate(iso: string | null | undefined): string {
  if (!iso) return "—";
  return new Date(iso).toLocaleString("es-MX", { timeZone: "America/Mexico_City", dateStyle: "medium", timeStyle: "short" });
}

export function snapshotFrom(rev: Revision | null): Snapshot {
  if (!rev) {
    return { title: "", slug: "", summary: "", body: { type: "doc", content: [] }, seo: {}, category_id: null, tag_ids: [] };
  }
  return {
    title: rev.title,
    slug: rev.slug,
    summary: rev.summary,
    body: rev.body,
    seo: rev.seo ?? {},
    ...(rev.project_fields ? { project_fields: rev.project_fields } : {}),
    category_id: rev.category_id,
    tag_ids: rev.tag_ids ?? [],
    ...(rev.cover_asset_id ? { cover_asset_id: rev.cover_asset_id } : {}),
  };
}

/** Suggests a slug from a title: lowercase ASCII, digits and single hyphens. */
export function slugify(title: string): string {
  return title
    .normalize("NFD")
    .replace(/[̀-ͯ]/g, "")
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 120)
    .replace(/-+$/, "");
}

// Publication (WEB-005, web-v1.md §8.1).
export const EDITORIAL_ZONE = "America/Mexico_City";

export type EditorialStatus = "unpublished" | "published" | "withdrawn";
export type JobStatus = "scheduled" | "succeeded" | "failed" | "cancelled";

export type PublicationJob = {
  id: string;
  revision_version: number;
  run_at: string;
  run_at_local: string;
  status: JobStatus;
  attempts: number;
  max_attempts: number;
  next_attempt_at: string | null;
  last_error: string | null;
  error_kind: "transient" | "terminal" | null;
  cancel_reason: string | null;
  created_at: string;
  finished_at: string | null;
  attempt_log: { attempt: number; at: string; outcome: "succeeded" | "transient_error" | "terminal_error"; error: string | null }[];
};

export type EditorialState = {
  status: EditorialStatus;
  editorial_version: number;
  latest_version: number;
  published_version: number | null;
  published_at: string | null;
  first_published_at: string | null;
  withdrawn_at: string | null;
  route: string | null;
  active_job: PublicationJob | null;
  time_zone: typeof EDITORIAL_ZONE;
};

export const editorialStatusLabels: Record<EditorialStatus, string> = {
  unpublished: "Sin publicar",
  published: "Publicada",
  withdrawn: "Retirada",
};

export const jobStatusLabels: Record<JobStatus, string> = {
  scheduled: "Programada",
  succeeded: "Publicada",
  failed: "Fallida",
  cancelled: "Cancelada",
};

export const cancelReasonLabels: Record<string, string> = {
  cancelled: "cancelada a mano",
  replaced: "reemplazada por otra programación",
  manual_publish: "sustituida por una publicación manual",
  withdrawn: "cancelada al retirar",
};

/** "2026-10-01T09:30" (editorial wall clock) → "1 oct 2026, 9:30 (America/Mexico_City)". */
export function formatLocal(local: string): string {
  const [date, time] = local.split("T");
  const [y, m, d] = date.split("-").map(Number);
  const month = new Date(Date.UTC(y, m - 1, 15)).toLocaleString("es-MX", { month: "short", timeZone: "UTC" });
  return `${d} ${month.replace(".", "")} ${y}, ${time} (${EDITORIAL_ZONE})`;
}

/** The current editorial wall clock, "YYYY-MM-DDTHH:MM", for datetime-local min values. */
export function nowLocal(now = new Date()): string {
  const parts = Object.fromEntries(
    new Intl.DateTimeFormat("en-CA", { timeZone: EDITORIAL_ZONE, year: "numeric", month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", hourCycle: "h23" })
      .formatToParts(now)
      .map((p) => [p.type, p.value]),
  );
  return `${parts.year}-${parts.month}-${parts.day}T${parts.hour}:${parts.minute}`;
}

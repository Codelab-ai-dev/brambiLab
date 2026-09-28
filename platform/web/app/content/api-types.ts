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

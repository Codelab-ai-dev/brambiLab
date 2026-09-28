// Shared pieces of the public loaders: locale, validated query parameters, SEO and the language
// switcher target. Nothing here reads cookies.
import { data } from "react-router";
import type { Locale } from "~/i18n";
import { isSlug, localeOfPath } from "./paths";
import { absolute } from "./seo.server";
import type { Seo } from "./seo";
import type { Cover } from "./types";

export function localeOf(request: Request): Locale {
  return localeOfPath(new URL(request.url).pathname);
}

export type ListQuery = { page: number; category?: string; tag?: string; kind?: string; q?: string };

/** Parses ?page, ?category, ?tag, ?kind and ?q. Malformed values are a 400, never guessed. */
export function listQuery(request: Request, allowed: (keyof ListQuery)[]): ListQuery {
  const params = new URL(request.url).searchParams;
  const out: ListQuery = { page: 1 };
  for (const key of params.keys()) {
    if (!allowed.includes(key as keyof ListQuery) || params.getAll(key).length > 1) throw data(null, { status: 400 });
  }
  const page = params.get("page");
  if (page !== null) {
    if (!/^[1-9][0-9]{0,4}$/.test(page)) throw data(null, { status: 400 });
    out.page = Number(page);
  }
  for (const key of ["category", "tag"] as const) {
    const v = params.get(key);
    if (v === null || v === "") continue; // an empty select means "all"
    if (!isSlug(v)) throw data(null, { status: 400 });
    out[key] = v;
  }
  const kind = params.get("kind");
  if (kind) {
    if (!["project", "article", "log"].includes(kind)) throw data(null, { status: 400 });
    out.kind = kind;
  }
  const q = params.get("q");
  if (q !== null) out.q = q.slice(0, 200);
  return out;
}

/** Query string for the API from a parsed list query (only set values). */
export function apiQuery(q: ListQuery, extra: Record<string, string> = {}): string {
  const p = new URLSearchParams(extra);
  if (q.page > 1) p.set("page", String(q.page));
  for (const key of ["category", "tag", "kind", "q"] as const) if (q[key]) p.set(key, q[key]!);
  const s = p.toString();
  return s ? `?${s}` : "";
}

type SeoInput = {
  locale: Locale;
  path: string;
  /** Query string kept in the canonical (e.g. ?page=2), without filters. */
  canonicalQuery?: string;
  title: string;
  description: string;
  alternates: Partial<Record<Locale, string>>;
  cover?: Cover | null;
  type?: "website" | "article";
  noindex?: boolean;
};

export function seo(input: SeoInput): Seo {
  const alternates: Partial<Record<Locale, string>> = {};
  for (const [l, p] of Object.entries(input.alternates) as [Locale, string][]) alternates[l] = absolute(p + (input.canonicalQuery ?? ""));
  return {
    title: input.title === "BrambiLab" ? input.title : `${input.title} · BrambiLab`,
    description: input.description,
    canonical: absolute(input.path + (input.canonicalQuery ?? "")),
    locale: input.locale,
    alternates,
    image: input.cover ? { url: absolute(`/media/${input.cover.asset_id}`), width: input.cover.width, height: input.cover.height, alt: input.cover.alt } : undefined,
    type: input.type,
    noindex: input.noindex,
  };
}

export type { Switcher } from "./types";

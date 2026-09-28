// Loader shared by the project and article indexes: filters, paging, taxonomy and SEO.
import { data } from "react-router";
import { t, type MessageKey } from "~/i18n";
import { publicGet } from "./api.server";
import { apiQuery, listQuery, localeOf, seo } from "./loader.server";
import { sectionPath } from "./paths";
import { noindexHeaders, publicHeaders } from "./seo";
import type { CardPage, Taxonomy } from "./types";

export async function listLoader(request: Request, kind: "project" | "article") {
  const locale = localeOf(request);
  const q = listQuery(request, ["page", "category", "tag"]);
  const section = kind === "project" ? "projects" : "articles";
  const [page, taxonomy] = await Promise.all([
    publicGet<CardPage>(`/api/v1/public/${locale}/contents${apiQuery(q, { kind })}`),
    publicGet<Taxonomy>(`/api/v1/public/${locale}/taxonomy?kind=${kind}`),
  ]);
  const filtered = !!(q.category || q.tag);
  const other = locale === "es" ? "en" : "es";
  const titleKey = `list.${section}.title` as MessageKey;
  const body = {
    locale,
    kind,
    page,
    taxonomy,
    selected: { category: q.category, tag: q.tag },
    path: sectionPath(locale, section),
    // Indexes exist in both locales; filtered views are noindex; paging keeps its own canonical.
    seo: seo({
      locale,
      path: sectionPath(locale, section),
      canonicalQuery: q.page > 1 && !filtered ? `?page=${q.page}` : filtered ? apiQuery({ ...q }) : "",
      title: t(locale, titleKey),
      description: t(locale, `list.${section}.lead` as MessageKey),
      alternates: filtered ? {} : { es: sectionPath("es", section), en: sectionPath("en", section) },
      noindex: filtered,
    }),
    switcher: { href: sectionPath(other, section), available: true },
  };
  return data(body, { headers: filtered ? noindexHeaders : publicHeaders });
}

export type ListData = Awaited<ReturnType<typeof listLoader>>["data"];

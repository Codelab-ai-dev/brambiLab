// sitemap.xml from canonical, visible pages only (web-v1.md §9.1). XML is escaped; alternates use
// xhtml:link hreflang only when the other translation is visible.
import { locales, type Locale } from "~/i18n";
import { publicGet } from "./api.server";
import { contentPath, homePath, sectionPath, type Section } from "./paths";
import { absolute } from "./seo.server";
import type { SitemapPage } from "./types";

export const API_PAGE = 5000;

export function xmlEscape(s: string): string {
  return s.replace(/[<>&'"]/g, (c) => ({ "<": "&lt;", ">": "&gt;", "&": "&amp;", "'": "&apos;", '"': "&quot;" })[c]!);
}

type Url = { loc: string; lastmod?: string; alternates: Partial<Record<Locale, string>> };

/** Indexable static pages: home, unfiltered indexes, about and contact (never search). */
export function staticUrls(): Url[] {
  const pages: (Section | "home")[] = ["home", "projects", "articles", "about", "contact"];
  return pages.flatMap((p) => {
    const path = (l: Locale) => (p === "home" ? homePath(l) : sectionPath(l, p));
    const alternates = Object.fromEntries(locales.map((l) => [l, absolute(path(l))]));
    return locales.map((l) => ({ loc: absolute(path(l)), alternates }));
  });
}

export async function contentUrls(page: number): Promise<{ urls: Url[]; total: number }> {
  const res = await publicGet<SitemapPage>(`/api/v1/public/sitemap?page=${page}`);
  const urls: Url[] = [];
  for (const e of res.items) {
    const path = contentPath(e.locale, e.kind, e.slug, e.project_slug);
    if (!path) continue;
    const alternates: Partial<Record<Locale, string>> = { [e.locale]: absolute(path) };
    for (const a of e.alternates) {
      const alt = contentPath(a.locale, e.kind, a.slug, a.project_slug);
      if (alt) alternates[a.locale] = absolute(alt);
    }
    urls.push({ loc: absolute(path), lastmod: e.lastmod, alternates });
  }
  return { urls, total: res.total };
}

export function urlset(urls: Url[]): string {
  const body = urls
    .map((u) => {
      const alts = Object.entries(u.alternates);
      const links = alts.length > 1 ? alts.map(([l, href]) => `<xhtml:link rel="alternate" hreflang="${l}" href="${xmlEscape(href!)}"/>`).join("") : "";
      return `<url><loc>${xmlEscape(u.loc)}</loc>${u.lastmod ? `<lastmod>${xmlEscape(u.lastmod)}</lastmod>` : ""}${links}</url>`;
    })
    .join("\n");
  return `<?xml version="1.0" encoding="UTF-8"?>\n<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" xmlns:xhtml="http://www.w3.org/1999/xhtml">\n${body}\n</urlset>\n`;
}

export function sitemapIndex(pages: number): string {
  const body = Array.from({ length: pages }, (_, i) => `<sitemap><loc>${xmlEscape(absolute(`/sitemaps/${i + 1}.xml`))}</loc></sitemap>`).join("\n");
  return `<?xml version="1.0" encoding="UTF-8"?>\n<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">\n${body}\n</sitemapindex>\n`;
}

export const xmlHeaders = { "Content-Type": "application/xml; charset=utf-8", "Cache-Control": "no-store" };

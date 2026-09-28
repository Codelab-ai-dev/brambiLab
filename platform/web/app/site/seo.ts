// SEO description computed by loaders (absolute URLs need the server-only origin) and turned into
// tags by each route's meta(). Everything here is rendered in the initial HTML.
import type { MetaDescriptor } from "react-router";
import type { Locale } from "~/i18n";

export type Seo = {
  title: string;
  description: string;
  canonical: string;
  locale: Locale;
  /** Absolute URLs of the visible translations of this page, by locale (self included). */
  alternates: Partial<Record<Locale, string>>;
  image?: { url: string; width: number | null; height: number | null; alt: string };
  type?: "website" | "article";
  noindex?: boolean;
};

const ogLocale: Record<Locale, string> = { es: "es_MX", en: "en_US" };

export function seoMeta(seo: Seo | undefined): MetaDescriptor[] {
  if (!seo) return [{ title: "BrambiLab" }];
  const tags: MetaDescriptor[] = [
    { title: seo.title },
    { name: "description", content: seo.description },
    { tagName: "link", rel: "canonical", href: seo.canonical },
    { property: "og:site_name", content: "BrambiLab" },
    { property: "og:title", content: seo.title },
    { property: "og:description", content: seo.description },
    { property: "og:url", content: seo.canonical },
    { property: "og:type", content: seo.type ?? "website" },
    { property: "og:locale", content: ogLocale[seo.locale] },
  ];
  // hreflang only when another visible translation exists; always reciprocal (both sides list both).
  const entries = Object.entries(seo.alternates) as [Locale, string][];
  if (entries.length > 1) {
    for (const [locale, href] of entries) tags.push({ tagName: "link", rel: "alternate", hrefLang: locale, href });
  }
  if (seo.image) {
    tags.push({ property: "og:image", content: seo.image.url }, { property: "og:image:alt", content: seo.image.alt });
    if (seo.image.width && seo.image.height) {
      tags.push({ property: "og:image:width", content: String(seo.image.width) }, { property: "og:image:height", content: String(seo.image.height) });
    }
  }
  if (seo.noindex) tags.push({ name: "robots", content: "noindex, follow" });
  return tags;
}

export const publicHeaders = { "Cache-Control": "no-store" };
export const noindexHeaders = { "Cache-Control": "no-store", "X-Robots-Tag": "noindex" };

/** Route headers(): what the loader (or the thrown error) decided; public HTML is never cached. */
export function forwardHeaders({ loaderHeaders, errorHeaders }: { loaderHeaders: Headers; errorHeaders?: Headers }): Headers {
  const h = new Headers(errorHeaders ?? loaderHeaders);
  if (!h.has("Cache-Control")) h.set("Cache-Control", "no-store");
  return h;
}

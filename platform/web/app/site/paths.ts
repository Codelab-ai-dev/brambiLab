// Public site routes (web-v1.md §9) and the validated mapping from API reader paths to them.
// Shared by the SSR loaders, the language switcher, the sitemap and the admin "Ver publicación".
import type { Locale } from "~/i18n";

export const sections = {
  es: { projects: "proyectos", log: "bitacora", articles: "articulos", search: "buscar", about: "acerca-de", contact: "contacto" },
  en: { projects: "projects", log: "log", articles: "articles", search: "search", about: "about", contact: "contact" },
} as const satisfies Record<Locale, Record<string, string>>;

export type Section = keyof (typeof sections)["es"];
export type ContentKind = "project" | "article" | "log";

const slugPattern = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;
export const isSlug = (s: string | undefined): s is string => !!s && s.length <= 120 && slugPattern.test(s);

export function homePath(locale: Locale) {
  return `/${locale}`;
}

export function sectionPath(locale: Locale, section: Section) {
  return `/${locale}/${sections[locale][section]}`;
}

export function projectPath(locale: Locale, slug: string) {
  return `${sectionPath(locale, "projects")}/${slug}`;
}

export function logPath(locale: Locale, projectSlug: string, slug: string) {
  return `${projectPath(locale, projectSlug)}/${sections[locale].log}/${slug}`;
}

export function articlePath(locale: Locale, slug: string) {
  return `${sectionPath(locale, "articles")}/${slug}`;
}

/** Site path of a content by kind and current slugs (null if a log has no project slug). */
export function contentPath(locale: Locale, kind: ContentKind, slug: string, projectSlug?: string | null): string | null {
  switch (kind) {
    case "project":
      return projectPath(locale, slug);
    case "article":
      return articlePath(locale, slug);
    case "log":
      return projectSlug ? logPath(locale, projectSlug, slug) : null;
  }
}

const apiPath =
  /^\/api\/v1\/public\/(es|en)\/(?:projects\/([a-z0-9]+(?:-[a-z0-9]+)*)(?:\/logs\/([a-z0-9]+(?:-[a-z0-9]+)*))?|articles\/([a-z0-9]+(?:-[a-z0-9]+)*))$/;

/**
 * Translates a public API reader path (a 301 Location or the panel's route) into the site path.
 * Anything else, including absolute URLs to other origins, yields null: never an open redirect.
 */
export function siteFromApiPath(location: string | null | undefined): string | null {
  if (!location) return null;
  const m = apiPath.exec(location);
  if (!m) return null;
  const locale = m[1] as Locale;
  if (m[4]) return articlePath(locale, m[4]);
  if (m[3]) return logPath(locale, m[2], m[3]);
  return projectPath(locale, m[2]);
}

/** First path segment as a locale (admin and unknown paths fall back to the default). */
export function localeOfPath(pathname: string): Locale {
  const first = pathname.split("/")[1];
  return first === "en" ? "en" : "es";
}

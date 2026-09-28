// Loader shared by project, log and article pages.
import { data, redirect } from "react-router";
import { type Locale } from "~/i18n";
import { publicGet, publicRead } from "./api.server";
import { localeOf, seo } from "./loader.server";
import { contentPath, isSlug, sectionPath, siteFromApiPath, type Section } from "./paths";
import { publicHeaders } from "./seo";
import type { CardPage, PublicContent, Switcher } from "./types";

const notFound = () => data(null, { status: 404, headers: publicHeaders });

/** Plain text of the first paragraphs, for a description when the author wrote none. */
function firstText(c: PublicContent): string {
  const out: string[] = [];
  for (const b of c.body.content) {
    if (b.type === "paragraph" && b.content) out.push(b.content.map((n) => ("text" in n ? n.text : " ")).join(""));
    if (out.join(" ").length > 160) break;
  }
  const text = out.join(" ").replace(/\s+/g, " ").trim();
  return text.length > 160 ? `${text.slice(0, 157).trimEnd()}…` : text;
}

export async function detailLoader(request: Request, apiPath: string, slugs: (string | undefined)[], index: Section) {
  const locale = localeOf(request);
  if (!slugs.every(isSlug)) throw notFound();
  const read = await publicRead<PublicContent>(`/api/v1/public/${locale}/${apiPath}`);
  if (read.moved) {
    // An alias: one 301 to the current site path. Only a validated API reader path is accepted.
    const target = siteFromApiPath(read.location);
    if (!target) throw notFound();
    throw redirect(target, { status: 301, headers: publicHeaders });
  }
  const c = read.data;
  const path = contentPath(locale, c.kind, c.slug, c.project?.slug);
  if (!path) throw notFound();
  const other: Locale = locale === "es" ? "en" : "es";
  const alt = c.alternates.find((a) => a.locale === other);
  const altPath = alt ? contentPath(alt.locale, alt.kind, alt.slug, alt.project_slug) : null;
  const switcher: Switcher = altPath ? { href: altPath, available: true } : { href: sectionPath(other, index), available: false };
  const alternates: Partial<Record<Locale, string>> = { [locale]: path };
  if (altPath) alternates[other] = altPath;

  const url = new URL(request.url);
  let logs: CardPage | null = null;
  if (c.kind === "project") {
    const page = url.searchParams.get("page");
    if (page !== null && !/^[1-9][0-9]{0,4}$/.test(page)) throw data(null, { status: 400, headers: publicHeaders });
    logs = await publicGet<CardPage>(`/api/v1/public/${locale}/contents?kind=log&project=${c.slug}&page_size=10${page ? `&page=${page}` : ""}`);
  }
  return data(
    {
      locale,
      content: c,
      path,
      logs,
      switcher,
      seo: seo({
        locale,
        path,
        canonicalQuery: logs && logs.page > 1 ? `?page=${logs.page}` : "",
        title: c.seo.title || c.title,
        description: c.seo.description || c.summary || firstText(c),
        alternates: logs && logs.page > 1 ? { [locale]: path } : alternates,
        cover: c.cover,
        type: "article",
      }),
    },
    { headers: publicHeaders },
  );
}

export type DetailData = Awaited<ReturnType<typeof detailLoader>>["data"];

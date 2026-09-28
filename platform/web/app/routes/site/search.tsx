import { data } from "react-router";
import type { Route } from "./+types/search";
import { t } from "~/i18n";
import { publicGet } from "~/site/api.server";
import { apiQuery, listQuery, localeOf, seo } from "~/site/loader.server";
import { sectionPath } from "~/site/paths";
import { forwardHeaders, noindexHeaders, seoMeta } from "~/site/seo";
import type { SearchPage, Taxonomy } from "~/site/types";
import { CardGrid, Container, Empty, FilterForm, PageIntro, Pager } from "~/site/ui";

export const headers = forwardHeaders;

export function meta({ loaderData }: Route.MetaArgs) {
  return seoMeta(loaderData?.seo);
}

export async function loader({ request }: Route.LoaderArgs) {
  const locale = localeOf(request);
  const q = listQuery(request, ["q", "kind", "category", "tag", "page"]);
  const [results, taxonomy] = await Promise.all([
    publicGet<SearchPage>(`/api/v1/public/${locale}/search${apiQuery(q)}`),
    publicGet<Taxonomy>(`/api/v1/public/${locale}/taxonomy`),
  ]);
  const other = locale === "es" ? "en" : "es";
  return data(
    {
      locale,
      q,
      results,
      taxonomy,
      // Search results are never indexed; the canonical is the bare search page.
      seo: seo({ locale, path: sectionPath(locale, "search"), title: t(locale, "search.title"), description: t(locale, "search.hint"), alternates: {}, noindex: true }),
      switcher: { href: sectionPath(other, "search"), available: true },
    },
    { headers: noindexHeaders },
  );
}

export default function Search({ loaderData }: Route.ComponentProps) {
  const { locale, q, results, taxonomy } = loaderData;
  const path = sectionPath(locale, "search");
  const query = new URLSearchParams();
  for (const k of ["q", "kind", "category", "tag"] as const) if (q[k]) query.set(k, q[k]!);
  return (
    <main>
      <PageIntro title={t(locale, "search.title")} lead={t(locale, "search.hint")}>
        <div className="mt-8">
          <FilterForm locale={locale} action={path} categories={taxonomy.categories} tags={taxonomy.tags} selected={q} kinds>
            <label className="flex w-full flex-col gap-1 text-sm font-medium sm:w-auto sm:min-w-80">
              {t(locale, "search.label")}
              <input
                type="search"
                name="q"
                defaultValue={q.q ?? ""}
                maxLength={200}
                aria-describedby="search-help"
                className="min-h-10 rounded-md border border-border-strong bg-surface px-3 text-base focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-accent"
              />
              <span id="search-help" className="text-xs font-normal text-text-muted">
                {t(locale, "search.help")}
              </span>
            </label>
          </FilterForm>
        </div>
      </PageIntro>
      <Container className="py-10">
        {!results.has_terms ? (
          <p className="text-text-muted" role="status">
            {t(locale, "search.hint")}
          </p>
        ) : results.items.length === 0 ? (
          <Empty>{t(locale, "search.none")}</Empty>
        ) : (
          <>
            <p className="mb-6 font-mono text-sm text-text-muted" role="status">
              {results.total} {t(locale, results.total === 1 ? "search.result" : "search.results")}
            </p>
            <CardGrid locale={locale} cards={results.items} headingLevel={2} />
          </>
        )}
        <Pager locale={locale} page={results.page} pageSize={results.page_size} total={results.total} path={path} query={query} />
      </Container>
    </main>
  );
}

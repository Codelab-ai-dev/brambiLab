import type { Route } from "./+types/home";
import { Areas, AREA_COUNT } from "~/components/site/Areas";
import { CTASection } from "~/components/site/CTASection";
import { Frame, SectionHeader } from "~/components/site/editorial";
import { Hero } from "~/components/site/Hero";
import { ProjectFeature, ProjectNavigation } from "~/components/site/ProjectFeature";
import { StatsBand } from "~/components/site/StatsBand";
import { formatDay, isoDay, t, type Locale, type MessageKey } from "~/i18n";
import { publicGet } from "~/site/api.server";
import { localeOf, seo } from "~/site/loader.server";
import { contentPath, homePath, sectionPath } from "~/site/paths";
import { forwardHeaders, seoMeta } from "~/site/seo";
import type { Card, CardPage, Home as HomeData } from "~/site/types";

export const headers = forwardHeaders;

export function meta({ loaderData }: Route.MetaArgs) {
  return seoMeta(loaderData?.seo);
}

export async function loader({ request }: Route.LoaderArgs) {
  const locale = localeOf(request);
  const [home, projects] = await Promise.all([
    publicGet<HomeData>(`/api/v1/public/${locale}/home`),
    // Only the total: every public project in this language, never the length of a page.
    publicGet<CardPage>(`/api/v1/public/${locale}/contents?kind=project&page_size=1`),
  ]);
  return {
    locale,
    home,
    projectTotal: projects.total,
    seo: seo({
      locale,
      path: homePath(locale),
      title: "BrambiLab",
      description: home.site.intro || t(locale, "site.tagline"),
      alternates: { es: homePath("es"), en: homePath("en") },
    }),
    switcher: { href: homePath(locale === "es" ? "en" : "es"), available: true },
  };
}

// Editorial order of the sections on this page (not identifiers).
const index = (n: number) => String(n).padStart(2, "0");

export default function Home({ loaderData }: Route.ComponentProps) {
  const { home, projectTotal } = loaderData;
  const locale: Locale = loaderData.locale;
  const featured = home.featured.length > 0;
  const projects = featured ? home.featured : home.latest_projects;

  return (
    <main>
      <Hero locale={locale} />

      <section aria-labelledby="projects-title">
        <Frame className="grid gap-12 py-20 sm:py-28 lg:grid-cols-12 lg:gap-8">
          <div className="lg:col-span-8">
            <SectionHeader id="projects-title" index={index(1)} label={t(locale, "nav.projects")} title={t(locale, featured ? "home.featured" : "home.projects")} />
            {projects.length === 0 && <p className="mt-10 max-w-xl text-lg text-text-muted">{t(locale, "home.noneProjects")}</p>}
          </div>
          <div className="flex flex-col justify-end gap-8 lg:col-span-4">
            {projects.length > 1 && <ProjectNavigation locale={locale} cards={projects} />}
            {projectTotal > 0 && (
              <a
                href={sectionPath(locale, "projects")}
                className="group flex min-h-24 items-end justify-between gap-6 border border-border-strong p-5 hover:border-signal focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-signal"
              >
                <span className="bl-meta">{t(locale, "home.allProjects")}</span>
                <svg viewBox="0 0 48 48" aria-hidden="true" className="bl-arrow size-12 text-signal" fill="none" stroke="currentColor" strokeWidth="5" strokeLinecap="square">
                  <path d="M10 38 36 12M14 11h23v23" />
                </svg>
              </a>
            )}
          </div>
        </Frame>
        {projects.length > 0 && (
          <div className="bl-light">
            <Frame>
              {projects.map((c, i) => (
                <ProjectFeature key={c.slug} locale={locale} card={c} position={i + 1} total={projects.length} />
              ))}
            </Frame>
          </div>
        )}
      </section>

      <Areas locale={locale} index={index(2)} />

      <Latest locale={locale} logs={home.latest_logs} articles={home.latest_articles} />

      <section aria-labelledby="method-title" className="bl-light">
        <Frame className="grid gap-12 py-20 sm:py-28 lg:grid-cols-12 lg:gap-8">
          <div className="lg:col-span-6">
            <SectionHeader id="method-title" index={index(4)} label={t(locale, "method.label")} title={t(locale, "site.method.title")} />
          </div>
          <ol className="self-end border-t border-border lg:col-span-6">
            {(["site.method.hypothesis", "site.method.failures", "site.method.evidence"] as MessageKey[]).map((m, i) => (
              <li key={m} className="flex items-baseline gap-6 border-b border-border py-6">
                <span className="font-mono text-sm text-accent">{index(i + 1)}</span>
                <p className="text-xl leading-snug font-medium tracking-tight sm:text-2xl">{t(locale, m)}</p>
              </li>
            ))}
          </ol>
        </Frame>
      </section>

      <StatsBand locale={locale} areas={AREA_COUNT} projects={projectTotal} />

      <CTASection locale={locale} site={home.site} />
    </main>
  );
}

/** Latest log entries and articles as editorial rows; an honest line when nothing is published. */
function Latest({ locale, logs, articles }: { locale: Locale; logs: Card[]; articles: Card[] }) {
  const columns = [
    { id: "logs", title: "home.logs" as MessageKey, cards: logs, more: undefined },
    { id: "articles", title: "home.articles" as MessageKey, cards: articles, more: { href: sectionPath(locale, "articles"), label: "home.allArticles" as MessageKey } },
  ].filter((c) => c.cards.length > 0);
  return (
    <section aria-labelledby="latest-title" className="border-t border-border">
      <Frame className="py-20 sm:py-28">
        <SectionHeader id="latest-title" index={index(3)} label={t(locale, "kind.log")} title={t(locale, "home.latest")} />
        {columns.length === 0 ? (
          <p className="mt-10 max-w-xl text-lg text-text-muted">{t(locale, "home.noneLatest")}</p>
        ) : (
          <div className="mt-16 grid gap-16 lg:grid-cols-2 lg:gap-8">
            {columns.map((col) => (
              <section key={col.id} aria-labelledby={`${col.id}-title`} className="min-w-0">
                <div className="flex items-end justify-between gap-4">
                  <h3 id={`${col.id}-title`} className="text-2xl font-bold tracking-tight">
                    {t(locale, col.title)}
                  </h3>
                  {col.more && (
                    <a href={col.more.href} className="bl-meta rounded-sm text-accent underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-accent">
                      {t(locale, col.more.label)}
                    </a>
                  )}
                </div>
                <ul className="mt-6 border-t border-border">
                  {col.cards.map((c) => (
                    <li key={`${c.kind}-${c.slug}`} className="border-b border-border">
                      <a href={contentPath(locale, c.kind, c.slug, c.project?.slug) ?? "#"} className="group grid gap-2 py-5 focus-visible:outline-2 focus-visible:outline-accent sm:grid-cols-[9rem_minmax(0,1fr)] sm:gap-6">
                        <span className="font-mono text-xs text-text-muted">
                          <time dateTime={isoDay(c.first_published_at)}>{formatDay(locale, c.first_published_at)}</time>
                        </span>
                        <span className="min-w-0">
                          {c.project && c.kind === "log" && <span className="bl-meta block text-text-muted">{c.project.title}</span>}
                          <span className="mt-1 block text-lg leading-snug font-medium [overflow-wrap:anywhere] group-hover:text-accent">{c.title}</span>
                        </span>
                      </a>
                    </li>
                  ))}
                </ul>
              </section>
            ))}
          </div>
        )}
      </Frame>
    </section>
  );
}

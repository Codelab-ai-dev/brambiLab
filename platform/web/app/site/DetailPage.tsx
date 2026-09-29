// Project, log and article pages (WEB-009): navy header with the big title, the cover with
// controlled prominence (full colour, never cropped), the text on a paper surface at a moderate
// reading width, the project's technical sheet beside it, the log and real related links.
import { TechnicalMeta, type MetaRow } from "~/components/site/editorial";
import { DocumentView } from "~/content/DocumentView";
import { t, type Locale, type MessageKey } from "~/i18n";
import type { DetailData } from "./detail.server";
import { projectPath, sectionPath } from "./paths";
import { ContentList, Frame, Pager, PublishedDate, StatusBadge, TermList } from "./ui";

function MissingTranslation({ locale, href }: { locale: Locale; href: string }) {
  const other = locale === "es" ? "en" : "es";
  return (
    <p className="mt-8 text-sm text-text-muted" data-translation="missing">
      {t(locale, "switch.missingNotice")}{" "}
      <a href={href} hrefLang={other} lang={other} className="text-accent underline underline-offset-2">
        {t(locale, "switch.missingLink")}
      </a>
    </p>
  );
}

const relatedLink = "inline-flex min-h-12 items-center gap-3 rounded-sm border border-border-strong px-5 font-medium hover:border-accent focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent";

export function DetailPage({ data }: { data: DetailData }) {
  const { locale, content: c, logs, switcher, path } = data;
  const pf = c.project_fields;
  const listSection = c.kind === "article" ? "articles" : "projects";
  const filterHref = (kind: "category" | "tag", slug: string) => `${sectionPath(locale, listSection)}?${kind}=${slug}`;
  const updated = c.published_at.slice(0, 10) !== c.first_published_at.slice(0, 10);
  const facts: MetaRow[] = [];
  if (pf?.status) facts.push({ label: t(locale, "project.status"), value: t(locale, `status.${pf.status}` as MessageKey) });
  if (pf?.technologies?.length) facts.push({ label: t(locale, "project.technologies"), value: pf.technologies.join(" · ") });
  if (c.category) facts.push({ label: t(locale, "filter.category"), value: c.category.label });
  const sheet = c.kind === "project" && pf && (facts.length > 0 || pf.objective || pf.links?.length || pf.results);

  return (
    <main>
      <article>
        <header className="bl-navy border-b border-border">
          <Frame className="py-14 sm:py-20">
            {c.kind === "log" && c.project && (
              <nav aria-label={t(locale, "log.project")} className="mb-8">
                <a href={projectPath(locale, c.project.slug)} className="bl-meta rounded-xs text-accent hover:underline focus-visible:outline-2 focus-visible:outline-accent">
                  ← {c.project.title}
                </a>
              </nav>
            )}
            <div className="flex flex-wrap items-center gap-3">
              <span className="bl-meta text-accent">{t(locale, `kind.${c.kind}` as MessageKey)}</span>
              {c.kind === "project" && <StatusBadge locale={locale} status={pf?.status} />}
            </div>
            <h1 className="bl-h-project mt-6 max-w-[20ch]">{c.title}</h1>
            {c.summary && <p className="mt-8 max-w-3xl text-lg leading-relaxed text-text-muted sm:text-xl">{c.summary}</p>}
            <div className="mt-10 flex flex-wrap items-center gap-x-6 gap-y-3 border-t border-border pt-6">
              <PublishedDate locale={locale} iso={c.first_published_at} />
              {updated && <PublishedDate locale={locale} iso={c.published_at} label="date.updated" />}
              <TermList locale={locale} category={c.category} tags={c.tags} hrefFor={filterHref} />
            </div>
            {!switcher.available && <MissingTranslation locale={locale} href={switcher.href} />}
          </Frame>
          {c.cover && (
            <Frame className="pb-14 sm:pb-20">
              {/* The file's own proportions, in colour: a cover may carry wiring or diagrams. */}
              <img
                src={`/media/${c.cover.asset_id}`}
                alt={c.cover.alt}
                width={c.cover.width ?? undefined}
                height={c.cover.height ?? undefined}
                fetchPriority="high"
                decoding="async"
                className="mx-auto h-auto max-h-[75vh] w-auto max-w-full bg-deep object-contain"
              />
            </Frame>
          )}
        </header>

        <div className="bl-light">
          <Frame className="grid gap-12 py-14 sm:py-20 lg:grid-cols-12 lg:gap-8">
            {sheet && (
              <aside aria-labelledby="facts-title" className="flex h-fit flex-col gap-8 lg:sticky lg:top-24 lg:col-span-4 lg:border-r lg:border-border lg:pr-8">
                <h2 id="facts-title" className="bl-meta text-text-muted">
                  {t(locale, "detail.facts")}
                </h2>
                <TechnicalMeta rows={facts} />
                {pf?.objective && (
                  <section>
                    <h3 className="bl-meta text-text-muted">{t(locale, "project.objective")}</h3>
                    <p className="mt-3 leading-relaxed whitespace-pre-line">{pf.objective}</p>
                  </section>
                )}
                {pf?.links && pf.links.length > 0 && (
                  <section>
                    <h3 className="bl-meta text-text-muted">{t(locale, "project.links")}</h3>
                    <ul className="mt-3 flex flex-col gap-2">
                      {pf.links.map((l) => (
                        <li key={l.url}>
                          <a href={l.url} rel="noopener noreferrer" className="break-all text-accent underline underline-offset-2">
                            {l.label}
                          </a>
                        </li>
                      ))}
                    </ul>
                  </section>
                )}
                {pf?.results && (
                  <section>
                    <h3 className="bl-meta text-text-muted">{t(locale, "project.results")}</h3>
                    <p className="mt-3 leading-relaxed whitespace-pre-line">{pf.results}</p>
                  </section>
                )}
              </aside>
            )}
            <div className={`bl-prose min-w-0 text-lg ${sheet ? "lg:col-span-8" : "lg:col-span-8 lg:col-start-3"}`}>
              <div className="max-w-[34em]">
                <DocumentView doc={c.body} locale={locale} assets={c.assets} publicOnly />
              </div>
            </div>
          </Frame>
        </div>
      </article>

      {logs && (
        <section id="bitacora" aria-labelledby="log-title" className="border-b border-border">
          <Frame className="py-16 sm:py-24">
            <h2 id="log-title" className="bl-h-project">
              {t(locale, "project.log")} <span className="font-mono text-lg font-normal tracking-normal text-text-muted">({logs.total})</span>
            </h2>
            <div className="mt-10">
              {logs.items.length > 0 ? <ContentList locale={locale} cards={logs.items} /> : <p className="text-lg text-text-muted">{t(locale, "project.noLogs")}</p>}
            </div>
            <Pager locale={locale} page={logs.page} pageSize={logs.page_size} total={logs.total} path={path} query={new URLSearchParams()} />
          </Frame>
        </section>
      )}

      {/* Only destinations that exist: the index, the project of a log, its category. */}
      <nav aria-label={t(locale, "detail.related")}>
        <Frame className="flex flex-wrap gap-4 py-12">
          {c.kind === "log" && c.project && (
            <a href={projectPath(locale, c.project.slug)} className={relatedLink}>
              ← {t(locale, "log.back")}
            </a>
          )}
          <a href={sectionPath(locale, listSection)} className={relatedLink}>
            {t(locale, listSection === "articles" ? "detail.allArticles" : "detail.allProjects")} →
          </a>
          {c.category && c.kind !== "log" && (
            <a href={filterHref("category", c.category.slug)} className={relatedLink}>
              {t(locale, "filter.category")}: {c.category.label} →
            </a>
          )}
        </Frame>
      </nav>
    </main>
  );
}

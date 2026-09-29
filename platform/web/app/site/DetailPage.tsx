import { DocumentView } from "~/content/DocumentView";
import { t, type Locale, type MessageKey } from "~/i18n";
import type { DetailData } from "./detail.server";
import { projectPath, sectionPath } from "./paths";
import { CardGrid, Container, Pager, PublishedDate, StatusBadge, TermList } from "./ui";

function MissingTranslation({ locale, href }: { locale: Locale; href: string }) {
  const other = locale === "es" ? "en" : "es";
  return (
    <p className="mt-6 text-sm text-text-muted" data-translation="missing">
      {t(locale, "switch.missingNotice")}{" "}
      <a href={href} hrefLang={other} lang={other} className="text-accent underline underline-offset-2">
        {t(locale, "switch.missingLink")}
      </a>
    </p>
  );
}

export function DetailPage({ data }: { data: DetailData }) {
  const { locale, content: c, logs, switcher, path } = data;
  const pf = c.project_fields;
  const listSection = c.kind === "article" ? "articles" : "projects";
  const filterHref = (kind: "category" | "tag", slug: string) => `${sectionPath(locale, listSection)}?${kind}=${slug}`;
  const updated = c.published_at.slice(0, 10) !== c.first_published_at.slice(0, 10);

  return (
    <main>
      <article>
        <header className="border-b border-border bg-surface-muted">
          <Container className="py-10 sm:py-14">
            {c.kind === "log" && c.project && (
              <nav aria-label={t(locale, "log.project")} className="mb-4 text-sm">
                <a href={projectPath(locale, c.project.slug)} className="text-accent underline underline-offset-2">
                  ← {c.project.title}
                </a>
              </nav>
            )}
            <div className="flex flex-wrap items-center gap-3">
              <span className="font-mono text-xs tracking-widest text-accent uppercase">{t(locale, `kind.${c.kind}` as MessageKey)}</span>
              {c.kind === "project" && <StatusBadge locale={locale} status={pf?.status} />}
            </div>
            <h1 className="mt-3 max-w-4xl text-3xl font-semibold tracking-tight break-words sm:text-5xl">{c.title}</h1>
            {c.summary && <p className="mt-4 max-w-3xl text-lg text-text-muted">{c.summary}</p>}
            <div className="mt-6 flex flex-wrap items-center gap-x-6 gap-y-3">
              <PublishedDate locale={locale} iso={c.first_published_at} />
              {updated && <PublishedDate locale={locale} iso={c.published_at} label="date.updated" />}
              <TermList locale={locale} category={c.category} tags={c.tags} hrefFor={filterHref} />
            </div>
            {!switcher.available && <MissingTranslation locale={locale} href={switcher.href} />}
          </Container>
        </header>

        <Container className="grid gap-10 py-10 lg:grid-cols-[minmax(0,1fr)_18rem]">
          <div className="min-w-0">
            {c.cover && (
              <img
                src={`/media/${c.cover.asset_id}`}
                alt={c.cover.alt}
                width={c.cover.width ?? undefined}
                height={c.cover.height ?? undefined}
                fetchPriority="high"
                decoding="async"
                className="mb-8 h-auto w-full rounded-lg border border-border"
              />
            )}
            <div className="bl-prose max-w-3xl text-lg">
              <DocumentView doc={c.body} locale={locale} assets={c.assets} publicOnly />
            </div>
          </div>

          {c.kind === "project" && pf && (
            <aside aria-label={t(locale, "project.status")} className="flex h-fit flex-col gap-5 rounded-lg border border-border p-5 text-sm lg:sticky lg:top-6">
              {pf.objective && (
                <section>
                  <h2 className="font-mono text-xs tracking-widest text-text-muted uppercase">{t(locale, "project.objective")}</h2>
                  <p className="mt-2 leading-relaxed whitespace-pre-line">{pf.objective}</p>
                </section>
              )}
              {pf.status && (
                <section>
                  <h2 className="font-mono text-xs tracking-widest text-text-muted uppercase">{t(locale, "project.status")}</h2>
                  <p className="mt-2">
                    <StatusBadge locale={locale} status={pf.status} />
                  </p>
                </section>
              )}
              {pf.technologies && pf.technologies.length > 0 && (
                <section>
                  <h2 className="font-mono text-xs tracking-widest text-text-muted uppercase">{t(locale, "project.technologies")}</h2>
                  <ul className="mt-2 flex flex-wrap gap-1.5">
                    {pf.technologies.map((tech) => (
                      <li key={tech} className="rounded bg-surface-muted px-2 py-0.5 font-mono text-xs">
                        {tech}
                      </li>
                    ))}
                  </ul>
                </section>
              )}
              {pf.links && pf.links.length > 0 && (
                <section>
                  <h2 className="font-mono text-xs tracking-widest text-text-muted uppercase">{t(locale, "project.links")}</h2>
                  <ul className="mt-2 flex flex-col gap-1">
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
              {pf.results && (
                <section>
                  <h2 className="font-mono text-xs tracking-widest text-text-muted uppercase">{t(locale, "project.results")}</h2>
                  <p className="mt-2 leading-relaxed whitespace-pre-line">{pf.results}</p>
                </section>
              )}
            </aside>
          )}
        </Container>
      </article>

      {logs && (
        <section aria-labelledby="log-title" className="border-t border-border">
          <Container className="py-12">
            <h2 id="log-title" className="text-2xl font-semibold tracking-tight">
              {t(locale, "project.log")} <span className="font-mono text-sm text-text-muted">({logs.total})</span>
            </h2>
            <div className="mt-6">
              {logs.items.length > 0 ? <CardGrid locale={locale} cards={logs.items} /> : <p className="text-text-muted">{t(locale, "project.noLogs")}</p>}
            </div>
            <Pager locale={locale} page={logs.page} pageSize={logs.page_size} total={logs.total} path={path} query={new URLSearchParams()} />
          </Container>
        </section>
      )}

      {c.kind === "log" && c.project && (
        <Container className="pb-12">
          <a href={projectPath(locale, c.project.slug)} className="text-sm font-medium text-accent underline underline-offset-4">
            ← {t(locale, "log.back")}
          </a>
        </Container>
      )}
    </main>
  );
}

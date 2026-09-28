// Public site building blocks in the "cuaderno de laboratorio" style: paper sections, ink text,
// amber signal, IBM Plex Mono for data. Plain links and GET forms: everything works without JS.
import type { ReactNode } from "react";
import { formatDay, isoDay, t, type Locale, type MessageKey } from "~/i18n";
import { contentPath } from "./paths";
import type { Card, Term } from "./types";

export function Container({ children, className = "" }: { children: ReactNode; className?: string }) {
  return <div className={`mx-auto w-full max-w-6xl px-4 sm:px-6 ${className}`}>{children}</div>;
}

export function PageIntro({ eyebrow, title, lead, children }: { eyebrow?: string; title: string; lead?: string; children?: ReactNode }) {
  return (
    <header className="border-b border-border bg-surface-muted">
      <Container className="py-10 sm:py-14">
        {eyebrow && <p className="font-mono text-xs tracking-widest text-accent uppercase">{eyebrow}</p>}
        <h1 className="mt-2 max-w-3xl text-3xl font-semibold tracking-tight break-words sm:text-5xl">{title}</h1>
        {lead && <p className="mt-4 max-w-2xl text-lg text-text-muted">{lead}</p>}
        {children}
      </Container>
    </header>
  );
}

export function Empty({ children }: { children: ReactNode }) {
  return (
    <div className="rounded-lg border border-dashed border-border px-6 py-10 text-center text-text-muted" role="status">
      {children}
    </div>
  );
}

export function PublishedDate({ locale, iso, label = "date.published" }: { locale: Locale; iso: string; label?: MessageKey }) {
  return (
    <span className="font-mono text-xs text-text-muted">
      {t(locale, label)} <time dateTime={isoDay(iso)}>{formatDay(locale, iso)}</time>
    </span>
  );
}

export function TermList({ category, tags, hrefFor }: { locale?: Locale; category: Term | null; tags: Term[]; hrefFor?: (kind: "category" | "tag", slug: string) => string }) {
  if (!category && tags.length === 0) return null;
  const chip = "rounded border border-border px-2 py-0.5 font-mono text-xs text-text-muted";
  const link = (kind: "category" | "tag", term: Term) =>
    hrefFor ? (
      <a href={hrefFor(kind, term.slug)} className={`${chip} hover:border-accent hover:text-text`}>
        {term.label}
      </a>
    ) : (
      <span className={chip}>{term.label}</span>
    );
  return (
    <ul className="flex flex-wrap gap-1.5">
      {category && <li>{link("category", category)}</li>}
      {tags.map((tg) => (
        <li key={tg.slug}>{link("tag", tg)}</li>
      ))}
    </ul>
  );
}

export function StatusBadge({ locale, status }: { locale: Locale; status?: string }) {
  if (!status) return null;
  return (
    <span className="inline-flex items-center gap-1.5 rounded bg-primary px-2 py-0.5 font-mono text-xs text-primary-contrast">
      <span aria-hidden="true" className="size-1.5 rounded-full bg-accent" />
      {t(locale, `status.${status}` as MessageKey)}
    </span>
  );
}

/** A card links to its detail; the whole card is not a link, so text stays selectable. */
export function ContentCard({ locale, card, headingLevel = 3 }: { locale: Locale; card: Card; headingLevel?: 2 | 3 }) {
  const href = contentPath(locale, card.kind, card.slug, card.project?.slug) ?? "#";
  const H = headingLevel === 2 ? "h2" : "h3";
  return (
    <article className="group flex w-full flex-col overflow-hidden rounded-lg border border-border bg-surface transition-shadow hover:shadow-[0_0_0_1px_var(--color-accent)]">
      {card.cover && (
        <img
          src={`/media/${card.cover.asset_id}`}
          alt={card.cover.alt}
          width={card.cover.width ?? undefined}
          height={card.cover.height ?? undefined}
          loading="lazy"
          decoding="async"
          className="aspect-[16/9] w-full border-b border-border bg-surface-muted object-cover"
        />
      )}
      <div className="flex flex-1 flex-col gap-3 p-5">
        <div className="flex flex-wrap items-center gap-2">
          <span className="font-mono text-xs tracking-widest text-accent uppercase">{t(locale, `kind.${card.kind}` as MessageKey)}</span>
          {card.kind === "project" && <StatusBadge locale={locale} status={card.project_fields?.status} />}
        </div>
        <H className="text-xl leading-snug font-semibold break-words">
          <a href={href} className="underline-offset-4 group-hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent">
            {card.title}
          </a>
        </H>
        {card.project && card.kind === "log" && (
          <p className="text-sm text-text-muted">
            {t(locale, "log.project")}: {card.project.title}
          </p>
        )}
        {card.snippet && card.snippet.length > 0 ? (
          <p className="text-sm leading-relaxed text-text-muted">
            {/* Plain-text segments from the API; <mark> only around real matches. */}
            {card.snippet.map((s, i) => (s.hit ? <mark key={i} className="rounded-sm bg-accent/20 px-0.5 text-text">{s.text}</mark> : <span key={i}>{s.text}</span>))}
          </p>
        ) : (
          card.summary && <p className="leading-relaxed text-text-muted">{card.summary}</p>
        )}
        {card.project_fields?.technologies && card.project_fields.technologies.length > 0 && (
          <ul className="flex flex-wrap gap-1.5" aria-label={t(locale, "project.technologies")}>
            {card.project_fields.technologies.map((tech) => (
              <li key={tech} className="rounded bg-surface-muted px-2 py-0.5 font-mono text-xs">
                {tech}
              </li>
            ))}
          </ul>
        )}
        <div className="mt-auto flex flex-wrap items-center justify-between gap-2 pt-2">
          <PublishedDate locale={locale} iso={card.first_published_at} />
          <TermList locale={locale} category={card.category} tags={card.tags} />
        </div>
      </div>
    </article>
  );
}

export function CardGrid({ locale, cards, headingLevel = 3, columns = 3 }: { locale: Locale; cards: Card[]; headingLevel?: 2 | 3; columns?: 2 | 3 }) {
  return (
    <ul className={`grid gap-5 sm:grid-cols-2 ${columns === 3 ? "lg:grid-cols-3" : ""}`}>
      {cards.map((c) => (
        <li key={`${c.kind}-${c.slug}`} className="flex">
          <ContentCard locale={locale} card={c} headingLevel={headingLevel} />
        </li>
      ))}
    </ul>
  );
}

/** Previous/next links that keep every other query parameter (filters survive paging). */
export function Pager({ locale, page, pageSize, total, path, query }: { locale: Locale; page: number; pageSize: number; total: number; path: string; query: URLSearchParams }) {
  const pages = Math.max(1, Math.ceil(total / pageSize));
  if (pages <= 1 && page <= 1) return null;
  const href = (n: number) => {
    const q = new URLSearchParams(query);
    if (n > 1) q.set("page", String(n));
    else q.delete("page");
    const s = q.toString();
    return s ? `${path}?${s}` : path;
  };
  const link = "inline-flex min-h-10 items-center rounded-md border border-border-strong px-4 text-sm font-medium hover:bg-surface-muted focus-visible:outline-2 focus-visible:outline-accent";
  return (
    <nav aria-label={t(locale, "pager.label")} className="mt-10 flex items-center justify-between gap-4">
      {page > 1 ? (
        <a href={href(Math.min(page - 1, pages))} rel="prev" className={link}>
          ← {t(locale, "pager.prev")}
        </a>
      ) : (
        <span />
      )}
      <span className="font-mono text-xs text-text-muted">
        {t(locale, "pager.page")} {page} {t(locale, "pager.of")} {pages}
      </span>
      {page < pages ? (
        <a href={href(page + 1)} rel="next" className={link}>
          {t(locale, "pager.next")} →
        </a>
      ) : (
        <span />
      )}
    </nav>
  );
}

const selectClass =
  "min-h-10 rounded-md border border-border-strong bg-surface px-3 text-sm focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-accent";

/** GET form: changing filters starts at page 1 (no page field is sent). */
export function FilterForm({
  locale,
  action,
  categories,
  tags,
  selected,
  kinds,
  children,
}: {
  locale: Locale;
  action: string;
  categories: Term[];
  tags: Term[];
  selected: { category?: string; tag?: string; kind?: string };
  kinds?: boolean;
  children?: ReactNode;
}) {
  const hasFilters = !!(selected.category || selected.tag || selected.kind);
  return (
    <form method="get" action={action} className="flex flex-wrap items-end gap-3" role="search" aria-label={t(locale, "filter.label")}>
      {children}
      {kinds && (
        <label className="flex flex-col gap-1 text-sm font-medium">
          {t(locale, "filter.kind")}
          <select name="kind" defaultValue={selected.kind ?? ""} className={selectClass}>
            <option value="">{t(locale, "filter.anyKind")}</option>
            {(["project", "log", "article"] as const).map((k) => (
              <option key={k} value={k}>
                {t(locale, `kind.${k}`)}
              </option>
            ))}
          </select>
        </label>
      )}
      {categories.length > 0 && (
        <label className="flex flex-col gap-1 text-sm font-medium">
          {t(locale, "filter.category")}
          <select name="category" defaultValue={selected.category ?? ""} className={selectClass}>
            <option value="">{t(locale, "filter.any")}</option>
            {categories.map((c) => (
              <option key={c.slug} value={c.slug}>
                {c.label} ({c.count})
              </option>
            ))}
          </select>
        </label>
      )}
      {tags.length > 0 && (
        <label className="flex flex-col gap-1 text-sm font-medium">
          {t(locale, "filter.tag")}
          <select name="tag" defaultValue={selected.tag ?? ""} className={selectClass}>
            <option value="">{t(locale, "filter.any")}</option>
            {tags.map((c) => (
              <option key={c.slug} value={c.slug}>
                {c.label} ({c.count})
              </option>
            ))}
          </select>
        </label>
      )}
      {(children || kinds || categories.length > 0 || tags.length > 0) && (
        <button type="submit" className="inline-flex min-h-10 items-center rounded-md bg-primary px-4 text-sm font-medium text-primary-contrast hover:opacity-90 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent">
          {children ? t(locale, "search.submit") : t(locale, "filter.apply")}
        </button>
      )}
      {hasFilters && (
        <a href={action} className="min-h-10 self-end py-2 text-sm text-accent underline underline-offset-2">
          {t(locale, "list.clear")}
        </a>
      )}
    </form>
  );
}

// Public site building blocks in the editorial direction (WEB-009): wide frame, big type, mono
// metadata, rows instead of card grids. Plain links and GET forms: everything works without JS.
// Surfaces come from the scope classes in app.css; these components only use semantic tokens.
import type { ReactNode } from "react";
import { Arrow, Eyebrow, Frame } from "~/components/site/editorial";
import { formatDay, isoDay, t, type Locale, type MessageKey } from "~/i18n";
import { contentPath } from "./paths";
import type { Card, Term } from "./types";

export { Frame };

/** Page header of the interior pages: navy band, eyebrow, big title and an optional lead. */
export function PageHeader({ eyebrow, index, title, lead, children }: { eyebrow?: string; index?: string; title: string; lead?: string; children?: ReactNode }) {
  return (
    <header className="bl-navy border-b border-border">
      <Frame className="py-16 sm:py-24">
        {eyebrow && <Eyebrow index={index}>{eyebrow}</Eyebrow>}
        <h1 className="bl-h-section mt-6 max-w-[16ch]">{title}</h1>
        {lead && <p className="mt-8 max-w-2xl text-lg leading-relaxed text-text-muted sm:text-xl">{lead}</p>}
        {children}
      </Frame>
    </header>
  );
}

/** Honest empty state: says what is missing, never pretends. */
export function Empty({ children }: { children: ReactNode }) {
  return (
    <div className="border-y border-border py-12 text-lg text-text-muted" role="status">
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
  const chip = "inline-flex min-h-7 items-center rounded-xs border border-border px-2 font-mono text-[0.6875rem] tracking-[0.08em] text-text-muted uppercase";
  const link = (kind: "category" | "tag", term: Term) =>
    hrefFor ? (
      <a href={hrefFor(kind, term.slug)} className={`${chip} hover:border-accent hover:text-text focus-visible:outline-2 focus-visible:outline-accent`}>
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

// The dot colour is a reinforcement only: the status is always written out.
const statusDot: Record<string, string> = { idea: "bg-text-muted", in_development: "bg-signal", paused: "bg-warning", completed: "bg-success" };

export function StatusBadge({ locale, status }: { locale: Locale; status?: string }) {
  if (!status) return null;
  return (
    <span className="inline-flex min-h-7 items-center gap-2 rounded-xs border border-border-strong px-2 font-mono text-[0.6875rem] tracking-[0.08em] text-text uppercase">
      <span aria-hidden="true" className={`size-1.5 rounded-full ${statusDot[status] ?? "bg-text-muted"}`} />
      {t(locale, `status.${status}` as MessageKey)}
    </span>
  );
}

/**
 * One published item as an editorial row: date and kind, a big linked title, summary (or search
 * snippet), technical facts and an optional desaturated thumbnail. The row is not one big link,
 * so its text stays selectable; the title is the link.
 */
export function ContentRow({ locale, card, headingLevel = 3 }: { locale: Locale; card: Card; headingLevel?: 2 | 3 }) {
  const href = contentPath(locale, card.kind, card.slug, card.project?.slug) ?? "#";
  const H = headingLevel === 2 ? "h2" : "h3";
  const tech = card.project_fields?.technologies ?? [];
  return (
    <article className="group grid gap-x-8 gap-y-4 border-b border-border py-8 sm:grid-cols-6 lg:grid-cols-12">
      <div className="flex flex-wrap items-center gap-x-4 gap-y-2 sm:col-span-6 lg:col-span-2 lg:flex-col lg:items-start">
        <span className="bl-meta text-accent">{t(locale, `kind.${card.kind}` as MessageKey)}</span>
        <span className="font-mono text-xs text-text-muted">
          <time dateTime={isoDay(card.first_published_at)}>{formatDay(locale, card.first_published_at)}</time>
        </span>
      </div>
      <div className={`min-w-0 sm:col-span-6 ${card.cover ? "lg:col-span-6" : "lg:col-span-7"}`}>
        {card.project && card.kind === "log" && <p className="bl-meta mb-2 text-text-muted">{card.project.title}</p>}
        <H className="text-[clamp(1.5rem,2.6vw,2.5rem)] leading-[1.05] font-bold tracking-[-0.02em] [overflow-wrap:anywhere]">
          <a href={href} className="rounded-xs underline-offset-[0.18em] decoration-2 group-hover:underline hover:decoration-signal focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-accent">
            {card.title}
          </a>
        </H>
        {card.snippet && card.snippet.length > 0 ? (
          <p className="mt-4 max-w-2xl leading-relaxed text-text-muted">
            {/* Plain-text segments from the API; <mark> only around real matches. */}
            {card.snippet.map((s, i) => (s.hit ? <mark key={i} className="rounded-xs bg-signal/25 px-0.5 text-text">{s.text}</mark> : <span key={i}>{s.text}</span>))}
          </p>
        ) : (
          card.summary && <p className="mt-4 max-w-2xl text-lg leading-relaxed text-text-muted">{card.summary}</p>
        )}
      </div>
      <div className={`flex min-w-0 flex-col items-start gap-3 sm:col-span-6 ${card.cover ? "lg:col-span-2" : "lg:col-span-3"}`}>
        {card.kind === "project" && <StatusBadge locale={locale} status={card.project_fields?.status} />}
        {tech.length > 0 && (
          <p className="font-mono text-xs text-text-muted">
            <span className="sr-only">{t(locale, "project.technologies")}: </span>
            {tech.join(" · ")}
          </p>
        )}
        <TermList locale={locale} category={card.category} tags={card.tags} />
      </div>
      {card.cover && (
        <a href={href} tabIndex={-1} aria-hidden="true" className="block overflow-hidden bg-surface-muted sm:col-span-3 lg:col-span-2" style={{ aspectRatio: "4 / 3" }}>
          <img
            src={`/media/${card.cover.asset_id}`}
            alt=""
            width={card.cover.width ?? undefined}
            height={card.cover.height ?? undefined}
            loading="lazy"
            decoding="async"
            className="bl-photo h-full w-full object-cover"
          />
        </a>
      )}
    </article>
  );
}

export function ContentList({ locale, cards, headingLevel = 3 }: { locale: Locale; cards: Card[]; headingLevel?: 2 | 3 }) {
  return (
    <ul className="border-t border-border">
      {cards.map((c) => (
        <li key={`${c.kind}-${c.slug}`}>
          <ContentRow locale={locale} card={c} headingLevel={headingLevel} />
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
  const link = "inline-flex min-h-12 items-center gap-3 rounded-sm border border-border-strong px-5 font-medium hover:border-accent focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent";
  return (
    <nav aria-label={t(locale, "pager.label")} className="mt-12 flex items-center justify-between gap-4">
      {page > 1 ? (
        <a href={href(Math.min(page - 1, pages))} rel="prev" className={link}>
          ← {t(locale, "pager.prev")}
        </a>
      ) : (
        <span />
      )}
      <span className="bl-meta text-text-muted">
        {t(locale, "pager.page")} <span className="text-text">{page}</span> {t(locale, "pager.of")} {pages}
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

const fieldLabel = "flex min-w-0 flex-col gap-2 bl-meta text-text-muted";
const control =
  "min-h-12 w-full rounded-sm border border-border-strong bg-surface px-3 font-sans text-base tracking-normal text-text normal-case focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-accent";

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
    <form method="get" action={action} className={`flex flex-wrap items-end gap-x-4 gap-y-5 ${children ? "lg:pb-8" : ""}`} role="search" aria-label={t(locale, "filter.label")}>
      {children}
      {kinds && (
        <label className={`${fieldLabel} w-full sm:w-48`}>
          {t(locale, "filter.kind")}
          <select name="kind" defaultValue={selected.kind ?? ""} className={control}>
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
        <label className={`${fieldLabel} w-full sm:w-56`}>
          {t(locale, "filter.category")}
          <select name="category" defaultValue={selected.category ?? ""} className={control}>
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
        <label className={`${fieldLabel} w-full sm:w-56`}>
          {t(locale, "filter.tag")}
          <select name="tag" defaultValue={selected.tag ?? ""} className={control}>
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
        <button type="submit" className="inline-flex min-h-12 items-center gap-3 rounded-sm bg-primary px-5 font-medium text-primary-contrast hover:bg-white focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-signal">
          {children ? t(locale, "search.submit") : t(locale, "filter.apply")}
          <Arrow direction="right" />
        </button>
      )}
      {hasFilters && (
        <a href={action} className="inline-flex min-h-12 items-center text-sm text-accent underline underline-offset-4 focus-visible:outline-2 focus-visible:outline-accent">
          {t(locale, "list.clear")}
        </a>
      )}
    </form>
  );
}

/** Input style shared with the search field. */
export const textControl = control;
export const textLabel = fieldLabel;

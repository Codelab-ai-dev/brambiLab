// Featured projects as wide editorial blocks (WEB-009), not a grid of cards. Everything shown is
// what the public API publishes: a project without a cover gets a text composition (no stand-in
// photo), and metadata rows appear only when the field has a value.
import { formatDay, isoDay, t, type Locale, type MessageKey } from "~/i18n";
import { contentPath } from "~/site/paths";
import type { Card } from "~/site/types";
import { Arrow, Eyebrow, Photo, TechnicalMeta, type MetaRow } from "./editorial";

const pad = (n: number) => String(n).padStart(2, "0");
export const featureId = (card: Card) => `project-${card.slug}`;

export function ProjectFeature({ locale, card, position, total }: { locale: Locale; card: Card; position: number; total: number }) {
  const href = contentPath(locale, card.kind, card.slug, card.project?.slug) ?? "#";
  const tech = card.project_fields?.technologies ?? [];
  const status = card.project_fields?.status;
  const rows: MetaRow[] = [];
  if (status) rows.push({ label: t(locale, "project.status"), value: t(locale, `status.${status}` as MessageKey) });
  if (tech.length > 0) rows.push({ label: t(locale, "project.technologies"), value: tech.join(" · ") });
  if (card.category) rows.push({ label: t(locale, "filter.category"), value: card.category.label });
  rows.push({ label: t(locale, "date.published"), value: <time dateTime={isoDay(card.first_published_at)}>{formatDay(locale, card.first_published_at)}</time> });
  const titleId = `${featureId(card)}-title`;

  return (
    <article id={featureId(card)} aria-labelledby={titleId} className="group grid gap-10 border-t border-border py-14 first:border-t-0 sm:py-20 lg:grid-cols-12 lg:gap-8">
      <div className={`flex min-w-0 flex-col ${card.cover ? "lg:col-span-5" : "lg:col-span-8"}`}>
        <Eyebrow index={pad(position)}>{card.category?.label ?? t(locale, "kind.project")}</Eyebrow>
        <h3 id={titleId} className="bl-h-project mt-6">
          {card.title}
        </h3>
        {card.summary && <p className="mt-6 max-w-xl text-lg leading-relaxed text-text-muted">{card.summary}</p>}
        <div className="mt-8 flex flex-wrap items-center gap-x-6 gap-y-4">
          <a
            href={href}
            className="inline-flex min-h-12 items-center gap-3 rounded-sm bg-primary px-5 font-medium text-primary-contrast focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-accent"
          >
            {t(locale, "home.viewProject")}
            <span className="sr-only">: {card.title}</span>
            <Arrow />
          </a>
          {tech.length > 0 && (
            <p className="flex items-center gap-4 font-mono text-xs text-text-muted">
              <span aria-hidden="true" className="h-px w-12 bg-current" />
              {tech.slice(0, 4).join(" · ")}
            </p>
          )}
        </div>
      </div>

      {card.cover && (
        <a href={href} tabIndex={-1} aria-hidden="true" className="block min-w-0 lg:col-span-4">
          <Photo cover={card.cover} />
        </a>
      )}

      <div className={`flex min-w-0 flex-col justify-between gap-8 border-t border-border pt-6 lg:border-t-0 lg:border-l lg:pt-0 lg:pl-6 ${card.cover ? "lg:col-span-3" : "lg:col-span-4"}`}>
        <TechnicalMeta rows={rows} />
        <p className="font-mono text-xs text-text-muted">
          <span aria-hidden="true">
            <span className="text-text">{pad(position)}</span> — {pad(total)}
          </span>
          <span className="sr-only">{t(locale, "home.position").replace("{n}", String(position)).replace("{total}", String(total))}</span>
        </p>
      </div>
    </article>
  );
}

/** In-page index of the featured blocks: every entry is a real destination on this page. */
export function ProjectNavigation({ locale, cards }: { locale: Locale; cards: Card[] }) {
  return (
    <nav aria-label={t(locale, "home.projectsIndex")}>
      <ol className="border-t border-border">
        {cards.map((c, i) => (
          <li key={c.slug} className="border-b border-border">
            <a href={`#${featureId(c)}`} className="group flex min-h-12 items-center gap-4 py-3 font-mono text-xs tracking-[0.12em] uppercase hover:text-accent focus-visible:outline-2 focus-visible:outline-accent">
              <span className="text-accent">{pad(i + 1)}</span>
              <span className="min-w-0 flex-1 [overflow-wrap:anywhere]">{c.title}</span>
              <Arrow direction="right" />
            </a>
          </li>
        ))}
      </ol>
    </nav>
  );
}

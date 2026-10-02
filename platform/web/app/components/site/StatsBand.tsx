// The one full-cyan band of the site (WEB-009). Only real counts: the three areas described on
// this page and, once there are enough to read as a figure, the public projects in this language
// (the API total, never a page length; drafts and other translations are not counted).
import { t, type Locale } from "~/i18n";
import { Frame } from "./editorial";

/** Below this, a count is not a meaningful figure and the item is left out. */
export const MIN_PROJECTS_SHOWN = 3;

export function StatsBand({ locale, areas, projects }: { locale: Locale; areas: number; projects: number }) {
  const items = [{ n: areas, label: t(locale, "stats.areas") }];
  if (projects >= MIN_PROJECTS_SHOWN) items.push({ n: projects, label: t(locale, "stats.projects") });
  return (
    <section aria-label={t(locale, "stats.label")} className="bl-signal" data-stats>
      <Frame className="grid gap-10 py-12 sm:py-14 lg:grid-cols-12 lg:items-end">
        {/* The cyan band itself never fades: a sudden brightness change is avoided (#52). */}
        <dl data-reveal className="flex flex-wrap gap-x-14 gap-y-8 lg:col-span-7">
          {items.map((i) => (
            <div key={i.label} className="flex flex-col-reverse">
              <dt className="bl-meta mt-3 border-t border-current pt-2 text-text-muted">{i.label}</dt>
              <dd className="bl-h-section leading-none">{i.n}</dd>
            </div>
          ))}
        </dl>
        <p data-reveal className="max-w-[28ch] text-2xl leading-snug font-medium tracking-tight sm:text-3xl lg:col-span-5">{t(locale, "stats.method")}</p>
      </Frame>
    </section>
  );
}

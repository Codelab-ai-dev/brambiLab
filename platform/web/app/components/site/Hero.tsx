// Public home hero (WEB-009): asymmetric composition with the title block, a metadata column and a
// visual column. The visual is an original conceptual illustration until an authorised photograph
// exists; it is labelled as such. Everything is static and complete in the server HTML.
import { Fragment, type CSSProperties } from "react";
import { t, type Locale, type MessageKey } from "~/i18n";
import { sectionPath } from "~/site/paths";
import { Arrow, Frame } from "./editorial";
import { RidgeTexture } from "./textures";

const focus: MessageKey[] = ["hero.focus.autonomy", "hero.focus.connected", "hero.focus.ai", "hero.focus.edge"];

export function Hero({ locale }: { locale: Locale }) {
  // The cyan word carries the idea: "Intelligent" in English, "inteligentes" in Spanish.
  const lit = locale === "en" ? 2 : 3;
  // Widest line in em (Space Grotesk 700, -0.04em) plus a margin: "Ingeniería para" 6.63,
  // "Engineering" 5.32, measured in Chromium with the served font; see .bl-display.
  const measure = locale === "en" ? 5.45 : 6.8;
  const lines: MessageKey[] = ["hero.line1", "hero.line2", "hero.line3"];
  return (
    <section aria-labelledby="hero-title" className="bl-navy relative border-b border-border">
      <Frame className="grid lg:grid-cols-12 lg:pr-0">
        <div className="bl-fit min-w-0 py-14 sm:py-20 lg:col-span-7 lg:py-24 lg:pr-8">
          <h1 id="hero-title" className="bl-display" style={{ "--bl-measure": measure } as CSSProperties}>
            {lines.map((key, i) => (
              <span key={key} className={`block ${i + 1 === lit ? "text-signal" : ""}`}>
                {t(locale, key)}
                {i < lines.length - 1 ? " " : ""}
              </span>
            ))}
          </h1>
          <p className="mt-8 font-mono text-sm tracking-[0.14em] text-text sm:text-base sm:tracking-[0.22em]">
            {/* Break only between fields, never inside one. */}
            {t(locale, "hero.fields")
              .split(" · ")
              .map((f, i) => (
                <Fragment key={f}>
                  {i > 0 && " · "}
                  <span className="whitespace-nowrap">{f}</span>
                </Fragment>
              ))}
          </p>
          <a
            href={sectionPath(locale, "projects")}
            className="mt-10 inline-flex min-h-12 items-center gap-3 rounded-sm bg-primary px-5 font-medium text-primary-contrast hover:bg-white focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-signal"
          >
            {t(locale, "hero.cta")}
            <Arrow />
          </a>
        </div>

        <div className="grid min-w-0 gap-8 border-t border-border py-10 [overflow-wrap:anywhere] sm:grid-cols-2 lg:col-span-2 lg:flex lg:flex-col lg:justify-between lg:border-t-0 lg:py-24 lg:pr-6">
          <p className="bl-meta max-w-[22ch] text-text">{t(locale, "hero.statement")}</p>
          <span aria-hidden="true" className="hidden h-16 w-px bg-signal lg:block" />
          <div>
            <p id="hero-focus" className="bl-meta text-text-muted">
              {t(locale, "hero.focus")}
            </p>
            <ul aria-labelledby="hero-focus" className="bl-meta mt-3 space-y-1 text-text">
              {focus.map((f) => (
                <li key={f}>{t(locale, f)}</li>
              ))}
            </ul>
          </div>
        </div>

        <figure className="relative -mx-4 h-64 overflow-hidden border-t border-border bg-deep text-text-muted sm:-mx-6 sm:h-80 lg:col-span-3 lg:mx-0 lg:h-auto lg:min-h-[36rem] lg:border-t-0 lg:border-l">
          <RidgeTexture className="absolute inset-0 h-full w-full [--color-surface:var(--color-deep)]" />
          <svg viewBox="0 0 24 24" aria-hidden="true" className="absolute right-4 bottom-4 size-6 text-signal" fill="none" stroke="currentColor" strokeWidth="1">
            <path d="M0 12h24M12 0v24" />
            <circle cx="12" cy="12" r="6" />
          </svg>
          <figcaption className="bl-meta absolute bottom-3 left-3 bg-deep px-1.5 py-0.5 text-[0.6875rem] text-text-muted">{t(locale, "visual.conceptual")}</figcaption>
        </figure>
      </Frame>
    </section>
  );
}

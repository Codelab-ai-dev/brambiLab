import type { Route } from "./+types/home";
import { defaultLocale, isLocale, t, type Locale, type MessageKey } from "~/i18n";

export function meta({ loaderData }: Route.MetaArgs) {
  return [
    { title: "BrambiLab" },
    { name: "description", content: t(loaderData.locale, "site.tagline") },
    // Provisional home: keep it out of search indexes until WEB-006 defines SEO.
    { name: "robots", content: "noindex" },
  ];
}

export function loader({ params }: Route.LoaderArgs) {
  return { locale: isLocale(params.lang) ? params.lang : defaultLocale };
}

const areas: MessageKey[] = ["site.area.software", "site.area.ai", "site.area.mcu", "site.area.iot", "site.area.robotics"];
const sections: { n: string; title: MessageKey; text: MessageKey }[] = [
  { n: "01", title: "site.publish.projects.title", text: "site.publish.projects.text" },
  { n: "02", title: "site.publish.log.title", text: "site.publish.log.text" },
  { n: "03", title: "site.publish.articles.title", text: "site.publish.articles.text" },
];
const method: MessageKey[] = ["site.method.hypothesis", "site.method.failures", "site.method.evidence"];

export default function Home({ loaderData }: Route.ComponentProps) {
  const locale: Locale = loaderData.locale;
  return (
    <main>
      <section className="bl-grid border-b border-border">
        <div className="mx-auto max-w-5xl px-4 py-16 sm:px-6 sm:py-24">
          <p className="inline-flex items-center gap-2 rounded-full border border-border bg-surface px-3 py-1 font-mono text-xs text-text-muted">
            <span aria-hidden="true" className="size-2 rounded-full bg-accent" />
            {t(locale, "site.status")}
          </p>
          <h1 className="mt-6 max-w-3xl text-4xl leading-tight font-semibold text-balance sm:text-5xl">{t(locale, "site.tagline")}</h1>
          <p className="mt-6 max-w-2xl text-lg text-text-muted">{t(locale, "site.lab")}</p>
          <div className="mt-8">
            <h2 className="sr-only">{t(locale, "site.areas")}</h2>
            <ul className="flex flex-wrap gap-2">
              {areas.map((a) => (
                <li key={a} className="rounded border border-border-strong bg-surface px-2.5 py-1 font-mono text-xs">
                  {t(locale, a)}
                </li>
              ))}
            </ul>
          </div>
        </div>
      </section>

      <section aria-labelledby="publish" className="mx-auto max-w-5xl px-4 py-14 sm:px-6">
        <h2 id="publish" className="font-mono text-xs tracking-widest text-text-muted uppercase">
          {t(locale, "site.publish.title")}
        </h2>
        <ol className="mt-6 grid gap-px overflow-hidden rounded-md border border-border bg-border sm:grid-cols-3">
          {sections.map((s) => (
            <li key={s.n} className="flex flex-col gap-3 bg-surface p-6">
              <div className="flex items-baseline justify-between gap-2">
                <span className="font-mono text-sm text-accent">{s.n}</span>
                <span className="font-mono text-[0.7rem] tracking-wider text-text-muted uppercase">{t(locale, "site.soon")}</span>
              </div>
              <h3 className="text-xl font-semibold">{t(locale, s.title)}</h3>
              <p className="text-sm leading-relaxed text-text-muted">{t(locale, s.text)}</p>
            </li>
          ))}
        </ol>
      </section>

      <section aria-labelledby="method" className="border-t border-border bg-surface-muted">
        <div className="mx-auto max-w-5xl px-4 py-14 sm:px-6">
          <h2 id="method" className="font-mono text-xs tracking-widest text-text-muted uppercase">
            {t(locale, "site.method.title")}
          </h2>
          <ul className="mt-6 grid gap-6 sm:grid-cols-3">
            {method.map((m) => (
              <li key={m} className="border-l-2 border-accent pl-4 text-base">
                {t(locale, m)}
              </li>
            ))}
          </ul>
        </div>
      </section>
    </main>
  );
}

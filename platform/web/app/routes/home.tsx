import type { Route } from "./+types/home";
import { BenchHero } from "~/components/site/BenchHero";
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

const channels: { ch: string; title: MessageKey; text: MessageKey }[] = [
  { ch: "CH1", title: "site.publish.projects.title", text: "site.publish.projects.text" },
  { ch: "CH2", title: "site.publish.log.title", text: "site.publish.log.text" },
  { ch: "CH3", title: "site.publish.articles.title", text: "site.publish.articles.text" },
];
const protocol: MessageKey[] = ["site.method.hypothesis", "site.method.failures", "site.method.evidence"];

export default function Home({ loaderData }: Route.ComponentProps) {
  const locale: Locale = loaderData.locale;
  return (
    <main>
      <BenchHero locale={locale} />

      {/* Output channels: what will be published. Honest status: nothing is on air yet. */}
      <section id="publish" aria-labelledby="publish-title" className="scroll-mt-4">
        <div className="mx-auto max-w-6xl px-4 py-16 sm:px-6 sm:py-20">
          <h2 id="publish-title" className="max-w-2xl text-3xl font-semibold tracking-tight sm:text-4xl">
            {t(locale, "site.publish.title")}
          </h2>
          <ol className="mt-10 grid gap-5 md:grid-cols-3">
            {channels.map((c) => (
              <li key={c.ch} className="group relative flex flex-col overflow-hidden rounded-lg border border-border bg-surface p-6 transition-shadow hover:shadow-[0_0_0_1px_var(--color-accent)]">
                <span aria-hidden="true" className="absolute inset-x-0 top-0 h-1 origin-left scale-x-25 bg-accent transition-transform duration-500 group-hover:scale-x-100" />
                <div className="flex items-center justify-between font-mono text-xs tracking-widest uppercase">
                  <span className="rounded bg-primary px-2 py-0.5 text-primary-contrast">{c.ch}</span>
                  <span className="flex items-center gap-2 text-text-muted">
                    <span aria-hidden="true" className="size-1.5 rounded-full border border-text-muted" />
                    {t(locale, "site.soon")}
                  </span>
                </div>
                <h3 className="mt-6 text-2xl font-semibold">{t(locale, c.title)}</h3>
                <p className="mt-3 leading-relaxed text-text-muted">{t(locale, c.text)}</p>
              </li>
            ))}
          </ol>
        </div>
      </section>

      {/* Test protocol: how everything is documented. */}
      <section aria-labelledby="method-title" className="border-t border-border bg-surface-muted">
        <div className="mx-auto grid max-w-6xl gap-10 px-4 py-16 sm:px-6 sm:py-20 lg:grid-cols-[minmax(0,1fr)_minmax(0,2fr)]">
          <h2 id="method-title" className="text-3xl font-semibold tracking-tight sm:text-4xl">
            {t(locale, "site.method.title")}
          </h2>
          <ol className="flex flex-col divide-y divide-border border-y border-border">
            {protocol.map((m, i) => (
              <li key={m} className="flex items-baseline gap-6 py-5">
                <span className="font-mono text-sm text-accent">{String(i + 1).padStart(2, "0")}</span>
                <p className="text-lg sm:text-xl">{t(locale, m)}</p>
              </li>
            ))}
          </ol>
        </div>
      </section>
    </main>
  );
}

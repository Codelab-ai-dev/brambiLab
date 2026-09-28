import type { Route } from "./+types/home";
import { BenchHero } from "~/components/site/BenchHero";
import { t, type Locale, type MessageKey } from "~/i18n";
import { publicGet } from "~/site/api.server";
import { localeOf, seo } from "~/site/loader.server";
import { homePath, sectionPath } from "~/site/paths";
import { forwardHeaders, seoMeta } from "~/site/seo";
import type { Card, Home as HomeData } from "~/site/types";
import { CardGrid, Container } from "~/site/ui";
import { ContactLinks } from "~/site/ContactLinks";

export const headers = forwardHeaders;

export function meta({ loaderData }: Route.MetaArgs) {
  return seoMeta(loaderData?.seo);
}

export async function loader({ request }: Route.LoaderArgs) {
  const locale = localeOf(request);
  const home = await publicGet<HomeData>(`/api/v1/public/${locale}/home`);
  return {
    locale,
    home,
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

type Channel = { id: string; ch: string; title: MessageKey; empty: MessageKey; cards: Card[]; more?: { href: string; label: MessageKey } };

export default function Home({ loaderData }: Route.ComponentProps) {
  const { home } = loaderData;
  const locale: Locale = loaderData.locale;
  const featured = home.featured.length > 0;
  const channels: Channel[] = [
    {
      id: "projects",
      ch: "CH1",
      title: featured ? "home.featured" : "home.projects",
      empty: "home.noneProjects",
      cards: featured ? home.featured : home.latest_projects,
      more: { href: sectionPath(locale, "projects"), label: "home.allProjects" },
    },
    { id: "logs", ch: "CH2", title: "home.logs", empty: "home.noneLogs", cards: home.latest_logs },
    { id: "articles", ch: "CH3", title: "home.articles", empty: "home.noneArticles", cards: home.latest_articles, more: { href: sectionPath(locale, "articles"), label: "home.allArticles" } },
  ];
  const hasContact = home.site.contact_email !== "" || home.site.links.length > 0;

  return (
    <main>
      <BenchHero locale={locale} />

      {home.site.intro && (
        <section aria-label={t(locale, "about.title")} className="border-b border-border">
          <Container className="py-12">
            <p className="max-w-3xl text-xl leading-relaxed whitespace-pre-line sm:text-2xl">{home.site.intro}</p>
          </Container>
        </section>
      )}

      {/* Output channels: real published content, or an honest empty state per channel. */}
      <div id="publish" className="scroll-mt-4">
        {channels.map((c) => (
          <section key={c.id} aria-labelledby={`${c.id}-title`} className="border-b border-border">
            <Container className="py-14 sm:py-16">
              <div className="flex flex-wrap items-end justify-between gap-4">
                <h2 id={`${c.id}-title`} className="flex items-center gap-3 text-2xl font-semibold tracking-tight sm:text-3xl">
                  <span className="rounded bg-primary px-2 py-0.5 font-mono text-xs tracking-widest text-primary-contrast">{c.ch}</span>
                  {t(locale, c.title)}
                </h2>
                {c.more && c.cards.length > 0 && (
                  <a href={c.more.href} className="text-sm font-medium text-accent underline underline-offset-4">
                    {t(locale, c.more.label)} →
                  </a>
                )}
              </div>
              <div className="mt-8">
                {c.cards.length > 0 ? (
                  <CardGrid locale={locale} cards={c.cards} />
                ) : (
                  <p className="flex items-center gap-2 font-mono text-sm text-text-muted">
                    <span aria-hidden="true" className="size-1.5 rounded-full border border-text-muted" />
                    {t(locale, c.empty)}
                  </p>
                )}
              </div>
            </Container>
          </section>
        ))}
      </div>

      <section aria-labelledby="method-title" className="border-b border-border bg-surface-muted">
        <Container className="grid gap-10 py-16 sm:py-20 lg:grid-cols-[minmax(0,1fr)_minmax(0,2fr)]">
          <h2 id="method-title" className="text-3xl font-semibold tracking-tight sm:text-4xl">
            {t(locale, "site.method.title")}
          </h2>
          <ol className="flex flex-col divide-y divide-border border-y border-border">
            {(["site.method.hypothesis", "site.method.failures", "site.method.evidence"] as MessageKey[]).map((m, i) => (
              <li key={m} className="flex items-baseline gap-6 py-5">
                <span className="font-mono text-sm text-accent">{String(i + 1).padStart(2, "0")}</span>
                <p className="text-lg sm:text-xl">{t(locale, m)}</p>
              </li>
            ))}
          </ol>
        </Container>
      </section>

      <section aria-labelledby="contact-title">
        <Container className="py-14">
          <h2 id="contact-title" className="text-2xl font-semibold tracking-tight">
            {t(locale, "home.contact")}
          </h2>
          <div className="mt-6">{hasContact ? <ContactLinks locale={locale} site={home.site} /> : <p className="text-text-muted">{t(locale, "contact.empty")}</p>}</div>
        </Container>
      </section>
    </main>
  );
}

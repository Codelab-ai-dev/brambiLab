import type { Route } from "./+types/contact";
import { t } from "~/i18n";
import { publicGet } from "~/site/api.server";
import { localeOf, seo } from "~/site/loader.server";
import { sectionPath } from "~/site/paths";
import { forwardHeaders, seoMeta } from "~/site/seo";
import type { PublicSite } from "~/site/types";
import { ContactLinks } from "~/site/ContactLinks";
import { Container, Empty, PageIntro } from "~/site/ui";
import { ContactForm, contactStates, type ContactState } from "~/site/ContactForm";

export const headers = forwardHeaders;

export function meta({ loaderData }: Route.MetaArgs) {
  return seoMeta(loaderData?.seo);
}

export async function loader({ request }: Route.LoaderArgs) {
  const locale = localeOf(request);
  const [site, contact] = await Promise.all([
    publicGet<PublicSite>(`/api/v1/public/${locale}/site`),
    publicGet<{ available: boolean; retention_days: number }>("/api/v1/public/contact"),
  ]);
  const estado = new URL(request.url).searchParams.get("estado");
  const other = locale === "es" ? "en" : "es";
  return {
    locale,
    site,
    contact,
    // Only known words are echoed back; anything else is ignored.
    state: contactStates.includes(estado as ContactState) ? (estado as ContactState) : null,
    // Receipt idempotency key for this rendering (the no-JS form sends it as a hidden field).
    key: `page-${crypto.randomUUID().replaceAll("-", "")}`,
    seo: seo({
      locale,
      path: sectionPath(locale, "contact"),
      title: t(locale, "contact.title"),
      description: site.intro || t(locale, "site.tagline"),
      alternates: { es: sectionPath("es", "contact"), en: sectionPath("en", "contact") },
      // The ?estado variants are not separate pages.
      noindex: estado !== null,
    }),
    switcher: { href: sectionPath(other, "contact"), available: true },
  };
}

export default function Contact({ loaderData }: Route.ComponentProps) {
  const { locale, site, contact, state, key } = loaderData;
  const has = site.contact_email !== "" || site.links.length > 0;
  return (
    <main>
      <PageIntro title={t(locale, "contact.title")} />
      <Container className="grid gap-10 py-10 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.4fr)]">
        <div>
          <h2 className="font-mono text-xs tracking-widest text-text-muted uppercase">{t(locale, "contact.links")}</h2>
          <div className="mt-4">{has ? <ContactLinks locale={locale} site={site} /> : <Empty>{t(locale, "contact.empty")}</Empty>}</div>
        </div>
        {contact.available ? (
          <ContactForm locale={locale} serverKey={key} initialState={state} retentionDays={contact.retention_days} />
        ) : (
          <p className="rounded-lg border border-dashed border-border p-6 text-text-muted" data-contact="disabled">
            {t(locale, "form.disabled")}
          </p>
        )}
      </Container>
    </main>
  );
}

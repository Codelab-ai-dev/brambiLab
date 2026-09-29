import type { Route } from "./+types/contact";
import { t } from "~/i18n";
import { publicGet } from "~/site/api.server";
import { localeOf, seo } from "~/site/loader.server";
import { sectionPath } from "~/site/paths";
import { forwardHeaders, seoMeta } from "~/site/seo";
import type { PublicSite } from "~/site/types";
import { ContactLinks } from "~/site/ContactLinks";
import { Empty, Frame, PageHeader } from "~/site/ui";
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
      <PageHeader title={t(locale, "contact.title")} lead={site.intro || t(locale, "site.lab")} />
      <div className="bl-light">
        <Frame className="grid gap-12 py-14 sm:py-20 lg:grid-cols-12 lg:gap-8">
          <div className="min-w-0 lg:col-span-4">
            <h2 className="bl-meta text-text-muted">{t(locale, "contact.links")}</h2>
            <div className="mt-6">{has ? <ContactLinks locale={locale} site={site} /> : <Empty>{t(locale, "contact.empty")}</Empty>}</div>
          </div>
          <div className="min-w-0 lg:col-span-7 lg:col-start-6">
            {contact.available ? (
              <ContactForm locale={locale} serverKey={key} initialState={state} retentionDays={contact.retention_days} />
            ) : (
              <p className="border-y border-border py-8 text-lg text-text-muted" data-contact="disabled">
                {t(locale, "form.disabled")}
              </p>
            )}
          </div>
        </Frame>
      </div>
    </main>
  );
}

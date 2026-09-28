import type { Route } from "./+types/contact";
import { t } from "~/i18n";
import { publicGet } from "~/site/api.server";
import { localeOf, seo } from "~/site/loader.server";
import { sectionPath } from "~/site/paths";
import { forwardHeaders, seoMeta } from "~/site/seo";
import type { PublicSite } from "~/site/types";
import { ContactLinks } from "~/site/ContactLinks";
import { Container, Empty, PageIntro } from "~/site/ui";

export const headers = forwardHeaders;

export function meta({ loaderData }: Route.MetaArgs) {
  return seoMeta(loaderData?.seo);
}

export async function loader({ request }: Route.LoaderArgs) {
  const locale = localeOf(request);
  const site = await publicGet<PublicSite>(`/api/v1/public/${locale}/site`);
  const other = locale === "es" ? "en" : "es";
  return {
    locale,
    site,
    seo: seo({
      locale,
      path: sectionPath(locale, "contact"),
      title: t(locale, "contact.title"),
      description: site.intro || t(locale, "site.tagline"),
      alternates: { es: sectionPath("es", "contact"), en: sectionPath("en", "contact") },
    }),
    switcher: { href: sectionPath(other, "contact"), available: true },
  };
}

export default function Contact({ loaderData }: Route.ComponentProps) {
  const { locale, site } = loaderData;
  const has = site.contact_email !== "" || site.links.length > 0;
  return (
    <main>
      <PageIntro title={t(locale, "contact.title")} />
      <Container className="py-10">
        {has ? <ContactLinks locale={locale} site={site} /> : <Empty>{t(locale, "contact.empty")}</Empty>}
        <p className="mt-8 text-sm text-text-muted">{t(locale, "contact.formLater")}</p>
      </Container>
    </main>
  );
}

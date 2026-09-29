import type { Route } from "./+types/about";
import { t } from "~/i18n";
import { publicGet } from "~/site/api.server";
import { localeOf, seo } from "~/site/loader.server";
import { sectionPath } from "~/site/paths";
import { forwardHeaders, seoMeta } from "~/site/seo";
import type { PublicSite } from "~/site/types";
import { ContactLinks } from "~/site/ContactLinks";
import { Empty, Frame, PageHeader } from "~/site/ui";

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
      path: sectionPath(locale, "about"),
      title: t(locale, "about.title"),
      description: site.intro || t(locale, "site.tagline"),
      alternates: { es: sectionPath("es", "about"), en: sectionPath("en", "about") },
    }),
    switcher: { href: sectionPath(other, "about"), available: true },
  };
}

export default function About({ loaderData }: Route.ComponentProps) {
  const { locale, site } = loaderData;
  const paragraphs = site.bio.split(/\n{2,}/).filter((p) => p.trim());
  return (
    <main>
      <PageHeader eyebrow="BrambiLab" title={t(locale, "about.title")} lead={site.intro || undefined} />
      <div className="bl-light">
        <Frame className="grid gap-12 py-14 sm:py-20 lg:grid-cols-12 lg:gap-8">
          <div className="bl-prose min-w-0 lg:col-span-8 lg:col-start-3">
            {paragraphs.length > 0 ? (
              <div className="max-w-[34em] space-y-5 text-lg leading-relaxed">
                {paragraphs.map((p, i) => (
                  <p key={i} className="whitespace-pre-line">{p}</p>
                ))}
              </div>
            ) : (
              <Empty>{t(locale, "about.empty")}</Empty>
            )}
            {(site.links.length > 0 || site.contact_email) && (
              <div className="mt-12">
                <ContactLinks locale={locale} site={site} />
              </div>
            )}
          </div>
        </Frame>
      </div>
    </main>
  );
}

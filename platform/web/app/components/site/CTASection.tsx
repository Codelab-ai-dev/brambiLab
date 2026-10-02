// Closing call to action of the home (WEB-009): big type, a short presentation (the site intro
// when configured) and the localized contact link.
import { t, type Locale } from "~/i18n";
import { ContactLinks } from "~/site/ContactLinks";
import { sectionPath } from "~/site/paths";
import type { PublicSite } from "~/site/types";
import { Arrow, Frame } from "./editorial";

export function CTASection({ locale, site }: { locale: Locale; site: PublicSite }) {
  const hasLinks = site.contact_email !== "" || site.links.length > 0;
  return (
    <section aria-labelledby="cta-title" className="bl-navy border-t border-border">
      <Frame className="grid gap-12 py-20 sm:py-28 lg:grid-cols-12 lg:items-end">
        <h2 id="cta-title" data-reveal className="bl-h-section min-w-0 lg:col-span-8">
          {t(locale, "cta.title.before")} <span className="text-signal">{t(locale, "cta.title.key")}</span>
          {t(locale, "cta.title.after")}
        </h2>
        <div data-reveal className="flex min-w-0 flex-col gap-8 lg:col-span-4">
          <p className="max-w-md text-lg leading-relaxed whitespace-pre-line text-text-muted">{site.intro || t(locale, "site.lab")}</p>
          <a
            href={sectionPath(locale, "contact")}
            className="inline-flex min-h-12 items-center gap-3 self-start rounded-sm bg-primary px-5 font-medium text-primary-contrast hover:bg-white focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-signal"
          >
            {t(locale, "cta.button")}
            <Arrow />
          </a>
          {hasLinks && <ContactLinks locale={locale} site={site} />}
        </div>
      </Frame>
    </section>
  );
}

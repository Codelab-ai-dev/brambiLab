// Public footer (WEB-009): brand, the complete section navigation (also the no-JS target of the
// mobile menu), configured profile links only, and the licences.
import { BrambiLabLogo } from "~/components/brand/BrambiLabLogo";
import { t, type Locale } from "~/i18n";
import { homePath, sectionPath } from "~/site/paths";
import type { SiteLink } from "~/site/types";
import { Arrow, Frame } from "./editorial";
import { isCurrent, navItems } from "./Navigation";

const repository = "https://github.com/Codelab-ai-dev/brambiLab";
const link = "rounded-sm underline-offset-4 hover:text-text hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-signal";

export function Footer({ locale, pathname, links, year }: { locale: Locale; pathname: string; links: SiteLink[]; year: number }) {
  return (
    <footer className="border-t border-border text-sm text-text-muted">
      <Frame className="grid gap-10 py-12 md:grid-cols-12 md:items-start">
        <a href={homePath(locale)} aria-label="BrambiLab" className="self-start justify-self-start rounded-sm focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-signal md:col-span-3">
          <BrambiLabLogo className="h-7" />
        </a>
        <nav id="footer-nav" aria-label={t(locale, "footer.nav")} className="md:col-span-6">
          <ul className="flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:gap-x-8">
            {navItems.map((n) => {
              const href = sectionPath(locale, n.section);
              return (
                <li key={n.section}>
                  <a href={href} aria-current={isCurrent(pathname, href) ? "page" : undefined} className={`inline-flex min-h-11 items-center aria-[current=page]:text-text ${link}`}>
                    {t(locale, n.label)}
                  </a>
                </li>
              );
            })}
          </ul>
        </nav>
        {links.length > 0 && (
          <ul aria-label={t(locale, "footer.social")} className="flex flex-wrap gap-x-6 gap-y-2 md:col-span-3 md:justify-end">
            {links.map((l) => (
              <li key={l.url}>
                <a href={l.url} rel="noopener noreferrer me" className={`inline-flex min-h-11 items-center gap-2 font-mono text-xs tracking-[0.12em] uppercase ${link}`}>
                  {l.label}
                  <Arrow />
                </a>
              </li>
            ))}
          </ul>
        )}
      </Frame>
      <Frame className="flex flex-col gap-3 border-t border-border py-6 text-xs sm:flex-row sm:items-center sm:justify-between">
        <p>
          © {year} Gustavo González ·{" "}
          <a href={`${repository}/blob/main/LICENSE.md`} className={`underline ${link}`}>
            {t(locale, "site.footer.licenses")}
          </a>
        </p>
        <a href={repository} className={`inline-flex items-center gap-2 font-mono ${link}`}>
          {t(locale, "site.footer.source")}
          <Arrow />
        </a>
      </Frame>
    </footer>
  );
}

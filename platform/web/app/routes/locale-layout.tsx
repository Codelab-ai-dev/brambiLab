import type { ReactNode } from "react";
import { isRouteErrorResponse, Outlet, useLocation, useMatches, useRouteError } from "react-router";
import { Wordmark } from "~/components/site/Wordmark";
import { t, type Locale, type MessageKey } from "~/i18n";
import { homePath, localeOfPath, sectionPath, type Section } from "~/site/paths";
import type { Switcher } from "~/site/types";
import { forwardHeaders } from "~/site/seo";

export const headers = forwardHeaders;

const repository = "https://github.com/Codelab-ai-dev/brambiLab";

const nav: { section: Section; label: MessageKey }[] = [
  { section: "projects", label: "nav.projects" },
  { section: "articles", label: "nav.articles" },
  { section: "search", label: "nav.search" },
  { section: "about", label: "nav.about" },
  { section: "contact", label: "nav.contact" },
];

/** The deepest route that says where the other language is (detail pages know their alternate). */
function useSwitcher(locale: Locale, other: Locale): Switcher {
  const matches = useMatches();
  for (let i = matches.length - 1; i >= 0; i--) {
    const d = matches[i].loaderData as { switcher?: Switcher } | undefined;
    if (d?.switcher) return d.switcher;
  }
  return { href: homePath(other), available: true };
}

export default function LocaleLayout() {
  const { pathname } = useLocation();
  const locale = localeOfPath(pathname);
  const other: Locale = locale === "es" ? "en" : "es";
  return (
    <SiteChrome locale={locale} pathname={pathname} switcher={useSwitcher(locale, other)}>
      <Outlet />
    </SiteChrome>
  );
}

/** Errors keep the site header and footer; they are never indexed. */
export function ErrorBoundary() {
  const error = useRouteError();
  const { pathname } = useLocation();
  const locale = localeOfPath(pathname);
  const other: Locale = locale === "es" ? "en" : "es";
  const status = isRouteErrorResponse(error) ? error.status : 500;
  const key: MessageKey = status === 404 ? "error.notFound" : status === 400 ? "error.badRequest" : status === 503 ? "error.unavailable" : "error.generic";
  return (
    <SiteChrome locale={locale} pathname={pathname} switcher={{ href: homePath(other), available: true }}>
      <meta name="robots" content="noindex" />
      <title>{`${status} · BrambiLab`}</title>
      <main className="mx-auto max-w-2xl px-4 py-16" data-error={status}>
        <p className="font-mono text-xs tracking-widest text-accent uppercase">{status}</p>
        <h1 className="mt-2 text-3xl font-semibold">{t(locale, key)}</h1>
        <p className="mt-8">
          <a href={homePath(locale)} className="text-accent underline underline-offset-2">
            {t(locale, "error.home")}
          </a>
        </p>
      </main>
    </SiteChrome>
  );
}

function SiteChrome({ locale, pathname, switcher, children }: { locale: Locale; pathname: string; switcher: Switcher; children: ReactNode }) {
  const other: Locale = locale === "es" ? "en" : "es";
  return (
    <div className="flex min-h-screen flex-col">
      <a href="#main" className="sr-only focus:not-sr-only focus:fixed focus:top-2 focus:left-2 focus:z-50 focus:rounded focus:bg-primary focus:px-3 focus:py-2 focus:text-primary-contrast">
        {t(locale, "nav.skip")}
      </a>
      <header className="bl-bench-bar border-b border-[var(--bench-edge)]/40">
        <div className="mx-auto flex w-full max-w-6xl flex-wrap items-center justify-between gap-x-6 gap-y-3 px-4 py-4 sm:px-6">
          <a href={homePath(locale)} aria-label="BrambiLab" className="rounded text-[var(--bench-text)] focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-[var(--bench-signal)]">
            <Wordmark />
          </a>
          <div className="flex flex-wrap items-center gap-x-5 gap-y-2">
            <nav aria-label={t(locale, "nav.label")}>
              <ul className="flex flex-wrap gap-x-4 gap-y-1 text-sm">
                {nav.map((n) => {
                  const href = sectionPath(locale, n.section);
                  const current = pathname === href || pathname.startsWith(`${href}/`);
                  return (
                    <li key={n.section}>
                      <a
                        href={href}
                        aria-current={current ? "page" : undefined}
                        className="rounded text-[var(--bench-muted)] underline-offset-4 hover:text-[var(--bench-text)] aria-[current=page]:text-[var(--bench-text)] aria-[current=page]:underline aria-[current=page]:decoration-[var(--bench-signal)] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--bench-signal)]"
                      >
                        {t(locale, n.label)}
                      </a>
                    </li>
                  );
                })}
              </ul>
            </nav>
            <a
              href={switcher.href}
              hrefLang={other}
              lang={other}
              aria-label={switcher.available ? t(locale, "switch.available") : t(locale, "switch.missing")}
              data-switch={switcher.available ? "same" : "index"}
              className="rounded border border-[var(--bench-edge)] px-2 py-1 font-mono text-xs tracking-widest text-[var(--bench-muted)] uppercase hover:border-[var(--bench-signal)] hover:text-[var(--bench-text)] focus-visible:outline-2 focus-visible:outline-[var(--bench-signal)]"
            >
              {other}
            </a>
          </div>
        </div>
      </header>
      <div id="main" className="flex-1" tabIndex={-1}>
        {children}
      </div>
      <footer className="border-t border-border">
        <div className="mx-auto flex w-full max-w-6xl flex-col gap-2 px-4 py-6 text-xs text-text-muted sm:flex-row sm:items-center sm:justify-between sm:px-6">
          <p>
            © 2026 Gustavo González · <a href={`${repository}/blob/main/LICENSE.md`} className="underline underline-offset-2 hover:text-text">{t(locale, "site.footer.licenses")}</a>
          </p>
          <a href={repository} className="font-mono underline underline-offset-2 hover:text-text">
            {t(locale, "site.footer.source")} ↗
          </a>
        </div>
      </footer>
    </div>
  );
}

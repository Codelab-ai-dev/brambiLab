import type { ReactNode } from "react";
import { isRouteErrorResponse, Outlet, useLocation, useMatches, useRouteError, useRouteLoaderData } from "react-router";
import type { Route } from "./+types/locale-layout";
import { Footer } from "~/components/site/Footer";
import { Navigation } from "~/components/site/Navigation";
import { t, type Locale, type MessageKey } from "~/i18n";
import { publicGet } from "~/site/api.server";
import { localeOf } from "~/site/loader.server";
import { homePath, localeOfPath } from "~/site/paths";
import type { PublicSite, SiteLink, Switcher } from "~/site/types";
import { forwardHeaders } from "~/site/seo";

export const headers = forwardHeaders;


/**
 * Footer profile links on every public page. Decoration of the chrome: when the API cannot answer,
 * the page itself decides (its own loader fails with 503); the footer just shows no links.
 */
export async function loader({ request }: Route.LoaderArgs) {
  const year = new Date().getFullYear();
  try {
    const site = await publicGet<PublicSite>(`/api/v1/public/${localeOf(request)}/site`);
    return { links: site.links, year };
  } catch {
    return { links: [] as SiteLink[], year };
  }
}

/** The deepest route that says where the other language is (detail pages know their alternate). */
function useSwitcher(locale: Locale, other: Locale): Switcher {
  const matches = useMatches();
  for (let i = matches.length - 1; i >= 0; i--) {
    const d = matches[i].loaderData as { switcher?: Switcher } | undefined;
    if (d?.switcher) return d.switcher;
  }
  return { href: homePath(other), available: true };
}

export default function LocaleLayout({ loaderData }: Route.ComponentProps) {
  const { pathname } = useLocation();
  const locale = localeOfPath(pathname);
  const other: Locale = locale === "es" ? "en" : "es";
  return (
    <SiteChrome locale={locale} pathname={pathname} switcher={useSwitcher(locale, other)} links={loaderData.links} year={loaderData.year}>
      <Outlet />
    </SiteChrome>
  );
}

/** Errors keep the site header and footer; they are never indexed. */
export function ErrorBoundary() {
  const error = useRouteError();
  const { pathname } = useLocation();
  const data = useRouteLoaderData<typeof loader>(`${localeOfPath(pathname)}-layout`);
  const locale = localeOfPath(pathname);
  const other: Locale = locale === "es" ? "en" : "es";
  const status = isRouteErrorResponse(error) ? error.status : 500;
  const key: MessageKey = status === 404 ? "error.notFound" : status === 400 ? "error.badRequest" : status === 503 ? "error.unavailable" : "error.generic";
  return (
    <SiteChrome locale={locale} pathname={pathname} switcher={{ href: homePath(other), available: true }} links={data?.links} year={data?.year ?? new Date().getFullYear()}>
      <meta name="robots" content="noindex" />
      <title>{`${status} · BrambiLab`}</title>
      <main className="mx-auto w-full max-w-[100rem] px-4 py-20 sm:px-6 sm:py-28 lg:px-10" data-error={status}>
        <p className="bl-meta text-accent">{status}</p>
        <h1 className="bl-h-project mt-6 max-w-[18ch]">{t(locale, key)}</h1>
        <p className="mt-10">
          <a href={homePath(locale)} className="inline-flex min-h-12 items-center rounded-sm bg-primary px-5 font-medium text-primary-contrast focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-signal">
            {t(locale, "error.home")}
          </a>
        </p>
      </main>
    </SiteChrome>
  );
}

function SiteChrome({ locale, pathname, switcher, links = [], year, children }: { locale: Locale; pathname: string; switcher: Switcher; links?: SiteLink[]; year: number; children: ReactNode }) {
  return (
    <div className="bl-site flex min-h-screen flex-col">
      <a href="#main" className="sr-only focus:not-sr-only focus:fixed focus:top-2 focus:left-2 focus:z-50 focus:rounded-sm focus:bg-primary focus:px-3 focus:py-2 focus:text-primary-contrast focus:outline-2 focus:outline-offset-2 focus:outline-white">
        {t(locale, "nav.skip")}
      </a>
      <Navigation locale={locale} pathname={pathname} switcher={switcher} />
      <div id="main" className="flex-1" tabIndex={-1}>
        {children}
      </div>
      <Footer locale={locale} pathname={pathname} links={links} year={year} />
    </div>
  );
}

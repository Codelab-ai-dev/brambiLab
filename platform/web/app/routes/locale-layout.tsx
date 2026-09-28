import { data, Outlet } from "react-router";
import type { Route } from "./+types/locale-layout";
import { Wordmark } from "~/components/site/Wordmark";
import { isLocale, t, type Locale } from "~/i18n";

export function loader({ params }: Route.LoaderArgs) {
  if (!isLocale(params.lang)) {
    throw data(null, { status: 404 });
  }
  return { locale: params.lang };
}

const repository = "https://github.com/Codelab-ai-dev/brambiLab";

export default function LocaleLayout({ loaderData }: Route.ComponentProps) {
  const locale: Locale = loaderData.locale;
  const other: Locale = locale === "es" ? "en" : "es";
  return (
    <div className="flex min-h-screen flex-col">
      <header className="border-b border-border">
        <div className="mx-auto flex w-full max-w-5xl items-center justify-between gap-4 px-4 py-4 sm:px-6">
          <a href={`/${locale}`} aria-label="BrambiLab" className="rounded focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-accent">
            <Wordmark />
          </a>
          <a
            href={`/${other}`}
            hrefLang={other}
            lang={other}
            aria-label={t(locale, "site.switchLocaleLabel")}
            className="rounded px-2 py-1 font-mono text-xs uppercase tracking-widest text-text-muted hover:text-text focus-visible:outline-2 focus-visible:outline-accent"
          >
            {other}
          </a>
        </div>
      </header>
      <div className="flex-1">
        <Outlet />
      </div>
      <footer className="border-t border-border">
        <div className="mx-auto flex w-full max-w-5xl flex-col gap-2 px-4 py-6 text-xs text-text-muted sm:flex-row sm:items-center sm:justify-between sm:px-6">
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

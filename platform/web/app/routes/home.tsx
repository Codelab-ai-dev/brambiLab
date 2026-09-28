import type { Route } from "./+types/home";
import { defaultLocale, isLocale, t } from "~/i18n";

export function meta({ loaderData }: Route.MetaArgs) {
  return [
    { title: "BrambiLab" },
    { name: "description", content: t(loaderData.locale, "site.tagline") },
    // Placeholder page: keep it out of search indexes until WEB-006 defines SEO.
    { name: "robots", content: "noindex" },
  ];
}

export function loader({ params }: Route.LoaderArgs) {
  // The parent layout already rejected unsupported locales.
  return { locale: isLocale(params.lang) ? params.lang : defaultLocale };
}

export default function Home({ loaderData }: Route.ComponentProps) {
  const { locale } = loaderData;
  const other = locale === "es" ? "en" : "es";
  return (
    <main className="mx-auto max-w-2xl px-4 py-16">
      <h1 className="text-3xl font-semibold">BrambiLab</h1>
      <p className="mt-4 text-lg">{t(locale, "site.tagline")}</p>
      <p className="mt-8 text-gray-600 dark:text-gray-400">
        {t(locale, "site.underConstruction")}
      </p>
      <a
        className="mt-8 inline-block underline"
        href={`/${other}`}
        hrefLang={other}
      >
        {t(locale, "site.switchLocale")}
      </a>
    </main>
  );
}

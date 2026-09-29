import {
  isRouteErrorResponse,
  Links,
  Meta,
  Outlet,
  Scripts,
  ScrollRestoration,
  useLocation,
} from "react-router";

import type { Route } from "./+types/root";

// Brand icons (#45): SVG favicon with an ICO fallback and the Apple touch icon. No web manifest:
// the site is not a PWA.
export const links: Route.LinksFunction = () => [
  { rel: "icon", href: "/favicon.svg", type: "image/svg+xml" },
  { rel: "icon", href: "/favicon.ico", sizes: "32x32" },
  { rel: "apple-touch-icon", href: "/apple-touch-icon.png" },
];
import { t } from "./i18n";
import { homePath, localeOfPath } from "./site/paths";
import "./app.css";
import { forwardHeaders } from "./site/seo";

// Unmatched paths and errors outside a route with its own headers are never cached.
export const headers = forwardHeaders;

function useLocale() {
  return localeOfPath(useLocation().pathname);
}

export function Layout({ children }: { children: React.ReactNode }) {
  const locale = useLocale();
  return (
    <html lang={locale}>
      <head>
        <meta charSet="utf-8" />
        <meta name="viewport" content="width=device-width, initial-scale=1" />
        <Meta />
        <Links />
      </head>
      <body>
        {children}
        <ScrollRestoration />
        <Scripts />
      </body>
    </html>
  );
}

export default function App() {
  return <Outlet />;
}

export function ErrorBoundary({ error }: Route.ErrorBoundaryProps) {
  const locale = useLocale();
  let message = "Error";
  let details = t(locale, "error.generic");
  let stack: string | undefined;

  // 404, bad parameters and an unavailable API are different answers; none pretends to be empty.
  if (isRouteErrorResponse(error)) {
    message = String(error.status);
    if (error.status === 404) details = t(locale, "error.notFound");
    else if (error.status === 400) details = t(locale, "error.badRequest");
    else if (error.status === 503) details = t(locale, "error.unavailable");
  } else if (import.meta.env.DEV && error instanceof Error) {
    details = error.message;
    stack = error.stack;
  }

  return (
    <main className="mx-auto max-w-2xl px-4 py-16">
      <p className="font-mono text-xs tracking-widest text-accent uppercase">BrambiLab</p>
      <h1 className="mt-2 text-3xl font-semibold">{message}</h1>
      <p className="mt-4">{details}</p>
      <p className="mt-8">
        <a href={homePath(locale)} className="text-accent underline underline-offset-2">
          {t(locale, "error.home")}
        </a>
      </p>
      {stack && (
        <pre className="mt-4 w-full overflow-x-auto p-4">
          <code>{stack}</code>
        </pre>
      )}
    </main>
  );
}

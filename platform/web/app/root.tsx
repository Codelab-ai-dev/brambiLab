import {
  isRouteErrorResponse,
  Links,
  Meta,
  Outlet,
  Scripts,
  ScrollRestoration,
  useParams,
} from "react-router";

import type { Route } from "./+types/root";
import { defaultLocale, isLocale, t } from "./i18n";
import "./app.css";

function useLocale() {
  const { lang } = useParams();
  return isLocale(lang) ? lang : defaultLocale;
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

  if (isRouteErrorResponse(error) && error.status === 404) {
    message = "404";
    details = t(locale, "error.notFound");
  } else if (import.meta.env.DEV && error instanceof Error) {
    details = error.message;
    stack = error.stack;
  }

  return (
    <main className="mx-auto max-w-2xl px-4 py-16">
      <h1 className="text-3xl font-semibold">{message}</h1>
      <p className="mt-4">{details}</p>
      {stack && (
        <pre className="mt-4 w-full overflow-x-auto p-4">
          <code>{stack}</code>
        </pre>
      )}
    </main>
  );
}

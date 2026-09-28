import { Link, Outlet, redirect, useLocation } from "react-router";
import type { Route } from "./+types/layout";
import { getOwner } from "~/lib/api.server";

// Every /admin page (except login) requires the owner session, checked by Go on each request.
// Child loaders also call Go themselves; this loader is not their only protection.
export async function loader({ request }: Route.LoaderArgs) {
  const result = await getOwner(request);
  if (result.status !== "authenticated") {
    const url = new URL(request.url);
    throw redirect(`/admin/login?return_to=${encodeURIComponent(url.pathname + url.search)}`);
  }
  return { owner: result.owner };
}

// NavLink ignores the query string, so list tabs compare ?tipo= themselves.
const nav: { to: string; label: string; active: (path: string, tipo: string | null) => boolean }[] = [
  { to: "/admin", label: "Inicio", active: (p) => p === "/admin" },
  { to: "/admin/contenidos?tipo=project", label: "Proyectos", active: (p, t) => p === "/admin/contenidos" && (t ?? "project") === "project" },
  { to: "/admin/contenidos?tipo=article", label: "Artículos", active: (p, t) => p === "/admin/contenidos" && t === "article" },
  { to: "/admin/contenidos?tipo=log", label: "Bitácora", active: (p, t) => p === "/admin/contenidos" && t === "log" },
  { to: "/admin/taxonomia", label: "Categorías y etiquetas", active: (p) => p === "/admin/taxonomia" },
];

export default function AdminLayout({ loaderData }: Route.ComponentProps) {
  const { owner } = loaderData;
  const location = useLocation();
  const tipo = new URLSearchParams(location.search).get("tipo");
  return (
    <div className="min-h-screen">
      <a href="#contenido" className="sr-only focus:not-sr-only focus:absolute focus:m-2 focus:rounded focus:bg-surface focus:p-2">
        Saltar al contenido
      </a>
      <header className="border-b border-border">
        <div className="mx-auto flex max-w-6xl flex-wrap items-center justify-between gap-3 px-4 py-3">
          <a href="/admin" className="font-semibold">
            BrambiLab · Panel
          </a>
          <div className="flex items-center gap-4 text-sm">
            <span className="text-text-muted">@{owner.github_login}</span>
            {/* Plain form post to the API: works without JavaScript; Go checks Origin and CSRF. */}
            <form method="post" action="/api/v1/auth/logout">
              <input type="hidden" name="csrf_token" value={owner.csrf_token} />
              <button type="submit" className="underline underline-offset-2">
                Cerrar sesión
              </button>
            </form>
          </div>
        </div>
        <nav aria-label="Panel" className="mx-auto max-w-6xl overflow-x-auto px-4">
          <ul className="flex gap-1">
            {nav.map((item) => {
              const isActive = item.active(location.pathname.replace(/\/$/, "") || "/", tipo);
              return (
                <li key={item.to}>
                  <Link
                    to={item.to}
                    aria-current={isActive ? "page" : undefined}
                    className={`block whitespace-nowrap border-b-2 px-3 py-2 text-sm ${isActive ? "border-accent font-medium" : "border-transparent text-text-muted hover:text-text"}`}
                  >
                    {item.label}
                  </Link>
                </li>
              );
            })}
          </ul>
        </nav>
      </header>
      <main id="contenido" className="mx-auto max-w-6xl px-4 pb-16">
        <Outlet />
      </main>
    </div>
  );
}

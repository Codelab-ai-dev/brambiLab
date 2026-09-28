import { Outlet, redirect } from "react-router";
import type { Route } from "./+types/layout";
import { getOwner } from "~/lib/api.server";

// Every /admin page (except login) requires the owner session, checked by Go on each request.
export async function loader({ request }: Route.LoaderArgs) {
  const result = await getOwner(request);
  if (result.status !== "authenticated") {
    const url = new URL(request.url);
    const returnTo = url.pathname + url.search;
    throw redirect(`/admin/login?return_to=${encodeURIComponent(returnTo)}`);
  }
  return { owner: result.owner };
}

export default function AdminLayout({ loaderData }: Route.ComponentProps) {
  const { owner } = loaderData;
  return (
    <div className="mx-auto max-w-4xl px-4 py-8">
      <header className="flex items-center justify-between border-b border-gray-200 pb-4 dark:border-gray-800">
        <a href="/admin" className="font-semibold">
          BrambiLab · Panel
        </a>
        <div className="flex items-center gap-4 text-sm">
          <span>@{owner.github_login}</span>
          {/* Plain form post to the API: works without JavaScript; Go checks Origin and CSRF. */}
          <form method="post" action="/api/v1/auth/logout">
            <input type="hidden" name="csrf_token" value={owner.csrf_token} />
            <button type="submit" className="underline">
              Cerrar sesión
            </button>
          </form>
        </div>
      </header>
      <Outlet />
    </div>
  );
}

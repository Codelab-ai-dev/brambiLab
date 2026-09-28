import { redirect } from "react-router";
import type { Route } from "./+types/login";
import { getOwner, privateHeaders } from "~/lib/api.server";

export function headers() {
  return privateHeaders;
}

export function meta() {
  return [{ title: "Acceso · BrambiLab" }, { name: "robots", content: "noindex, nofollow" }];
}

const messages: Record<string, string> = {
  forbidden: "Esa cuenta de GitHub no tiene acceso a este panel.",
  denied: "Se canceló la autorización en GitHub.",
  failed: "No se pudo completar el inicio de sesión. Inténtalo de nuevo.",
};

export async function loader({ request }: Route.LoaderArgs) {
  const url = new URL(request.url);
  // Go validates return_to again; this only keeps the link tidy.
  const rawReturnTo = url.searchParams.get("return_to") ?? "/admin";
  const returnTo = rawReturnTo.startsWith("/admin") ? rawReturnTo : "/admin";

  const result = await getOwner(request);
  if (result.status === "authenticated") throw redirect(returnTo);

  const error = url.searchParams.get("error");
  return {
    configured: result.status !== "not_configured",
    startUrl: `/api/v1/auth/github/start?return_to=${encodeURIComponent(returnTo)}`,
    message: error ? (messages[error] ?? messages.failed) : null,
    loggedOut: url.searchParams.has("logged_out"),
  };
}

export default function Login({ loaderData }: Route.ComponentProps) {
  const { configured, startUrl, message, loggedOut } = loaderData;
  return (
    <main className="mx-auto max-w-md px-4 py-16">
      <h1 className="text-2xl font-semibold">Panel de BrambiLab</h1>
      {loggedOut && <p className="mt-4">Sesión cerrada.</p>}
      {message && (
        <p role="alert" className="mt-4 text-red-700 dark:text-red-400">
          {message}
        </p>
      )}
      {configured ? (
        // A full document navigation: the OAuth flow leaves the site and must not be client-routed.
        <a
          href={startUrl}
          className="mt-8 inline-block rounded bg-gray-900 px-4 py-2 text-white dark:bg-gray-100 dark:text-gray-900"
        >
          Iniciar sesión con GitHub
        </a>
      ) : (
        <p className="mt-8 text-gray-600 dark:text-gray-400">
          El acceso con GitHub no está configurado en este entorno.
        </p>
      )}
    </main>
  );
}

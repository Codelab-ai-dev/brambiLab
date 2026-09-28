import { redirect } from "react-router";
import type { Route } from "./+types/login";
import { privateHeaders } from "~/lib/api.server";
import { loginState } from "~/lib/login.server";

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
  const state = await loginState(request);
  if (state.kind === "redirect") throw redirect(state.to);

  const url = new URL(request.url);
  const error = url.searchParams.get("error");
  return {
    availability: state.availability,
    startUrl: `/api/v1/auth/github/start?return_to=${encodeURIComponent(state.returnTo)}`,
    message: error ? (messages[error] ?? messages.failed) : null,
    loggedOut: url.searchParams.has("logged_out"),
  };
}

export default function Login({ loaderData }: Route.ComponentProps) {
  const { availability, startUrl, message, loggedOut } = loaderData;
  return (
    <main className="mx-auto max-w-md px-4 py-16">
      <h1 className="text-2xl font-semibold">Panel de BrambiLab</h1>
      {loggedOut && <p className="mt-4">Sesión cerrada.</p>}
      {message && (
        <p role="alert" className="mt-4 text-red-700 dark:text-red-400">
          {message}
        </p>
      )}
      {availability === "enabled" && (
        // A full document navigation: the OAuth flow leaves the site and must not be client-routed.
        <a
          href={startUrl}
          className="mt-8 inline-block rounded bg-gray-900 px-4 py-2 text-white dark:bg-gray-100 dark:text-gray-900"
        >
          Iniciar sesión con GitHub
        </a>
      )}
      {availability === "not_configured" && (
        <p className="mt-8 text-gray-600 dark:text-gray-400">
          El acceso con GitHub no está configurado en este entorno.
        </p>
      )}
      {availability === "unavailable" && (
        <p role="alert" className="mt-8 text-gray-600 dark:text-gray-400">
          El servicio de acceso no responde. Inténtalo de nuevo en unos minutos.
        </p>
      )}
    </main>
  );
}

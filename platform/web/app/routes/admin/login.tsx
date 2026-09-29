import { redirect } from "react-router";
import type { Route } from "./+types/login";
import { privateHeaders } from "~/lib/api.server";
import { loginState } from "~/lib/login.server";
import { BrambiLabLogo } from "~/components/brand/BrambiLabLogo";

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
    <main className="bl-grid flex min-h-screen items-start justify-center px-4 py-16 sm:items-center">
      <div className="w-full max-w-sm rounded-md border border-border bg-surface p-8 shadow-sm">
        <p>
          <BrambiLabLogo className="h-8" />
          <span className="sr-only">BrambiLab</span>
        </p>
        <h1 className="mt-6 text-2xl font-semibold">Panel privado</h1>
        <p className="mt-2 text-sm text-text-muted">Acceso exclusivo del propietario mediante GitHub.</p>
        {loggedOut && (
          <p role="status" className="mt-4 text-sm">
            Sesión cerrada.
          </p>
        )}
        {message && (
          <p role="alert" className="mt-4 text-sm text-danger">
            {message}
          </p>
        )}
        {availability === "enabled" && (
          // A full document navigation: the OAuth flow leaves the site and must not be client-routed.
          <a
            href={startUrl}
            className="mt-6 inline-flex min-h-10 w-full items-center justify-center rounded-md bg-primary px-4 font-medium text-primary-contrast focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
          >
            Iniciar sesión con GitHub
          </a>
        )}
        {availability === "not_configured" && <p className="mt-6 text-sm text-text-muted">El acceso con GitHub no está configurado en este entorno.</p>}
        {availability === "unavailable" && (
          <p role="alert" className="mt-6 text-sm text-text-muted">
            El servicio de acceso no responde. Inténtalo de nuevo en unos minutos.
          </p>
        )}
      </div>
    </main>
  );
}

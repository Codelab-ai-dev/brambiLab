import type { Route } from "./+types/home";
import { privateHeaders } from "~/lib/api.server";

export function headers() {
  return privateHeaders;
}

export function meta() {
  return [{ title: "Panel · BrambiLab" }, { name: "robots", content: "noindex, nofollow" }];
}

export default function AdminHome({ matches }: Route.ComponentProps) {
  const owner = matches[1].loaderData.owner;
  return (
    <main className="py-8">
      <h1 className="text-2xl font-semibold">Panel</h1>
      <p className="mt-4">
        Sesión iniciada como <strong>@{owner.github_login}</strong>. Expira el{" "}
        {new Date(owner.expires_at).toLocaleString("es-MX", { timeZone: "America/Mexico_City" })}.
      </p>
      <p className="mt-4 text-gray-600 dark:text-gray-400">
        La gestión de contenido llegará con WEB-003.
      </p>
    </main>
  );
}

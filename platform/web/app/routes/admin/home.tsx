import { useRouteLoaderData } from "react-router";
import { LinkButton, PageHeader } from "~/components/admin/ui";
import { formatDate } from "~/content/api-types";
import { privateHeaders } from "~/lib/api.server";
import type { loader as layoutLoader } from "./layout";

export function headers() {
  return privateHeaders;
}

export function meta() {
  return [{ title: "Panel · BrambiLab" }, { name: "robots", content: "noindex, nofollow" }];
}

export default function AdminHome() {
  const { owner } = useRouteLoaderData<typeof layoutLoader>("routes/admin/layout")!;
  return (
    <>
      <PageHeader title="Panel" description={<>Sesión de @{owner.github_login}, válida hasta el {formatDate(owner.expires_at)}</>} />
      <div className="grid gap-4 sm:grid-cols-3">
        {[
          { tipo: "project", title: "Proyectos", text: "Ficha técnica, objetivo, estado y resultados." },
          { tipo: "article", title: "Artículos", text: "Textos independientes de cualquier proyecto." },
          { tipo: "log", title: "Bitácora", text: "Entradas de avance vinculadas a un proyecto." },
        ].map((c) => (
          <section key={c.tipo} className="flex flex-col gap-3 rounded-md border border-border p-4">
            <h2 className="font-semibold">{c.title}</h2>
            <p className="flex-1 text-sm text-text-muted">{c.text}</p>
            <LinkButton to={`/admin/contenidos?tipo=${c.tipo}`}>Ver {c.title.toLowerCase()}</LinkButton>
          </section>
        ))}
      </div>
      <p className="mt-6 text-sm text-text-muted">Cada idioma se publica o se programa por separado desde la ficha, el editor o el historial del contenido; lo no publicado sigue siendo privado. Los mensajes del formulario de contacto están en «Contacto».</p>
    </>
  );
}

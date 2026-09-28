import { Link } from "react-router";
import type { Route } from "./+types/contents";
import { EmptyState, LinkButton, PageHeader } from "~/components/admin/ui";
import { formatDate, kindLabels, type Content, type ContentKind, type Page } from "~/content/api-types";
import { adminGet } from "~/lib/admin-api.server";
import { privateHeaders } from "~/lib/api.server";

export function headers() {
  return privateHeaders;
}

export function meta({ loaderData }: Route.MetaArgs) {
  return [{ title: `${kindLabels[loaderData.kind].many} · Panel` }, { name: "robots", content: "noindex, nofollow" }];
}

const kinds: ContentKind[] = ["project", "article", "log"];

export async function loader({ request }: Route.LoaderArgs) {
  const url = new URL(request.url);
  const kind = (kinds as string[]).includes(url.searchParams.get("tipo") ?? "") ? (url.searchParams.get("tipo") as ContentKind) : "project";
  const archived = url.searchParams.get("archivados") === "1";
  const page = Math.max(1, Number.parseInt(url.searchParams.get("pagina") ?? "1", 10) || 1);
  const list = await adminGet<Page<Content>>(request, `/api/v1/admin/contents?kind=${kind}&archived=${archived}&page=${page}&page_size=20`);
  return { kind, archived, list };
}

function titleOf(c: Content): string {
  return c.translations.find((t) => t.title)?.title ?? "Sin título todavía";
}

export default function Contents({ loaderData }: Route.ComponentProps) {
  const { kind, archived, list } = loaderData;
  const labels = kindLabels[kind];
  const pages = Math.max(1, Math.ceil(list.total / list.page_size));
  const query = (p: number) => `/admin/contenidos?tipo=${kind}${archived ? "&archivados=1" : ""}&pagina=${p}`;
  return (
    <>
      <PageHeader
        title={labels.many}
        description={archived ? "Contenidos archivados: sólo lectura, con su historial completo." : `${list.total} en total.`}
        actions={
          kind === "log" ? (
            <p className="max-w-xs text-sm text-text-muted">Las entradas se crean desde la ficha de su proyecto.</p>
          ) : (
            <LinkButton tone="primary" to={`/admin/contenidos/nuevo?tipo=${kind}`}>
              {labels.new}
            </LinkButton>
          )
        }
      />
      <div className="mb-4 text-sm">
        <Link to={archived ? `/admin/contenidos?tipo=${kind}` : `/admin/contenidos?tipo=${kind}&archivados=1`} className="underline underline-offset-2">
          {archived ? "Ver activos" : "Ver archivados"}
        </Link>
      </div>

      {list.items.length === 0 ? (
        <EmptyState title={archived ? "No hay contenidos archivados." : `Todavía no hay ${labels.many.toLowerCase()}.`}>
          {!archived && kind !== "log" && <LinkButton to={`/admin/contenidos/nuevo?tipo=${kind}`}>{labels.new}</LinkButton>}
        </EmptyState>
      ) : (
        <div className="overflow-x-auto rounded-md border border-border">
          <table className="w-full text-sm">
            <thead className="bg-surface-muted text-left">
              <tr>
                <th scope="col" className="px-4 py-2 font-medium">Título</th>
                <th scope="col" className="px-4 py-2 font-medium">Idiomas</th>
                <th scope="col" className="px-4 py-2 font-medium">Última revisión</th>
              </tr>
            </thead>
            <tbody>
              {list.items.map((c) => (
                <tr key={c.id} className="border-t border-border">
                  <td className="px-4 py-3">
                    <Link to={`/admin/contenidos/${c.id}`} className="font-medium text-accent underline-offset-2 hover:underline">
                      {titleOf(c)}
                    </Link>
                  </td>
                  <td className="px-4 py-3">
                    <ul className="flex gap-2">
                      {c.translations.map((t) => (
                        <li key={t.locale} className="rounded bg-surface-muted px-2 py-0.5 font-mono text-xs uppercase">
                          {t.locale} · {t.latest_version ? `v${t.latest_version}` : "vacío"}
                        </li>
                      ))}
                    </ul>
                  </td>
                  <td className="px-4 py-3 text-text-muted">{formatDate(c.translations.map((t) => t.updated_at).filter(Boolean).sort().pop() ?? c.created_at)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {pages > 1 && (
        <nav aria-label="Paginación" className="mt-4 flex items-center justify-between text-sm">
          {list.page > 1 ? <Link to={query(list.page - 1)}>← Anterior</Link> : <span />}
          <span className="text-text-muted">
            Página {list.page} de {pages}
          </span>
          {list.page < pages ? <Link to={query(list.page + 1)}>Siguiente →</Link> : <span />}
        </nav>
      )}
    </>
  );
}

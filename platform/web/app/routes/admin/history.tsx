import { Form, Link, useNavigation, useRouteLoaderData } from "react-router";
import type { Route } from "./+types/history";
import { Button, Notice, PageHeader } from "~/components/admin/ui";
import { formatDate, localeLabels, revisionKindLabels, type Page, type RevisionSummary, type Translation } from "~/content/api-types";
import { adminGet } from "~/lib/admin-api.server";
import { apiSend, describe, revisionsPath } from "~/lib/admin-client";
import { privateHeaders } from "~/lib/api.server";
import type { loader as layoutLoader } from "./layout";

export function headers() {
  return privateHeaders;
}

export function meta() {
  return [{ title: "Historial · Panel" }, { name: "robots", content: "noindex, nofollow" }];
}

export async function loader({ request, params }: Route.LoaderArgs) {
  if (params.locale !== "es" && params.locale !== "en") throw new Response(null, { status: 404 });
  const page = Math.max(1, Number.parseInt(new URL(request.url).searchParams.get("pagina") ?? "1", 10) || 1);
  const [translation, revisions] = await Promise.all([
    adminGet<Translation>(request, `/api/v1/admin/contents/${params.id}/translations/${params.locale}`),
    adminGet<Page<RevisionSummary>>(request, `${revisionsPath(params.id, params.locale)}?page=${page}&page_size=20`),
  ]);
  return { translation, revisions, locale: params.locale };
}

export async function clientAction({ request, params }: Route.ClientActionArgs) {
  const form = await request.formData();
  const version = Number(form.get("version"));
  const result = await apiSend<{ created: boolean; revision: { version: number } }>(
    "POST",
    `${revisionsPath(params.id, params.locale)}/${version}/restore`,
    String(form.get("csrf_token")),
    { expected_version: Number(form.get("expected_version")) },
  );
  if (!result.ok) {
    return {
      ok: false as const,
      message: result.error.code === "version_conflict" ? "Se guardó otra versión mientras tanto. Recarga el historial e inténtalo de nuevo." : describe(result.error),
    };
  }
  return {
    ok: true as const,
    message: result.data.created ? `La versión ${version} se restauró como nueva versión ${result.data.revision.version}.` : "Esa versión ya es la actual; no se creó otra.",
  };
}

export default function History({ loaderData, actionData, params }: Route.ComponentProps) {
  const { translation, revisions, locale } = loaderData;
  const { owner } = useRouteLoaderData<typeof layoutLoader>("routes/admin/layout")!;
  const busy = useNavigation().state !== "idle";
  const pages = Math.max(1, Math.ceil(revisions.total / revisions.page_size));
  return (
    <>
      <PageHeader
        title={`Historial · ${localeLabels[locale as "es" | "en"]}`}
        description={
          <>
            {revisions.total} revisiones. Restaurar crea una versión nueva; ninguna revisión se modifica ni se borra.{" "}
            <Link to={`/admin/contenidos/${params.id}/${locale}`} className="underline">
              Volver al editor
            </Link>
          </>
        }
      />
      {actionData && <Notice tone={actionData.ok ? "success" : "danger"}>{actionData.message}</Notice>}
      <div className="mt-4 overflow-x-auto rounded-md border border-border">
        <table className="w-full text-sm">
          <caption className="sr-only">Revisiones, de la más reciente a la más antigua</caption>
          <thead className="bg-surface-muted text-left">
            <tr>
              <th scope="col" className="px-4 py-2 font-medium">Versión</th>
              <th scope="col" className="px-4 py-2 font-medium">Tipo</th>
              <th scope="col" className="px-4 py-2 font-medium">Fecha</th>
              <th scope="col" className="px-4 py-2 font-medium">Título</th>
              <th scope="col" className="px-4 py-2 font-medium"><span className="sr-only">Acciones</span></th>
            </tr>
          </thead>
          <tbody>
            {revisions.items.map((r) => (
              <tr key={r.version} className="border-t border-border">
                <td className="px-4 py-3 font-mono">
                  v{r.version}
                  {r.version === translation.latest_version && <span className="ml-2 rounded bg-surface-muted px-1.5 py-0.5 font-sans text-xs">actual</span>}
                </td>
                <td className="px-4 py-3">
                  {revisionKindLabels[r.kind]}
                  {r.restored_from_version && ` de v${r.restored_from_version}`}
                </td>
                <td className="px-4 py-3 whitespace-nowrap text-text-muted">{formatDate(r.created_at)}</td>
                <td className="px-4 py-3">{r.title}</td>
                <td className="px-4 py-3">
                  <div className="flex flex-wrap items-center justify-end gap-3">
                    <Link to={`/admin/contenidos/${params.id}/${locale}/v/${r.version}`} className="underline underline-offset-2">
                      Vista previa<span className="sr-only"> de la versión {r.version}</span>
                    </Link>
                    {r.version !== translation.latest_version && (
                      <Form method="post">
                        <input type="hidden" name="csrf_token" value={owner.csrf_token} />
                        <input type="hidden" name="version" value={r.version} />
                        <input type="hidden" name="expected_version" value={translation.latest_version} />
                        <Button type="submit" className="min-h-8 px-2" disabled={busy}>
                          Restaurar<span className="sr-only"> la versión {r.version}</span>
                        </Button>
                      </Form>
                    )}
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {pages > 1 && (
        <nav aria-label="Paginación" className="mt-4 flex items-center justify-between text-sm">
          {revisions.page > 1 ? <Link to={`?pagina=${revisions.page - 1}`}>← Más recientes</Link> : <span />}
          <span className="text-text-muted">Página {revisions.page} de {pages}</span>
          {revisions.page < pages ? <Link to={`?pagina=${revisions.page + 1}`}>Más antiguas →</Link> : <span />}
        </nav>
      )}
    </>
  );
}

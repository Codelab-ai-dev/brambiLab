import { Link, useRevalidator, useRouteLoaderData } from "react-router";
import type { Route } from "./+types/media";
import { AssetThumb } from "~/components/admin/AssetPicker";
import { EmptyState, PageHeader } from "~/components/admin/ui";
import { Uploader } from "~/components/admin/Uploader";
import { assetKindLabels, formatBytes, formatDate, type Asset, type AssetKind, type Page } from "~/content/api-types";
import { adminGet } from "~/lib/admin-api.server";
import { privateHeaders } from "~/lib/api.server";
import type { loader as layoutLoader } from "./layout";

export function headers() {
  return privateHeaders;
}

export function meta() {
  return [{ title: "Medios · Panel" }, { name: "robots", content: "noindex, nofollow" }];
}

const kinds: AssetKind[] = ["image", "video", "resource"];

export async function loader({ request }: Route.LoaderArgs) {
  const url = new URL(request.url);
  const kind = kinds.find((k) => k === url.searchParams.get("tipo")) ?? "";
  const page = Math.max(1, Number.parseInt(url.searchParams.get("pagina") ?? "1", 10) || 1);
  const list = await adminGet<Page<Asset>>(request, `/api/v1/admin/assets?page=${page}&page_size=24${kind ? `&kind=${kind}` : ""}`);
  return { kind, list };
}

export default function Media({ loaderData }: Route.ComponentProps) {
  const { kind, list } = loaderData;
  const { owner } = useRouteLoaderData<typeof layoutLoader>("routes/admin/layout")!;
  const revalidator = useRevalidator();
  const pages = Math.max(1, Math.ceil(list.total / list.page_size));
  const filters: { value: AssetKind | ""; label: string }[] = [{ value: "", label: "Todos" }, ...kinds.map((k) => ({ value: k, label: assetKindLabels[k] }))];
  return (
    <>
      <PageHeader title="Medios" description="Fotos, vídeos y recursos descargables. Todo lo que subes es privado hasta que una revisión publicada lo use y lo marques como público." />
      <Uploader
        csrf={owner.csrf_token}
        onUploaded={() => revalidator.revalidate()}
        renderDone={(a) => (
          <Link to={`/admin/medios/${a.id}`} className="text-xs underline underline-offset-2">
            Ver archivo<span className="sr-only"> {a.original_name}</span>
          </Link>
        )}
      />

      <nav aria-label="Filtrar por tipo" className="mt-8 flex flex-wrap gap-2 text-sm">
        {filters.map((f) => (
          <Link
            key={f.value}
            to={f.value ? `/admin/medios?tipo=${f.value}` : "/admin/medios"}
            aria-current={kind === f.value ? "page" : undefined}
            className={`rounded-full border px-3 py-1 ${kind === f.value ? "border-primary bg-primary text-primary-contrast" : "border-border-strong hover:bg-surface-muted"}`}
          >
            {f.label}
          </Link>
        ))}
      </nav>

      {list.items.length === 0 ? (
        <div className="mt-4">
          <EmptyState title="Todavía no hay archivos aquí." />
        </div>
      ) : (
        <ul className="mt-4 grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
          {list.items.map((a) => (
            <li key={a.id}>
              <Link to={`/admin/medios/${a.id}`} className="flex flex-col overflow-hidden rounded-md border border-border hover:border-accent focus-visible:outline-2 focus-visible:outline-accent">
                <AssetThumb asset={a} className="aspect-video w-full" />
                <span className="truncate px-3 pt-2 text-sm font-medium">{a.original_name}</span>
                <span className="flex flex-wrap gap-x-2 px-3 pb-2 font-mono text-[0.7rem] text-text-muted">
                  <span>{assetKindLabels[a.kind]}</span>
                  <span>{formatBytes(a.bytes)}</span>
                  <span>{a.public_enabled ? "público" : "privado"}</span>
                </span>
              </Link>
            </li>
          ))}
        </ul>
      )}
      {pages > 1 && (
        <nav aria-label="Paginación" className="mt-4 flex items-center justify-between text-sm">
          {list.page > 1 ? <Link to={`?${kind ? `tipo=${kind}&` : ""}pagina=${list.page - 1}`}>← Anterior</Link> : <span />}
          <span className="text-text-muted">Página {list.page} de {pages}</span>
          {list.page < pages ? <Link to={`?${kind ? `tipo=${kind}&` : ""}pagina=${list.page + 1}`}>Siguiente →</Link> : <span />}
        </nav>
      )}
      <p className="mt-6 font-mono text-xs text-text-muted">{list.total} archivos · actualizado {formatDate(new Date().toISOString())}</p>
    </>
  );
}

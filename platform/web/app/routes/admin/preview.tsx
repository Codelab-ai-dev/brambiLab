import { Link } from "react-router";
import type { Route } from "./+types/preview";
import { formatDate, localeLabels, revisionKindLabels, statusLabels, type Asset, type Content, type Revision, type Term } from "~/content/api-types";
import { DocumentView, type AssetMeta } from "~/content/DocumentView";
import type { Block } from "~/content/schema";
import { adminGet } from "~/lib/admin-api.server";
import { privateHeaders } from "~/lib/api.server";

export function headers() {
  return privateHeaders;
}

export function meta() {
  // The title stays generic: private text never goes into metadata that could leak.
  return [{ title: "Vista previa privada · Panel" }, { name: "robots", content: "noindex, nofollow" }];
}

export async function loader({ request, params }: Route.LoaderArgs) {
  if (params.locale !== "es" && params.locale !== "en") throw new Response(null, { status: 404 });
  const version = Number.parseInt(params.version, 10);
  if (!Number.isInteger(version) || version < 1) throw new Response(null, { status: 404 });
  const [content, revision, categories, tags] = await Promise.all([
    adminGet<Content>(request, `/api/v1/admin/contents/${params.id}`),
    adminGet<Revision>(request, `/api/v1/admin/contents/${params.id}/translations/${params.locale}/revisions/${version}`),
    adminGet<{ items: Term[] }>(request, "/api/v1/admin/categories"),
    adminGet<{ items: Term[] }>(request, "/api/v1/admin/tags"),
  ]);
  const locale: "es" | "en" = params.locale;
  // Metadata of every referenced asset (dimensions reserve space; sizes label downloads).
  const ids = new Set<string>(revision.cover_asset_id ? [revision.cover_asset_id] : []);
  const walk = (blocks: Block[]) =>
    blocks.forEach((b) => {
      if (b.type === "image" || b.type === "download") ids.add(b.attrs.assetId);
      if (b.type === "video") {
        ids.add(b.attrs.assetId);
        if (b.attrs.posterAssetId) ids.add(b.attrs.posterAssetId);
      }
      if ("content" in b && Array.isArray(b.content)) b.content.forEach((c) => "content" in c && Array.isArray(c.content) && walk(c.content as Block[]));
      if (b.type === "blockquote") walk(b.content);
    });
  walk(revision.body.content ?? []);
  const found = await Promise.all([...ids].map((id) => adminGet<Asset>(request, `/api/v1/admin/assets/${id}`).catch(() => null)));
  const assets: Record<string, AssetMeta> = {};
  for (const a of found) if (a) assets[a.id] = { width: a.width, height: a.height, bytes: a.bytes };
  return {
    assets,
    content,
    revision,
    locale,
    category: categories.items.find((c) => c.id === revision.category_id)?.labels[locale] ?? null,
    tags: tags.items.filter((t) => revision.tag_ids.includes(t.id)).map((t) => t.labels[locale]),
  };
}

export default function Preview({ loaderData, params }: Route.ComponentProps) {
  const { content, revision, locale, category, tags, assets } = loaderData;
  const pf = revision.project_fields;
  return (
    <div className="py-6">
      <div role="note" className="mb-6 flex flex-wrap items-center justify-between gap-2 rounded-md border border-warning bg-surface-muted px-4 py-3 text-sm">
        <span>
          <strong>Vista previa privada</strong> · versión {revision.version} ({revisionKindLabels[revision.kind].toLowerCase()}, {formatDate(revision.created_at)}) · {localeLabels[locale]} · no publicada
        </span>
        <span className="flex gap-3">
          <Link to={`/admin/contenidos/${params.id}/${locale}/historial`} className="underline">Historial</Link>
          <Link to={`/admin/contenidos/${params.id}/${locale}`} className="underline">Editor</Link>
        </span>
      </div>
      <article lang={locale} className="mx-auto max-w-3xl">
        {revision.cover_asset_id && (
          <img
            src={`/media/${revision.cover_asset_id}`}
            alt=""
            width={assets[revision.cover_asset_id]?.width ?? undefined}
            height={assets[revision.cover_asset_id]?.height ?? undefined}
            className="mb-8 aspect-[2/1] w-full rounded-md object-cover"
          />
        )}
        <header className="mb-8">
          {category && <p className="text-sm font-medium text-accent">{category}</p>}
          <h1 className="mt-1 text-3xl font-semibold break-words">{revision.title}</h1>
          {revision.summary && <p className="mt-3 text-lg text-text-muted">{revision.summary}</p>}
          {tags.length > 0 && (
            <ul className="mt-3 flex flex-wrap gap-2" aria-label="Etiquetas">
              {tags.map((t) => (
                <li key={t} className="rounded bg-surface-muted px-2 py-0.5 text-xs">{t}</li>
              ))}
            </ul>
          )}
        </header>
        {content.kind === "project" && pf && (
          <dl className="mb-8 grid gap-x-6 gap-y-3 rounded-md border border-border p-4 text-sm sm:grid-cols-[auto_1fr]">
            {pf.status && (<><dt className="font-medium">Estado</dt><dd>{statusLabels[pf.status]}</dd></>)}
            {pf.objective && (<><dt className="font-medium">Objetivo</dt><dd>{pf.objective}</dd></>)}
            {pf.technologies?.length ? (<><dt className="font-medium">Tecnologías</dt><dd>{pf.technologies.join(", ")}</dd></>) : null}
            {pf.links?.length ? (
              <>
                <dt className="font-medium">Enlaces</dt>
                <dd>
                  <ul>
                    {pf.links.map((l) => (
                      <li key={l.url}><a href={l.url} rel="noopener noreferrer" className="text-accent underline">{l.label}</a></li>
                    ))}
                  </ul>
                </dd>
              </>
            ) : null}
            {pf.results && (<><dt className="font-medium">Resultados</dt><dd>{pf.results}</dd></>)}
          </dl>
        )}
        <DocumentView doc={revision.body} locale={locale} assets={assets} />
      </article>
    </div>
  );
}

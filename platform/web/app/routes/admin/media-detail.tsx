import { Form, Link, redirect, useNavigation, useRouteLoaderData } from "react-router";
import type { Route } from "./+types/media-detail";
import { Button, Field, Notice, PageHeader, inputClass } from "~/components/admin/ui";
import { assetKindLabels, formatBytes, formatDate, localeLabels, type AssetDetail, type Locale } from "~/content/api-types";
import { adminGet } from "~/lib/admin-api.server";
import { assetRequest as send } from "~/lib/admin-client";
import { privateHeaders } from "~/lib/api.server";
import type { loader as layoutLoader } from "./layout";

export function headers() {
  return privateHeaders;
}

export function meta() {
  return [{ title: "Archivo · Panel" }, { name: "robots", content: "noindex, nofollow" }];
}

export async function loader({ request, params }: Route.LoaderArgs) {
  return { asset: await adminGet<AssetDetail>(request, `/api/v1/admin/assets/${params.id}`) };
}

export async function clientAction({ request, params }: Route.ClientActionArgs) {
  const form = await request.formData();
  const csrf = String(form.get("csrf_token"));
  if (form.get("intent") === "delete") {
    const r = await send("DELETE", params.id, csrf);
    if (r.ok) return redirect("/admin/medios");
    return { ok: false as const, message: r.json?.code === "asset_in_use" ? "Hay revisiones que usan este archivo; no se puede borrar." : "No se pudo borrar." };
  }
  const texts = Object.fromEntries(
    (["es", "en"] as Locale[]).map((l) => [l, { alt: String(form.get(`alt_${l}`) ?? ""), caption: String(form.get(`caption_${l}`) ?? "") }]),
  );
  const r = await send("PATCH", params.id, csrf, {
    public_enabled: form.get("public_enabled") === "on",
    downloadable: form.get("downloadable") === "on",
    texts,
  });
  return r.ok ? { ok: true as const, message: "Guardado." } : { ok: false as const, message: "Revisa los campos: alt hasta 300 caracteres y pie hasta 500." };
}

const usageLabels = { image: "imagen", video: "vídeo", poster: "póster", download: "descarga", cover: "portada" } as const;

export default function MediaDetail({ loaderData, actionData }: Route.ComponentProps) {
  const { asset } = loaderData;
  const { owner } = useRouteLoaderData<typeof layoutLoader>("routes/admin/layout")!;
  const busy = useNavigation().state !== "idle";
  const inUse = asset.references.length > 0;
  const publishedUse = asset.references.some((r) => r.published);
  return (
    <>
      <PageHeader
        title={asset.original_name}
        description={
          <span className="font-mono text-xs">
            {assetKindLabels[asset.kind]} · {asset.mime} · {formatBytes(asset.bytes)}
            {asset.width ? ` · ${asset.width}×${asset.height} px` : ""} · subido {formatDate(asset.created_at)}
          </span>
        }
        actions={<Link to="/admin/medios" className="text-sm underline">← Medios</Link>}
      />
      {actionData && <Notice tone={actionData.ok ? "success" : "danger"}>{actionData.message}</Notice>}

      <div className="mt-4 grid gap-8 lg:grid-cols-[minmax(0,1fr)_22rem]">
        <section aria-label="Vista previa" className="flex min-w-0 flex-col gap-3">
          {asset.kind === "image" && <img src={`/media/${asset.id}`} alt={asset.texts.es?.alt ?? ""} width={asset.width ?? undefined} height={asset.height ?? undefined} className="h-auto max-w-full rounded-md border border-border" />}
          {asset.kind === "video" && <video src={`/media/${asset.id}`} controls preload="metadata" className="w-full rounded-md border border-border" />}
          {asset.kind === "resource" && (
            <a href={`/media/${asset.id}/download`} className="self-start rounded-md border border-border-strong px-4 py-2 text-sm underline-offset-2 hover:underline">
              Descargar {asset.original_name}
            </a>
          )}
          <p className="font-mono text-[0.7rem] break-all text-text-muted">sha256 {asset.sha256}</p>

          <h2 className="mt-4 text-lg font-semibold">Dónde se usa</h2>
          {inUse ? (
            <ul className="divide-y divide-border rounded-md border border-border text-sm">
              {asset.references.map((r, i) => (
                <li key={i} className="flex flex-wrap items-center justify-between gap-2 px-4 py-2">
                  <Link to={`/admin/contenidos/${r.content_id}/${r.locale}/v/${r.version}`} className="text-accent hover:underline">
                    {localeLabels[r.locale]} · v{r.version}
                  </Link>
                  <span className="font-mono text-xs text-text-muted">
                    {usageLabels[r.usage]}
                    {r.published ? " · publicada" : ""}
                  </span>
                </li>
              ))}
            </ul>
          ) : (
            <p className="text-sm text-text-muted">Ninguna revisión lo usa todavía.</p>
          )}
        </section>

        <Form method="post" className="flex flex-col gap-5">
          <input type="hidden" name="csrf_token" value={owner.csrf_token} />
          {(["es", "en"] as Locale[]).map((l) => (
            <fieldset key={l} className="flex flex-col gap-3 rounded-md border border-border p-3">
              <legend className="px-1 text-sm font-medium">{localeLabels[l]}</legend>
              <Field label="Texto alternativo" help="Describe la imagen para quien no la ve. Se propone al insertarla; cada documento guarda el suyo.">
                {({ id, describedBy }) => <input id={id} name={`alt_${l}`} defaultValue={asset.texts[l]?.alt ?? ""} maxLength={300} className={inputClass} aria-describedby={describedBy} />}
              </Field>
              <Field label="Pie">
                {({ id }) => <input id={id} name={`caption_${l}`} defaultValue={asset.texts[l]?.caption ?? ""} maxLength={500} className={inputClass} />}
              </Field>
            </fieldset>
          ))}
          <fieldset className="flex flex-col gap-2 rounded-md border border-border p-3">
            <legend className="px-1 text-sm font-medium">Permisos</legend>
            <label className="flex items-start gap-2 text-sm">
              <input type="checkbox" name="public_enabled" defaultChecked={asset.public_enabled} className="mt-1" />
              <span>
                Permitir que sea público
                <span className="block text-xs text-text-muted">Sólo se sirve a visitantes cuando además lo usa una revisión publicada. {publishedUse ? "Ahora mismo lo usa al menos una." : "Ahora mismo ninguna revisión publicada lo usa."}</span>
              </span>
            </label>
            <label className="flex items-start gap-2 text-sm">
              <input type="checkbox" name="downloadable" defaultChecked={asset.downloadable} className="mt-1" />
              <span>
                Permitir descarga
                <span className="block text-xs text-text-muted">Necesario para PDF, ZIP y STL; en imágenes y vídeos añade un enlace de descarga.</span>
              </span>
            </label>
          </fieldset>
          <Button type="submit" tone="primary" disabled={busy} className="self-start">Guardar</Button>
        </Form>
      </div>

      <Form method="post" className="mt-10 border-t border-border pt-6">
        <input type="hidden" name="csrf_token" value={owner.csrf_token} />
        <input type="hidden" name="intent" value="delete" />
        <Button
          type="submit"
          tone="danger"
          disabled={busy || inUse}
          aria-describedby="delete-note"
          onClick={(e) => {
            if (!confirm(`¿Borrar ${asset.original_name}? No se puede deshacer.`)) e.preventDefault();
          }}
        >
          Borrar archivo
        </Button>
        <p id="delete-note" className="mt-2 text-xs text-text-muted">
          {inUse ? "No se puede borrar mientras alguna revisión lo use (también las antiguas)." : "Se borra el archivo del servidor."}
        </p>
      </Form>
    </>
  );
}

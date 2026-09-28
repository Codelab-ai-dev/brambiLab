import { Form, useNavigation, useRouteLoaderData } from "react-router";
import type { Route } from "./+types/site";
import { Button, Field, Notice, PageHeader, inputClass } from "~/components/admin/ui";
import { displayTitle, formatDate, type Content, type Page } from "~/content/api-types";
import { adminGet } from "~/lib/admin-api.server";
import { apiSend, describe } from "~/lib/admin-client";
import { privateHeaders } from "~/lib/api.server";
import type { loader as layoutLoader } from "./layout";

// Site settings (web-v1.md §9.1): saved explicitly with "Guardar cambios públicos", effective at
// once, guarded by the settings version. Featured projects only show while published.

type LinkKind = "github" | "linkedin" | "website" | "other";
type Settings = {
  version: number;
  intro: { es: string; en: string };
  bio: { es: string; en: string };
  contact_email: string;
  links: { kind: LinkKind; label: string; url: string }[];
  featured_projects: { content_id: string; titles: Record<string, string | null>; visible: Record<string, boolean> }[];
  updated_at: string;
};

const MAX_LINKS = 8;
const MAX_FEATURED = 6;
const linkKinds: Record<LinkKind, string> = { github: "GitHub", linkedin: "LinkedIn", website: "Sitio web", other: "Otro" };

export function headers() {
  return privateHeaders;
}

export function meta() {
  return [{ title: "Sitio público · Panel" }, { name: "robots", content: "noindex, nofollow" }];
}

export async function loader({ request }: Route.LoaderArgs) {
  const [settings, projects] = await Promise.all([
    adminGet<Settings>(request, "/api/v1/admin/site"),
    adminGet<Page<Content>>(request, "/api/v1/admin/contents?kind=project&page_size=100"),
  ]);
  return { settings, projects: projects.items };
}

export async function clientAction({ request }: Route.ClientActionArgs) {
  const form = await request.formData();
  const links = [];
  for (let i = 0; i < MAX_LINKS; i++) {
    const url = String(form.get(`link_url_${i}`) ?? "").trim();
    const label = String(form.get(`link_label_${i}`) ?? "").trim();
    if (!url && !label) continue; // empty rows are ignored
    links.push({ kind: String(form.get(`link_kind_${i}`)), label, url, row: i });
  }
  // Featured: checked projects ordered by their position field (ties keep the list order).
  const featured = form
    .getAll("featured")
    .map(String)
    .map((id, i) => ({ id, pos: Number(form.get(`position_${id}`)) || 99, i }))
    .sort((a, b) => a.pos - b.pos || a.i - b.i)
    .map((f) => f.id);
  const body = {
    expected_version: Number(form.get("expected_version")),
    intro: { es: String(form.get("intro_es")), en: String(form.get("intro_en")) },
    bio: { es: String(form.get("bio_es")), en: String(form.get("bio_en")) },
    contact_email: String(form.get("contact_email")),
    links: links.map(({ row: _row, ...l }) => l),
    featured_project_ids: featured,
  };
  const result = await apiSend<Settings>("PUT", "/api/v1/admin/site", String(form.get("csrf_token")), body);
  if (result.ok) return { ok: true as const, message: `Cambios públicos guardados (versión ${result.data.version}). Ya se ven en el sitio.`, fields: {} as Record<string, string> };
  // Map API field keys (links.N.url) back to the form rows the owner filled.
  const fields: Record<string, string> = {};
  for (const [key, msg] of Object.entries(result.error.fields ?? {})) {
    const m = /^links\.(\d+)\.(\w+)$/.exec(key);
    fields[m ? `link_${m[2]}_${links[Number(m[1])]?.row}` : key] = msg;
  }
  const message =
    result.error.code === "version_conflict"
      ? "Otra pestaña guardó cambios mientras tanto. Recarga la página para verlos; tus cambios no se guardaron."
      : result.status === 422
        ? "Revisa los campos marcados; no se guardó nada."
        : describe(result.error);
  return { ok: false as const, message, fields };
}

export default function SiteSettings({ loaderData, actionData }: Route.ComponentProps) {
  const { settings, projects } = loaderData;
  const { owner } = useRouteLoaderData<typeof layoutLoader>("routes/admin/layout")!;
  const busy = useNavigation().state !== "idle";
  const fields = actionData?.fields ?? {};
  const rows = Array.from({ length: Math.min(MAX_LINKS, settings.links.length + 2) }, (_, i) => settings.links[i] ?? { kind: "github" as LinkKind, label: "", url: "" });
  const featuredIndex = new Map(settings.featured_projects.map((f, i) => [f.content_id, i]));
  const visibility = new Map(settings.featured_projects.map((f) => [f.content_id, f.visible]));
  const active = projects.filter((p) => !p.archived_at);

  return (
    <>
      <PageHeader
        title="Sitio público"
        description={
          <>
            Presentación, biografía, contacto y proyectos destacados. Versión {settings.version} · guardada {formatDate(settings.updated_at)}.
          </>
        }
      />
      {actionData && <Notice tone={actionData.ok ? "success" : "danger"}>{actionData.message}</Notice>}

      <Form method="post" className="mt-4 flex flex-col gap-8">
        <input type="hidden" name="csrf_token" value={owner.csrf_token} />
        <input type="hidden" name="expected_version" value={settings.version} />

        <fieldset className="grid min-w-0 gap-4 md:grid-cols-2">
          <legend className="mb-2 text-lg font-semibold">Presentación y biografía</legend>
          {(["es", "en"] as const).map((l) => (
            <div key={l} className="flex flex-col gap-4">
              <Field label={`Presentación (${l === "es" ? "español" : "inglés"})`} help="Aparece en el inicio. Hasta 500 caracteres." error={fields[`intro.${l}`]}>
                {({ id, describedBy, invalid }) => (
                  <textarea id={id} name={`intro_${l}`} rows={3} maxLength={500} defaultValue={settings.intro[l]} className={inputClass} aria-describedby={describedBy} aria-invalid={invalid} />
                )}
              </Field>
              <Field label={`Biografía (${l === "es" ? "español" : "inglés"})`} help="Página «Acerca de». Separa párrafos con una línea en blanco. Hasta 5000 caracteres." error={fields[`bio.${l}`]}>
                {({ id, describedBy, invalid }) => (
                  <textarea id={id} name={`bio_${l}`} rows={8} maxLength={5000} defaultValue={settings.bio[l]} className={inputClass} aria-describedby={describedBy} aria-invalid={invalid} />
                )}
              </Field>
            </div>
          ))}
          <p className="text-xs text-text-muted md:col-span-2">Cada idioma se escribe a mano; un campo vacío se muestra como vacío, sin traducir el otro.</p>
        </fieldset>

        <fieldset className="flex min-w-0 flex-col gap-4">
          <legend className="mb-2 text-lg font-semibold">Contacto</legend>
          <div className="max-w-md">
            <Field label="Correo de contacto" help="Se publica tal cual. Déjalo vacío para no mostrar correo." error={fields.contact_email}>
              {({ id, describedBy, invalid }) => (
                <input id={id} name="contact_email" type="email" defaultValue={settings.contact_email} className={inputClass} aria-describedby={describedBy} aria-invalid={invalid} />
              )}
            </Field>
          </div>
          <p className="text-sm text-text-muted">Enlaces (sólo https://, hasta {MAX_LINKS}). Una fila vacía se ignora.</p>
          <ol className="flex flex-col gap-3">
            {rows.map((l, i) => (
              <li key={i}>
                <fieldset className="grid min-w-0 gap-3 rounded-md border border-border p-3 sm:grid-cols-[10rem_minmax(0,1fr)_minmax(0,2fr)]">
                  <legend className="px-1 text-xs font-medium text-text-muted">Enlace {i + 1}</legend>
                  <label className="flex min-w-0 flex-col gap-1 text-sm font-medium">
                    Tipo
                    <select name={`link_kind_${i}`} defaultValue={l.kind} aria-label={`Tipo del enlace ${i + 1}`} className={inputClass}>
                      {Object.entries(linkKinds).map(([k, label]) => (
                        <option key={k} value={k}>
                          {label}
                        </option>
                      ))}
                    </select>
                  </label>
                  <label className="flex min-w-0 flex-col gap-1 text-sm font-medium">
                    Texto
                    <input name={`link_label_${i}`} defaultValue={l.label} maxLength={60} aria-label={`Texto del enlace ${i + 1}`} aria-invalid={fields[`link_label_${i}`] ? true : undefined} className={inputClass} />
                    {fields[`link_label_${i}`] && <span className="text-xs font-normal text-danger">{fields[`link_label_${i}`]}</span>}
                  </label>
                  <label className="flex min-w-0 flex-col gap-1 text-sm font-medium">
                    URL
                    <input name={`link_url_${i}`} type="url" defaultValue={l.url} placeholder="https://" aria-label={`URL del enlace ${i + 1}`} aria-invalid={fields[`link_url_${i}`] ? true : undefined} className={`${inputClass} font-mono`} />
                    {fields[`link_url_${i}`] && <span className="text-xs font-normal text-danger">{fields[`link_url_${i}`]}</span>}
                  </label>
                </fieldset>
              </li>
            ))}
          </ol>
        </fieldset>

        <fieldset className="flex min-w-0 flex-col gap-3">
          <legend className="mb-2 text-lg font-semibold">Proyectos destacados</legend>
          <p className="text-sm text-text-muted">
            Hasta {MAX_FEATURED}, en el orden indicado. Destacar no publica nada: un proyecto sólo aparece en el inicio del idioma en que esté publicado.
          </p>
          {fields.featured_project_ids && <p className="text-sm text-danger">{fields.featured_project_ids}</p>}
          {active.length === 0 ? (
            <p className="text-sm text-text-muted">Todavía no hay proyectos.</p>
          ) : (
            <ul className="divide-y divide-border rounded-md border border-border text-sm">
              {active.map((p) => {
                const idx = featuredIndex.get(p.id);
                const vis = visibility.get(p.id);
                const published = (l: "es" | "en") => p.translations.find((t) => t.locale === l)?.published ?? false;
                return (
                  <li key={p.id} className="flex flex-col gap-2 px-4 py-3 sm:flex-row sm:items-center sm:gap-4">
                    <label className="flex min-w-0 items-start gap-2 sm:flex-1">
                      <input type="checkbox" name="featured" value={p.id} defaultChecked={idx !== undefined} className="mt-1" />
                      <span className="min-w-0 [overflow-wrap:anywhere]">{displayTitle(p, "Proyecto sin título")}</span>
                    </label>
                    <span className="font-mono text-xs text-text-muted">
                      ES {(vis?.es ?? published("es")) ? "publicado" : "no publicado"} · EN {(vis?.en ?? published("en")) ? "publicado" : "no publicado"}
                    </span>
                    <label className="flex items-center gap-1 text-xs text-text-muted">
                      Orden
                      <input name={`position_${p.id}`} type="number" min={1} max={MAX_FEATURED} defaultValue={idx !== undefined ? idx + 1 : ""} className="w-16 rounded-md border border-border-strong bg-surface px-2 py-1" />
                    </label>
                  </li>
                );
              })}
            </ul>
          )}
        </fieldset>

        <div className="flex flex-col gap-2 rounded-md border border-warning p-4">
          <p className="text-sm">Al guardar, los cambios se ven en el sitio público de inmediato. No hay borrador ni historial de ajustes.</p>
          <Button type="submit" tone="primary" className="self-start" disabled={busy}>
            Guardar cambios públicos
          </Button>
        </div>
      </Form>
    </>
  );
}

import { Form, redirect, useNavigation, useRouteLoaderData } from "react-router";
import type { Route } from "./+types/content-new";
import { Button, Field, Notice, PageHeader, inputClass } from "~/components/admin/ui";
import { kindLabels, localeLabels, type Content, type ContentKind, type Page } from "~/content/api-types";
import { adminGet } from "~/lib/admin-api.server";
import { apiSend, describe } from "~/lib/admin-client";
import { privateHeaders } from "~/lib/api.server";
import type { loader as layoutLoader } from "./layout";

export function headers() {
  return privateHeaders;
}

export function meta() {
  return [{ title: "Nuevo contenido · Panel" }, { name: "robots", content: "noindex, nofollow" }];
}

export async function loader({ request }: Route.LoaderArgs) {
  const url = new URL(request.url);
  const kind = (["project", "article", "log"] as const).find((k) => k === url.searchParams.get("tipo")) ?? "project";
  const project = url.searchParams.get("proyecto");
  const projects =
    kind === "log" ? (await adminGet<Page<Content>>(request, "/api/v1/admin/contents?kind=project&page_size=100")).items : [];
  return { kind, project, projects };
}

// Runs in the browser: the request goes to Go with the session CSRF token.
export async function clientAction({ request }: Route.ClientActionArgs) {
  const form = await request.formData();
  const kind = String(form.get("kind")) as ContentKind;
  const locale = String(form.get("locale"));
  const body: Record<string, string> = { kind, locale };
  if (kind === "log") body.project_id = String(form.get("project_id") ?? "");
  const result = await apiSend<Content>("POST", "/api/v1/admin/contents", String(form.get("csrf_token")), body);
  if (!result.ok) return { message: describe(result.error), fields: result.error.fields ?? {} };
  return redirect(`/admin/contenidos/${result.data.id}/${locale}`);
}

export default function NewContent({ loaderData, actionData }: Route.ComponentProps) {
  const { kind, project, projects } = loaderData;
  const { owner } = useRouteLoaderData<typeof layoutLoader>("routes/admin/layout")!;
  const busy = useNavigation().state !== "idle";
  const fields = actionData?.fields ?? {};
  return (
    <>
      <PageHeader title={kindLabels[kind].new} description="Primero eliges el idioma inicial; la otra traducción se añade después desde la ficha." />
      {actionData?.message && <Notice tone="danger">{actionData.message}</Notice>}
      <Form method="post" className="mt-4 flex max-w-lg flex-col gap-5">
        <input type="hidden" name="csrf_token" value={owner.csrf_token} />
        <input type="hidden" name="kind" value={kind} />
        <fieldset className="flex flex-col gap-2">
          <legend className="text-sm font-medium">Idioma inicial</legend>
          {(["es", "en"] as const).map((l) => (
            <label key={l} className="flex items-center gap-2 text-sm">
              <input type="radio" name="locale" value={l} defaultChecked={l === "es"} /> {localeLabels[l]}
            </label>
          ))}
        </fieldset>
        {kind === "log" &&
          (projects.length === 0 ? (
            <Notice tone="warning">Una entrada de bitácora necesita un proyecto. Crea primero un proyecto.</Notice>
          ) : (
            <Field label="Proyecto" help="La entrada pertenece a este proyecto y no se puede mover después." error={fields.project_id}>
              {({ id, describedBy, invalid }) => (
                <select id={id} name="project_id" defaultValue={project ?? ""} required aria-describedby={describedBy} aria-invalid={invalid} className={inputClass}>
                  <option value="" disabled>
                    Elige un proyecto
                  </option>
                  {projects.map((p) => (
                    <option key={p.id} value={p.id}>
                      {p.translations.find((t) => t.title)?.title ?? `Proyecto sin título (${p.id.slice(0, 8)})`}
                    </option>
                  ))}
                </select>
              )}
            </Field>
          ))}
        <div>
          <Button type="submit" tone="primary" disabled={busy || (kind === "log" && projects.length === 0)}>
            {busy ? "Creando…" : "Crear y empezar a escribir"}
          </Button>
        </div>
      </Form>
    </>
  );
}

import { Form, Link, useNavigation, useRouteLoaderData } from "react-router";
import type { Route } from "./+types/content";
import { Button, EmptyState, LinkButton, Notice, PageHeader } from "~/components/admin/ui";
import { displayTitle, formatDate, kindLabels, localeLabels, type Content, type Locale, type Page } from "~/content/api-types";
import { adminGet } from "~/lib/admin-api.server";
import { apiSend, describe } from "~/lib/admin-client";
import { privateHeaders } from "~/lib/api.server";
import type { loader as layoutLoader } from "./layout";

export function headers() {
  return privateHeaders;
}

export function meta({ loaderData }: Route.MetaArgs) {
  return [{ title: `${loaderData ? titleOf(loaderData.content) : "Contenido"} · Panel` }, { name: "robots", content: "noindex, nofollow" }];
}

function titleOf(c: Content) {
  return displayTitle(c, `${kindLabels[c.kind].one} sin título`);
}

export async function loader({ request, params }: Route.LoaderArgs) {
  const content = await adminGet<Content>(request, `/api/v1/admin/contents/${params.id}`);
  const logs =
    content.kind === "project"
      ? (await adminGet<Page<Content>>(request, `/api/v1/admin/contents?kind=log&project_id=${content.id}&page_size=100`)).items
      : [];
  const project = content.kind === "log" && content.project_id ? await adminGet<Content>(request, `/api/v1/admin/contents/${content.project_id}`) : null;
  return { content, logs, project };
}

export async function clientAction({ request, params }: Route.ClientActionArgs) {
  const form = await request.formData();
  const csrf = String(form.get("csrf_token"));
  const intent = String(form.get("intent"));
  const base = `/api/v1/admin/contents/${params.id}`;
  const result =
    intent === "translate"
      ? await apiSend("POST", `${base}/translations`, csrf, {
          locale: String(form.get("locale")),
          ...(form.get("copy_from") ? { copy_from_locale: String(form.get("copy_from")) } : {}),
        })
      : await apiSend("POST", `${base}/${intent === "archive" ? "archive" : "unarchive"}`, csrf);
  if (!result.ok) {
    const message =
      result.error.code === "published_content" ? "No se puede archivar un contenido publicado. Retíralo primero (WEB-005)." : describe(result.error);
    return { ok: false as const, message };
  }
  return { ok: true as const, message: intent === "translate" ? "Traducción creada." : intent === "archive" ? "Contenido archivado." : "Contenido activo de nuevo." };
}

export default function ContentOverview({ loaderData, actionData }: Route.ComponentProps) {
  const { content, logs, project } = loaderData;
  const { owner } = useRouteLoaderData<typeof layoutLoader>("routes/admin/layout")!;
  const busy = useNavigation().state !== "idle";
  const archived = content.archived_at !== null;
  const have = new Set(content.translations.map((t) => t.locale));
  const csrf = <input type="hidden" name="csrf_token" value={owner.csrf_token} />;

  return (
    <>
      <PageHeader
        title={titleOf(content)}
        description={
          <>
            {kindLabels[content.kind].one} · creado {formatDate(content.created_at)}
            {archived && <> · <strong className="text-warning">archivado {formatDate(content.archived_at)}</strong></>}
            {project && (
              <>
                {" "}· proyecto <Link className="underline" to={`/admin/contenidos/${project.id}`}>{titleOf(project)}</Link>
              </>
            )}
          </>
        }
        actions={
          <>
            <Button disabled title="Disponible con WEB-005" aria-describedby="publish-note">
              Publicar
            </Button>
            <Form method="post">
              {csrf}
              <input type="hidden" name="intent" value={archived ? "unarchive" : "archive"} />
              <Button
                type="submit"
                tone={archived ? "secondary" : "danger"}
                disabled={busy}
                onClick={(e) => {
                  if (!archived && !confirm("¿Archivar este contenido? Queda en sólo lectura y conserva todo su historial.")) e.preventDefault();
                }}
              >
                {archived ? "Desarchivar" : "Archivar"}
              </Button>
            </Form>
          </>
        }
      />
      <p id="publish-note" className="-mt-4 mb-4 text-xs text-text-muted">Publicar llegará con WEB-005.</p>
      {actionData && <Notice tone={actionData.ok ? "success" : "danger"}>{actionData.message}</Notice>}

      <section aria-labelledby="translations" className="mt-6">
        <h2 id="translations" className="text-lg font-semibold">Traducciones</h2>
        <div className="mt-3 grid gap-4 sm:grid-cols-2">
          {(["es", "en"] as Locale[]).map((locale) => {
            const t = content.translations.find((x) => x.locale === locale);
            const other = locale === "es" ? "en" : "es";
            const otherHasText = (content.translations.find((x) => x.locale === other)?.latest_version ?? 0) > 0;
            return (
              <div key={locale} className="flex flex-col gap-3 rounded-md border border-border p-4">
                <h3 className="font-medium">{localeLabels[locale]}</h3>
                {t ? (
                  <>
                    <p className="text-sm text-text-muted">
                      {t.latest_version ? `${t.title} · v${t.latest_version} · ${formatDate(t.updated_at)}` : "Sin revisiones todavía."}
                    </p>
                    <div className="flex flex-wrap gap-2">
                      <LinkButton tone="primary" to={`/admin/contenidos/${content.id}/${locale}`}>
                        {archived ? "Ver" : "Editar"}
                      </LinkButton>
                      {t.latest_version > 0 && <LinkButton to={`/admin/contenidos/${content.id}/${locale}/historial`}>Historial</LinkButton>}
                    </div>
                  </>
                ) : archived ? (
                  <p className="text-sm text-text-muted">No existe. Desarchiva para añadirla.</p>
                ) : (
                  <Form method="post" className="flex flex-wrap gap-2">
                    {csrf}
                    <input type="hidden" name="intent" value="translate" />
                    <input type="hidden" name="locale" value={locale} />
                    <Button type="submit" disabled={busy}>
                      Crear vacía
                    </Button>
                    {have.has(other) && (
                      <Button type="submit" name="copy_from" value={other} disabled={busy || !otherHasText} title={otherHasText ? undefined : "La otra traducción aún no tiene texto"}>
                        Copiar desde {other.toUpperCase()} como borrador
                      </Button>
                    )}
                  </Form>
                )}
              </div>
            );
          })}
        </div>
        <p className="mt-2 text-xs text-text-muted">Una copia queda marcada como pendiente de traducir; no se traduce automáticamente.</p>
      </section>

      {content.kind === "project" && (
        <section aria-labelledby="logs" className="mt-8">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <h2 id="logs" className="text-lg font-semibold">Bitácora del proyecto</h2>
            {!archived && <LinkButton to={`/admin/contenidos/nuevo?tipo=log&proyecto=${content.id}`}>Nueva entrada</LinkButton>}
          </div>
          {logs.length === 0 ? (
            <div className="mt-3">
              <EmptyState title="Este proyecto aún no tiene entradas de bitácora." />
            </div>
          ) : (
            <ul className="mt-3 divide-y divide-border rounded-md border border-border">
              {logs.map((l) => (
                <li key={l.id} className="flex flex-wrap items-center justify-between gap-2 px-4 py-3 text-sm">
                  <Link to={`/admin/contenidos/${l.id}`} className="font-medium text-accent hover:underline">
                    {titleOf(l)}
                  </Link>
                  <span className="text-text-muted">{formatDate(l.created_at)}</span>
                </li>
              ))}
            </ul>
          )}
        </section>
      )}
    </>
  );
}

import { Form, useNavigation, useRouteLoaderData } from "react-router";
import type { Route } from "./+types/taxonomy";
import { Button, EmptyState, Field, Notice, PageHeader, inputClass } from "~/components/admin/ui";
import type { Term } from "~/content/api-types";
import { adminGet } from "~/lib/admin-api.server";
import { apiSend, describe } from "~/lib/admin-client";
import { privateHeaders } from "~/lib/api.server";
import type { loader as layoutLoader } from "./layout";

export function headers() {
  return privateHeaders;
}

export function meta() {
  return [{ title: "Categorías y etiquetas · Panel" }, { name: "robots", content: "noindex, nofollow" }];
}

export async function loader({ request }: Route.LoaderArgs) {
  const [categories, tags] = await Promise.all([
    adminGet<{ items: Term[] }>(request, "/api/v1/admin/categories"),
    adminGet<{ items: Term[] }>(request, "/api/v1/admin/tags"),
  ]);
  return { categories: categories.items, tags: tags.items };
}

export async function clientAction({ request }: Route.ClientActionArgs) {
  const form = await request.formData();
  const kind = form.get("kind") === "tags" ? "tags" : "categories";
  const result = await apiSend("POST", `/api/v1/admin/${kind}`, String(form.get("csrf_token")), {
    slug: String(form.get("slug")),
    labels: { es: String(form.get("label_es")), en: String(form.get("label_en")) },
  });
  if (!result.ok) {
    return { kind, ok: false as const, message: result.error.code === "slug_taken" ? "Ese slug ya existe." : describe(result.error), fields: result.error.fields ?? {} };
  }
  return { kind, ok: true as const, message: "Creado.", fields: {} as Record<string, string> };
}

export default function Taxonomy({ loaderData, actionData }: Route.ComponentProps) {
  const { owner } = useRouteLoaderData<typeof layoutLoader>("routes/admin/layout")!;
  const busy = useNavigation().state !== "idle";
  return (
    <>
      <PageHeader title="Categorías y etiquetas" description="Cada término tiene un nombre en español y en inglés. Renombrar no cambia qué revisiones lo usan." />
      <div className="grid gap-8 md:grid-cols-2">
        {([
          ["categories", "Categorías", loaderData.categories],
          ["tags", "Etiquetas", loaderData.tags],
        ] as const).map(([kind, title, items]) => {
          const result = actionData?.kind === kind ? actionData : null;
          return (
            <section key={kind} aria-labelledby={`${kind}-title`} className="flex flex-col gap-4">
              <h2 id={`${kind}-title`} className="text-lg font-semibold">{title}</h2>
              {items.length === 0 ? (
                <EmptyState title={`Sin ${title.toLowerCase()} todavía.`} />
              ) : (
                <ul className="divide-y divide-border rounded-md border border-border text-sm">
                  {items.map((t) => (
                    <li key={t.id} className="flex flex-wrap justify-between gap-2 px-4 py-2">
                      <span>{t.labels.es} <span className="text-text-muted">/ {t.labels.en}</span></span>
                      <code className="text-xs text-text-muted">{t.slug}</code>
                    </li>
                  ))}
                </ul>
              )}
              <Form method="post" className="flex flex-col gap-3 rounded-md border border-border p-4">
                <h3 className="text-sm font-semibold">Añadir {kind === "tags" ? "etiqueta" : "categoría"}</h3>
                {result && <Notice tone={result.ok ? "success" : "danger"}>{result.message}</Notice>}
                <input type="hidden" name="csrf_token" value={owner.csrf_token} />
                <input type="hidden" name="kind" value={kind} />
                <Field label="Nombre en español" error={result?.fields["labels.es"]}>
                  {({ id, describedBy, invalid }) => <input id={id} name="label_es" required maxLength={60} className={inputClass} aria-describedby={describedBy} aria-invalid={invalid} />}
                </Field>
                <Field label="Nombre en inglés" error={result?.fields["labels.en"]}>
                  {({ id, describedBy, invalid }) => <input id={id} name="label_en" required maxLength={60} className={inputClass} aria-describedby={describedBy} aria-invalid={invalid} />}
                </Field>
                <Field label="Slug" help="Minúsculas, números y guiones." error={result?.fields.slug}>
                  {({ id, describedBy, invalid }) => <input id={id} name="slug" required pattern="[a-z0-9]+(-[a-z0-9]+)*" maxLength={60} className={`${inputClass} font-mono`} aria-describedby={describedBy} aria-invalid={invalid} />}
                </Field>
                <Button type="submit" tone="primary" className="self-start" disabled={busy}>Añadir</Button>
              </Form>
            </section>
          );
        })}
      </div>
    </>
  );
}

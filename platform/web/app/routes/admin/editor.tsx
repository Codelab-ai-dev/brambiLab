import { useRouteLoaderData, type ShouldRevalidateFunctionArgs } from "react-router";
import type { Route } from "./+types/editor";
import type { Content, EditorialState, Page, PublicationJob, Term, Translation } from "~/content/api-types";
import { EditorScreen } from "~/content/EditorScreen";
import { adminGet } from "~/lib/admin-api.server";
import { publicationPath } from "~/lib/admin-client";
import { privateHeaders } from "~/lib/api.server";
import type { loader as layoutLoader } from "./layout";

export function headers() {
  return privateHeaders;
}

export function meta({ loaderData }: Route.MetaArgs) {
  return [{ title: `${loaderData?.translation.latest?.title || "Editar"} · Panel` }, { name: "robots", content: "noindex, nofollow" }];
}

export async function loader({ request, params }: Route.LoaderArgs) {
  if (params.locale !== "es" && params.locale !== "en") throw new Response(null, { status: 404 });
  const base = `/api/v1/admin/contents/${params.id}`;
  const [content, translation, categories, tags, publication, jobs] = await Promise.all([
    adminGet<Content>(request, base),
    adminGet<Translation>(request, `${base}/translations/${params.locale}`),
    adminGet<{ items: Term[] }>(request, "/api/v1/admin/categories"),
    adminGet<{ items: Term[] }>(request, "/api/v1/admin/tags"),
    adminGet<EditorialState>(request, publicationPath(params.id, params.locale)),
    adminGet<Page<PublicationJob>>(request, `${publicationPath(params.id, params.locale, "jobs")}?page_size=5`),
  ]);
  return { content, translation, categories: categories.items, tags: tags.items, publication, jobs: jobs.items };
}

// Autosave keeps this page's data fresh; do not refetch (and reset the editor) after every save.
export function shouldRevalidate({ currentParams, nextParams, defaultShouldRevalidate }: ShouldRevalidateFunctionArgs) {
  return currentParams.id !== nextParams.id || currentParams.locale !== nextParams.locale ? defaultShouldRevalidate : false;
}

export default function EditorRoute({ loaderData }: Route.ComponentProps) {
  const { owner } = useRouteLoaderData<typeof layoutLoader>("routes/admin/layout")!;
  const { content, translation, categories, tags, publication, jobs } = loaderData;
  // key: a different translation gets a fresh screen (and a fresh autosaver).
  return <EditorScreen key={`${content.id}-${translation.locale}`} content={content} translation={translation} categories={categories} tags={tags} csrf={owner.csrf_token} publication={publication} jobs={jobs} />;
}

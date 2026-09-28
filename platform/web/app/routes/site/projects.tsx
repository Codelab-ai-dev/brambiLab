import type { Route } from "./+types/projects";
import { listLoader } from "~/site/list.server";
import { ListPage } from "~/site/ListPage";
import { forwardHeaders, seoMeta } from "~/site/seo";

export const headers = forwardHeaders;

export function meta({ loaderData }: Route.MetaArgs) {
  return seoMeta(loaderData?.seo);
}

export const loader = ({ request }: Route.LoaderArgs) => listLoader(request, "project");

export default function Index({ loaderData }: Route.ComponentProps) {
  return <ListPage data={loaderData} />;
}

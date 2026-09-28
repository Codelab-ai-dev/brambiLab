import type { Route } from "./+types/articles";
import { listLoader } from "~/site/list.server";
import { ListPage } from "~/site/ListPage";
import { forwardHeaders, seoMeta } from "~/site/seo";

export const headers = forwardHeaders;

export function meta({ loaderData }: Route.MetaArgs) {
  return seoMeta(loaderData?.seo);
}

export const loader = ({ request }: Route.LoaderArgs) => listLoader(request, "article");

export default function Index({ loaderData }: Route.ComponentProps) {
  return <ListPage data={loaderData} />;
}

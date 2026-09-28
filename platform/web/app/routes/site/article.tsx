import type { Route } from "./+types/article";
import { detailLoader } from "~/site/detail.server";
import { DetailPage } from "~/site/DetailPage";
import { forwardHeaders, seoMeta } from "~/site/seo";

export const headers = forwardHeaders;

export function meta({ loaderData }: Route.MetaArgs) {
  return seoMeta(loaderData?.seo);
}

export const loader = ({ request, params }: Route.LoaderArgs) => detailLoader(request, `articles/${params.slug}`, [params.slug], "articles");

export default function Detail({ loaderData }: Route.ComponentProps) {
  return <DetailPage data={loaderData} />;
}

import { data } from "react-router";
import type { Route } from "./+types/sitemap-page";
import { contentUrls, staticUrls, urlset, xmlHeaders } from "~/site/sitemap.server";

export async function loader({ params }: Route.LoaderArgs) {
  const m = /^([1-9][0-9]{0,3})\.xml$/.exec(params.page ?? "");
  if (!m) throw data(null, { status: 404 });
  const page = Number(m[1]);
  const { urls } = await contentUrls(page);
  if (urls.length === 0 && page > 1) throw data(null, { status: 404 });
  return new Response(urlset(page === 1 ? [...staticUrls(), ...urls] : urls), { headers: xmlHeaders });
}

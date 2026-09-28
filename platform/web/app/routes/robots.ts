import { absolute } from "~/site/seo.server";

// Robots guidance only: /admin is protected by the API, not by this file.
export function loader() {
  const body = `User-agent: *\nDisallow: /admin\nDisallow: /api/\n\nSitemap: ${absolute("/sitemap.xml")}\n`;
  return new Response(body, { headers: { "Content-Type": "text/plain; charset=utf-8", "Cache-Control": "no-store" } });
}

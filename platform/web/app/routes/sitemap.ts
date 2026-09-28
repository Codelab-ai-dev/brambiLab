import { API_PAGE, contentUrls, sitemapIndex, staticUrls, urlset, xmlHeaders } from "~/site/sitemap.server";

// One urlset while everything fits in one API page; otherwise an index of /sitemaps/N.xml.
export async function loader() {
  const first = await contentUrls(1);
  if (first.total <= API_PAGE) return new Response(urlset([...staticUrls(), ...first.urls]), { headers: xmlHeaders });
  return new Response(sitemapIndex(Math.ceil(first.total / API_PAGE)), { headers: xmlHeaders });
}

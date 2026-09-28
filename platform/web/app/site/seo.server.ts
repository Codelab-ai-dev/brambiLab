// Canonical origin for absolute URLs (canonical, hreflang, Open Graph, sitemap). It comes from the
// validated PUBLIC_ORIGIN, never from the request Host or the container's internal address.

let cached: string | undefined;

export function parseOrigin(raw: string | undefined, production: boolean): string {
  if (!raw) {
    if (production) throw new Error("PUBLIC_ORIGIN is required in production");
    return "http://localhost:8000";
  }
  let url: URL;
  try {
    url = new URL(raw);
  } catch {
    throw new Error("PUBLIC_ORIGIN must be an absolute URL");
  }
  const local = url.hostname === "localhost" || url.hostname === "127.0.0.1";
  if (url.protocol !== "https:" && !(url.protocol === "http:" && local)) throw new Error("PUBLIC_ORIGIN must use https (http only for localhost)");
  if (url.username || url.password || (url.pathname !== "/" && url.pathname !== "") || url.search || url.hash) {
    throw new Error("PUBLIC_ORIGIN must be a bare origin such as https://example.com");
  }
  return url.origin;
}

export function publicOrigin(): string {
  cached ??= parseOrigin(process.env.PUBLIC_ORIGIN, process.env.NODE_ENV === "production");
  return cached;
}

export const absolute = (path: string) => publicOrigin() + path;

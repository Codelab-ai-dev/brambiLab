// Server-only reads of the public API. Public pages never forward cookies: the owner sees exactly
// what an anonymous visitor sees (web-v1.md §9.1). Redirects are not followed, so an alias becomes
// a site 301 instead of silently rendering under the old URL.
import { data } from "react-router";
import { internalApiUrl } from "~/lib/api.server";

const noStore = { "Cache-Control": "no-store" };

export type PublicResult<T> = { moved: false; data: T } | { moved: true; location: string | null };

async function request(path: string): Promise<Response> {
  try {
    return await fetch(new URL(path, internalApiUrl), {
      headers: { Accept: "application/json" },
      redirect: "manual",
      signal: AbortSignal.timeout(8000),
    });
  } catch {
    // The API is down or slow: a 503 error page, never "no content" nor a false 404.
    throw data({ reason: "unavailable" }, { status: 503, headers: noStore });
  }
}

function fail(response: Response, path: string): never {
  if (response.status === 404) throw data(null, { status: 404, headers: noStore });
  if (response.status === 422) throw data(null, { status: 400, headers: noStore });
  console.error(`public API ${path} answered ${response.status}`);
  throw data({ reason: "unavailable" }, { status: 503, headers: noStore });
}

export async function publicGet<T>(path: string): Promise<T> {
  const response = await request(path);
  if (!response.ok) fail(response, path);
  return (await response.json()) as T;
}

/** Like publicGet, but an API alias (301) is returned for the caller to translate. */
export async function publicRead<T>(path: string): Promise<PublicResult<T>> {
  const response = await request(path);
  if (response.status === 301 || response.status === 308) return { moved: true, location: response.headers.get("Location") };
  if (!response.ok) fail(response, path);
  return { moved: false, data: (await response.json()) as T };
}

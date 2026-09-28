// Server-side reads of private admin data. Every loader calls Go itself (forwarding only the
// session cookie), so no child route relies on its parent loader for protection (#12 §5).

import { data, redirect } from "react-router";
import { internalApiUrl, sessionCookieHeader } from "./api.server";

export async function adminGet<T>(request: Request, path: string): Promise<T> {
  const cookie = sessionCookieHeader(request.headers.get("Cookie"));
  const url = new URL(request.url);
  const login = `/admin/login?return_to=${encodeURIComponent(url.pathname + url.search)}`;
  if (!cookie) throw redirect(login);

  const response = await fetch(new URL(path, internalApiUrl), {
    headers: { Cookie: cookie, Accept: "application/json" },
    redirect: "error",
    signal: AbortSignal.timeout(10000),
  });
  if (response.status === 401) throw redirect(login);
  // A private 404 reveals nothing: the error page never echoes titles or text.
  if (response.status === 404) throw data(null, { status: 404 });
  if (!response.ok) throw new Error(`GET ${path} returned ${response.status}`);
  return (await response.json()) as T;
}
